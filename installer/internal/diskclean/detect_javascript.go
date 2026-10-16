package diskclean

import (
	"context"
	"path/filepath"
	"strings"
)

// The JavaScript ecosystem's four package managers, each with the store its
// own documentation names and the maintenance command it ships.
//
// The pattern is the same for all four, and it is the pattern every detector
// in this package follows: the tool is offered when a program is on PATH, when
// an environment variable names the store, or when the store directory exists
// -- and the reason says which of the three it was. When the program is
// installed its own command is the mechanism; when it is not, the store is
// removed directly, because a cache whose owner is gone is residue.

// detectNpm finds npm's content-addressable cache and the npx package cache.
//
// `npm config get cache` is the official way to ask where the cache is, but
// the answer is also what npm_config_cache and the documented default say; the
// variable and the default are read here so detection costs no command. The
// cache content lives in _cacache inside that directory, and _npx holds the
// packages npx fetched.
func detectNpm(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	cacheRoot := probe.Getenv("npm_config_cache")
	named := cacheRoot != ""
	if cacheRoot == "" {
		cacheRoot = filepath.Join(probe.HomeDir, ".npm")
	}
	paths := existingPaths(probe,
		filepath.Join(cacheRoot, "_cacache"),
		filepath.Join(cacheRoot, "_npx"),
	)
	program := firstProgram(probe, "npm")
	if len(paths) == 0 && program == "" {
		return nil
	}

	return []Item{{
		ID:        "npm-cache",
		Tool:      "npm",
		Title:     "The npm cache",
		Detail:    "the packages npm downloaded and unpacked, addressed by content. npm writes what it needs again; the entries are shared between projects, so removing them costs the next install a download, not the project's dependencies.",
		Category:  CategoryCache,
		Risk:      RiskSafe,
		Reason:    npmReason(named, cacheRoot, program, paths),
		Paths:     paths,
		Mechanism: toolMechanism(program, []string{"cache", "clean", "--force"}, paths),
	}}
}

// npmReason says which evidence found the cache.
func npmReason(named bool, cacheRoot, program string, paths []string) string {
	switch {
	case named && len(paths) > 0:
		return "$npm_config_cache names " + cacheRoot + ", and the cache is there"
	case named:
		return "$npm_config_cache names " + cacheRoot + ", so npm is configured to keep its cache there"
	case len(paths) > 0 && program != "":
		return "npm is on PATH and its cache directory exists"
	case len(paths) > 0:
		return "the npm cache directory " + cacheRoot + " exists, though npm is not on PATH"
	default:
		return "npm is on PATH, and its cache directory " + cacheRoot + " is empty or has not been created yet"
	}
}

// detectPnpm asks `pnpm store path`, which is the only reliable way to find a
// store whose layout changed between major versions, and falls back to the
// documented locations when the command cannot answer.
func detectPnpm(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "pnpm")
	var candidates []string
	fromTool := ""
	if program != "" {
		if output, err := runTool(ctx, probe, opts, program, "store", "path"); err == nil {
			if path := strings.TrimSpace(string(output)); path != "" {
				fromTool = path
				candidates = append(candidates, path)
			}
		}
	}
	candidates = append(candidates,
		filepath.Join(userDataDir(probe, opts.Facts, "pnpm"), "store"),
		filepath.Join(probe.HomeDir, ".pnpm-store"),
	)
	paths := existingPaths(probe, candidates...)
	if len(paths) == 0 && program == "" {
		return nil
	}

	reason := "the pnpm store directory exists"
	switch {
	case fromTool != "" && len(paths) > 0:
		reason = "pnpm store path named " + fromTool + ", and the store is there"
	case fromTool != "":
		reason = "pnpm store path named " + fromTool + ", so pnpm is configured to keep its store there"
	case program != "":
		reason = "pnpm is on PATH and its store directory exists"
	}

	return []Item{{
		ID:        "pnpm-store",
		Tool:      "pnpm",
		Title:     "The pnpm store",
		Detail:    "every package version pnpm has fetched, hardlinked into the projects that use it. `pnpm store prune` removes only the versions no project references; the download of a pruned package happens again on the next install.",
		Category:  CategoryCache,
		Risk:      RiskSafe,
		Reason:    reason,
		Paths:     paths,
		Mechanism: toolMechanism(program, []string{"store", "prune"}, paths),
	}}
}

// detectYarn finds Yarn's cache. Yarn Classic keeps it in the platform cache
// directory; Yarn Berry keeps its cache inside each project, which is a
// project-level store and reaches the scan through an added project directory.
func detectYarn(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "yarn")
	var cachePath string
	if opts.Facts.GOOS == "darwin" {
		cachePath = filepath.Join(probe.HomeDir, "Library", "Caches", "Yarn")
	} else {
		cachePath = userCacheDir(probe, opts.Facts, "yarn")
	}
	paths := existingPaths(probe, cachePath)
	if len(paths) == 0 && program == "" {
		return nil
	}

	reason := "the Yarn cache directory " + cachePath + " exists"
	switch {
	case len(paths) > 0 && program != "":
		reason = "yarn is on PATH and its cache directory " + cachePath + " exists"
	case len(paths) > 0:
		reason = "the Yarn cache directory " + cachePath + " exists, though yarn is not on PATH"
	default:
		reason = "yarn is on PATH, and its cache directory " + cachePath + " is empty or has not been created yet"
	}

	return []Item{{
		ID:        "yarn-cache",
		Tool:      "yarn",
		Title:     "The Yarn cache",
		Detail:    "the archives Yarn Classic downloaded. Yarn Berry projects keep their cache in the project's own .yarn directory, which is not this one.",
		Category:  CategoryCache,
		Risk:      RiskSafe,
		Reason:    reason,
		Paths:     paths,
		Mechanism: toolMechanism(program, []string{"cache", "clean"}, paths),
	}}
}

// detectBun finds Bun's install cache. Bun ships its own archive format, so
// only Bun should be trusted to remove it: `bun pm cache rm` is the documented
// command and is preferred wherever Bun is installed.
func detectBun(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "bun")
	cachePath := filepath.Join(probe.HomeDir, ".bun", "install", "cache")
	paths := existingPaths(probe, cachePath)
	if len(paths) == 0 && program == "" {
		return nil
	}

	reason := "the Bun install cache directory " + cachePath + " exists"
	switch {
	case len(paths) > 0 && program != "":
		reason = "bun is on PATH and its install cache directory " + cachePath + " exists"
	case len(paths) > 0:
		reason = "the Bun install cache directory " + cachePath + " exists, though bun is not on PATH"
	default:
		reason = "bun is on PATH, and its install cache directory " + cachePath + " is empty or has not been created yet"
	}

	return []Item{{
		ID:        "bun-cache",
		Tool:      "bun",
		Title:     "The Bun install cache",
		Detail:    "the packages Bun downloaded, in its own archive format. They are fetched again by the next install.",
		Category:  CategoryCache,
		Risk:      RiskSafe,
		Reason:    reason,
		Paths:     paths,
		Mechanism: toolMechanism(program, []string{"pm", "cache", "rm"}, paths),
	}}
}
