package diskclean

import (
	"context"
	"encoding/json"
	"path/filepath"
)

// The Python ecosystem: pip's wheel and HTTP caches, uv's cache, poetry's
// artifact cache and environments, and conda's package directories.
//
// Two of these are worth a note. uv's cache is asked for rather than assumed,
// because UV_CACHE_DIR moves it. Poetry has no command that clears every cache
// it keeps -- `poetry cache clear` names one cache at a time -- so its
// artifacts are removed directly, with the reason on the row.

// detectPip finds pip's cache. PIP_CACHE_DIR names it when the user moved it;
// otherwise it is the platform's cache directory for pip.
func detectPip(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "pip3", "pip")
	named := probe.Getenv("PIP_CACHE_DIR")
	cachePath := named
	if cachePath == "" {
		cachePath = userCacheDir(probe, opts.Facts, "pip")
	}
	paths := existingPaths(probe, cachePath)
	if len(paths) == 0 && program == "" {
		return nil
	}

	reason := "the pip cache directory exists"
	switch {
	case named != "" && len(paths) > 0:
		reason = "$PIP_CACHE_DIR names " + cachePath + ", and the cache is there"
	case named != "":
		reason = "$PIP_CACHE_DIR names " + cachePath + ", so pip is configured to keep its cache there"
	case len(paths) > 0 && program != "":
		reason = program + " is on PATH and the pip cache directory exists"
	case len(paths) > 0:
		reason = "the pip cache directory exists, though pip is not on PATH"
	default:
		reason = program + " is on PATH, and its cache directory is empty or has not been created yet"
	}

	return []Item{{
		ID:        "pip-cache",
		Tool:      "pip",
		Title:     "The pip cache",
		Detail:    "the wheels pip built and the responses it downloaded. `pip cache purge` empties it, and the next install downloads what it needs again.",
		Category:  CategoryCache,
		Risk:      RiskSafe,
		Reason:    reason,
		Paths:     paths,
		Mechanism: toolMechanism(program, []string{"cache", "purge"}, paths),
	}}
}

// detectUV finds uv's cache, which is both a download cache and a build cache.
func detectUV(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "uv")
	named := probe.Getenv("UV_CACHE_DIR")
	cachePath := named
	if cachePath == "" {
		cachePath = userCacheDir(probe, opts.Facts, "uv")
	}
	paths := existingPaths(probe, cachePath)
	if len(paths) == 0 && program == "" {
		return nil
	}

	reason := "the uv cache directory exists"
	switch {
	case named != "" && len(paths) > 0:
		reason = "$UV_CACHE_DIR names " + cachePath + ", and the cache is there"
	case named != "":
		reason = "$UV_CACHE_DIR names " + cachePath + ", so uv is configured to keep its cache there"
	case len(paths) > 0 && program != "":
		reason = "uv is on PATH and its cache directory exists"
	case len(paths) > 0:
		reason = "the uv cache directory exists, though uv is not on PATH"
	default:
		reason = "uv is on PATH, and its cache directory is empty or has not been created yet"
	}

	return []Item{{
		ID:        "uv-cache",
		Tool:      "uv",
		Title:     "The uv cache",
		Detail:    "the wheels and source distributions uv downloaded and built, shared between projects. `uv cache prune` removes what no project references; `uv cache clean` empties the whole cache, which is what this row promises, and the next sync fetches again.",
		Category:  CategoryCache,
		Risk:      RiskSafe,
		Reason:    reason,
		Paths:     paths,
		Mechanism: toolMechanism(program, []string{"cache", "clean"}, paths),
	}}
}

// detectPoetry finds poetry's artifact cache and the virtual environments it
// keeps for projects. The two are separate rows on purpose: one is a cache,
// the other is a project's installed dependencies and costs an install to
// rebuild.
func detectPoetry(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	cacheRoot := probe.Getenv("XDG_CACHE_HOME")
	if cacheRoot != "" {
		cacheRoot = filepath.Join(cacheRoot, "pypoetry")
	} else if opts.Facts.GOOS == "darwin" {
		cacheRoot = filepath.Join(probe.HomeDir, "Library", "Caches", "pypoetry")
	} else {
		cacheRoot = filepath.Join(probe.HomeDir, ".cache", "pypoetry")
	}
	program := firstProgram(probe, "poetry")

	var items []Item
	artifactPaths := existingPaths(probe,
		filepath.Join(cacheRoot, "artifacts"),
		filepath.Join(cacheRoot, "cache"),
	)
	if len(artifactPaths) > 0 || program != "" {
		reason := "the poetry artifact cache exists"
		if len(artifactPaths) == 0 {
			reason = "poetry is on PATH, and its artifact cache is empty or has not been created yet"
		} else if program == "" {
			reason = "the poetry cache directory exists, though poetry is not on PATH"
		}
		items = append(items, Item{
			ID:        "poetry-artifacts",
			Tool:      "poetry",
			Title:     "The poetry artifact cache",
			Detail:    "the packages poetry downloaded and the wheels it built. Poetry has no command that clears every cache it keeps -- `poetry cache clear` names one at a time -- so this directory is what is removed; the next install downloads again.",
			Category:  CategoryCache,
			Risk:      RiskSafe,
			Reason:    reason,
			Paths:     artifactPaths,
			Mechanism: removeMechanism(),
		})
	}

	envPaths := existingPaths(probe, filepath.Join(cacheRoot, "virtualenvs"))
	if len(envPaths) > 0 {
		items = append(items, Item{
			ID:        "poetry-virtualenvs",
			Tool:      "poetry",
			Title:     "The poetry environments",
			Detail:    "the virtual environments poetry made for individual projects. They are not caches: each one holds the packages a project installed, and removing them means that project needs `poetry install` again before it runs. Nothing in them is source code.",
			Category:  CategoryDependencies,
			Risk:      RiskCareful,
			Reason:    "poetry keeps a virtual environment per project under " + cacheRoot,
			Paths:     envPaths,
			Mechanism: removeMechanism(),
		})
	}
	return items
}

// condaInfo is the part of `conda info --json` this package reads: the
// directories conda keeps its downloaded packages in.
type condaInfo struct {
	PackageDirs []string `json:"pkgs_dirs"`
}

// detectConda asks conda where its package directories are, because a conda
// installation lives wherever the user made it and no default path can be
// assumed. A conda whose answer cannot be read is left out rather than guessed
// at.
func detectConda(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	program := firstProgram(probe, "conda", "mamba")
	if program == "" {
		return nil
	}
	output, err := runTool(ctx, probe, opts, program, "info", "--json")
	if err != nil {
		return nil
	}
	var info condaInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil
	}
	paths := existingPaths(probe, info.PackageDirs...)
	if len(paths) == 0 {
		return nil
	}

	return []Item{{
		ID:        "conda-packages",
		Tool:      "conda",
		Title:     "The conda package cache",
		Detail:    "the package archives conda downloaded and the extracted copies it links environments from. `conda clean --all` clears the archives, the index cache and the packages no environment is using; a package an environment still needs is not touched.",
		Category:  CategoryDownloads,
		Risk:      RiskSafe,
		Reason:    program + " info --json named " + paths[0] + ", and it exists",
		Paths:     paths,
		Mechanism: Mechanism{Kind: MechanismTool, Program: program, Args: []string{"clean", "--all", "--yes"}},
	}}
}
