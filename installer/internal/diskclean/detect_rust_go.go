package diskclean

import (
	"context"
	"path/filepath"
	"strings"
)

// Rust and Go: the two toolchains whose stores have a documented location and,
// in Go's case, a documented command that reclaims them.

// detectCargo finds cargo's registry cache and the git checkouts it keeps.
//
// Neither `cargo` nor `rustup` has a command that prunes the registry, so the
// store is removed directly -- unless the widely used `cargo-cache` extension
// is installed, in which case its own command runs instead. That order is the
// rule of this package: the owner's command when there is one, a guarded
// removal when there is not.
func detectCargo(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	cargoHome := probe.Getenv("CARGO_HOME")
	named := cargoHome != ""
	if cargoHome == "" {
		cargoHome = filepath.Join(probe.HomeDir, ".cargo")
	}
	paths := existingPaths(probe,
		filepath.Join(cargoHome, "registry", "cache"),
		filepath.Join(cargoHome, "registry", "src"),
		filepath.Join(cargoHome, "git", "db"),
	)
	program := firstProgram(probe, "cargo")
	if len(paths) == 0 && program == "" {
		return nil
	}

	reason := "the cargo registry store exists"
	switch {
	case named && len(paths) > 0:
		reason = "$CARGO_HOME names " + cargoHome + ", and the registry store is there"
	case named:
		reason = "$CARGO_HOME names " + cargoHome + ", so cargo is configured to keep its store there"
	case len(paths) > 0 && program != "":
		reason = "cargo is on PATH and its registry store exists"
	case len(paths) > 0:
		reason = "the cargo registry store exists, though cargo is not on PATH"
	default:
		reason = "cargo is on PATH, and its registry store is empty or has not been created yet"
	}

	detail := "the crate archives cargo downloaded, the copies it unpacked from them, and the git " +
		"checkouts of git dependencies. cargo has no command of its own that prunes them; the next build " +
		"downloads what a project needs again. `cargo-cache` is not installed here."
	mechanism := Mechanism{Kind: MechanismRemove}
	if programOnPath(probe, "cargo-cache") && program != "" {
		detail = "the crate archives cargo downloaded, the copies it unpacked from them, and the git " +
			"checkouts of git dependencies. The installed `cargo-cache` extension's own command reclaims them."
		mechanism = Mechanism{Kind: MechanismTool, Program: program, Args: []string{"cache", "-a"}}
	}

	return []Item{{
		ID:        "cargo-registry",
		Tool:      "cargo",
		Title:     "The cargo registry store",
		Detail:    detail,
		Category:  CategoryDownloads,
		Risk:      RiskSafe,
		Reason:    reason,
		Paths:     paths,
		Mechanism: mechanism,
	}}
}

// detectGo asks the toolchain where its two caches are: `go env GOCACHE
// GOMODCACHE` is the documented interface, and it is what makes a host with
// GOPATH moved measure the directories Go really uses.
func detectGo(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "go")
	buildCache, moduleCache := "", ""
	if program != "" {
		if output, err := runTool(ctx, probe, opts, program, "env", "GOCACHE", "GOMODCACHE"); err == nil {
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			if len(lines) > 0 {
				buildCache = strings.TrimSpace(lines[0])
			}
			if len(lines) > 1 {
				moduleCache = strings.TrimSpace(lines[1])
			}
		}
	}

	var items []Item
	if buildCache != "" {
		if paths := existingPaths(probe, buildCache); len(paths) > 0 {
			items = append(items, Item{
				ID:        "go-build-cache",
				Tool:      "go",
				Title:     "The Go build cache",
				Detail:    "the compiled packages and test results Go keeps so a rebuild can reuse them. `go clean -cache` empties it, and the next build compiles again.",
				Category:  CategoryCache,
				Risk:      RiskSafe,
				Reason:    "go env GOCACHE named " + buildCache + ", and it exists",
				Paths:     paths,
				Mechanism: Mechanism{Kind: MechanismTool, Program: program, Args: []string{"clean", "-cache"}},
			})
		}
	}
	if moduleCache != "" {
		if paths := existingPaths(probe, moduleCache); len(paths) > 0 {
			items = append(items, Item{
				ID:        "go-module-cache",
				Tool:      "go",
				Title:     "The Go module cache",
				Detail:    "the source of every module version a project downloaded. `go clean -modcache` removes them, and the next build downloads and verifies them again; a project that vendors its dependencies still builds from the vendor directory.",
				Category:  CategoryDownloads,
				Risk:      RiskCareful,
				Reason:    "go env GOMODCACHE named " + moduleCache + ", and it exists",
				Paths:     paths,
				Mechanism: Mechanism{Kind: MechanismTool, Program: program, Args: []string{"clean", "-modcache"}},
			})
		}
	}
	return items
}
