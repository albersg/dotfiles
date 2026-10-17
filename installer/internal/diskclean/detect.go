package diskclean

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// This file is the detection half of the package: the registry of tools whose
// stores are looked for, and the helpers every detector shares.
//
// A detector answers one question -- is this tool's store on this machine --
// and it answers it with evidence: the program on PATH, the environment
// variable that named the path, or the directory that exists. A tool whose
// store cannot be established is left out rather than guessed at, because a
// row that names a path nobody looked at is worse than a missing row.

// Facts is what the scan knows about the platform before it looks at the
// machine: the operating system whose conventions decide where a cache lives,
// and the two hosting environments whose layout differs from a plain Linux
// install.
type Facts struct {
	GOOS string
	// WSL is a Linux host inside Windows. The screen names it; the store paths
	// are Linux paths either way.
	WSL bool
	// Termux is the Android terminal, where the system package manager's cache
	// lives under $PREFIX and needs no administrator rights.
	Termux bool
	// Prefix is Termux's $PREFIX, empty everywhere else.
	Prefix string
	// SystemRoot is the prefix the system package managers' caches hang from:
	// "/" on every real host, and moved only by a test, so a detection test
	// never reads or measures the machine it runs on.
	SystemRoot string
}

// FactsFor derives the scan's platform facts from a detected system, keeping
// the package free of a second platform detection.
func FactsFor(goos string, wsl, termux bool, prefix string) Facts {
	if goos == "" {
		goos = runtime.GOOS
	}
	return Facts{GOOS: goos, WSL: wsl, Termux: termux, Prefix: prefix}
}

// systemRoot is the root the system package managers' caches are resolved
// from. An unset root is the real filesystem root.
func systemRoot(facts Facts) string {
	if facts.SystemRoot == "" {
		return string(os.PathSeparator)
	}
	return facts.SystemRoot
}

// DetectOptions shape one detection pass.
type DetectOptions struct {
	Facts Facts
	// ProjectRoots are directories the user added to the scope. Their build
	// artifacts and dependency trees are findings of their own, and they are
	// the only place this package looks for project-level stores.
	ProjectRoots []string
	// ToolTimeout bounds every command a detector runs. A container engine
	// that is not answering must not hold the scan open.
	ToolTimeout time.Duration
}

// DefaultToolTimeout is the bound a detector's own command runs under.
const DefaultToolTimeout = 20 * time.Second

// Detect runs every detector and returns the findings that are present, in
// registry order. It never returns an error: a detector that cannot answer
// simply contributes nothing, and the reason the tool was not offered is that
// no evidence of it was found.
func Detect(ctx context.Context, probe Probe, opts DetectOptions) []Item {
	if opts.ToolTimeout <= 0 {
		opts.ToolTimeout = DefaultToolTimeout
	}

	var items []Item
	for _, detector := range detectors {
		if err := ctx.Err(); err != nil {
			break
		}
		items = append(items, detector(ctx, probe, opts)...)
	}
	if err := ctx.Err(); err != nil {
		return items
	}
	items = append(items, detectProjectArtifacts(probe, opts.ProjectRoots, opts.Facts)...)
	return dedupeByID(items)
}

// detector is one tool's rule for finding its own store.
type detector func(ctx context.Context, probe Probe, opts DetectOptions) []Item

// detectors is the registry, in the order the rows appear before they are
// sorted by size. The order groups an ecosystem's tools together so two rows
// about the same package manager never end up apart.
var detectors = []detector{
	detectNpm,
	detectPnpm,
	detectYarn,
	detectBun,
	detectPip,
	detectUV,
	detectPoetry,
	detectConda,
	detectCargo,
	detectGo,
	detectGradle,
	detectMaven,
	detectCoursier,
	detectApt,
	detectDnf,
	detectPacman,
	detectBrew,
	detectDocker,
	detectPodman,
}

// dedupeByID keeps the first item with each ID. Two detectors must not claim
// the same store; when they do, the first is the one kept and the second is
// dropped rather than shown twice.
func dedupeByID(items []Item) []Item {
	seen := make(map[string]bool, len(items))
	kept := items[:0]
	for _, item := range items {
		if item.ID == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		kept = append(kept, item)
	}
	return kept
}

// programOnPath reports whether a program is installed, without keeping the
// path: detection needs the fact, not the location.
func programOnPath(probe Probe, name string) bool {
	if probe.LookPath == nil {
		return false
	}
	_, err := probe.LookPath(name)
	return err == nil
}

// firstProgram returns the first of the names that is installed, so a tool
// that ships as python's pip3 is found as well as pip.
func firstProgram(probe Probe, names ...string) string {
	for _, name := range names {
		if programOnPath(probe, name) {
			return name
		}
	}
	return ""
}

// directoryExists reports whether a path is a directory, following no link:
// a symlink is not where the store is, and the walk refuses it later anyway.
func directoryExists(probe Probe, path string) bool {
	info, err := probe.Lstat(path)
	return err == nil && info != nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

// pathExists reports whether anything exists at a path.
func pathExists(probe Probe, path string) bool {
	_, err := probe.Lstat(path)
	return err == nil
}

// existingPaths keeps the candidates that exist, in the order they were given,
// so an item's paths are the ones that really hold something.
func existingPaths(probe Probe, candidates ...string) []string {
	var found []string
	for _, candidate := range candidates {
		if candidate != "" && pathExists(probe, candidate) {
			found = append(found, candidate)
		}
	}
	return found
}

// userCacheDir is the platform convention for a tool's cache directory:
// $XDG_CACHE_HOME when it is set, ~/Library/Caches on macOS, ~/.cache
// everywhere else. The tool's own subdirectory is appended by the caller.
func userCacheDir(probe Probe, facts Facts, tool string) string {
	if value := probe.Getenv("XDG_CACHE_HOME"); value != "" {
		return filepath.Join(value, tool)
	}
	if facts.GOOS == "darwin" {
		return filepath.Join(probe.HomeDir, "Library", "Caches", tool)
	}
	return filepath.Join(probe.HomeDir, ".cache", tool)
}

// userDataDir is the platform convention for a tool's data directory, where
// package stores that are not caches live: $XDG_DATA_HOME when it is set,
// ~/Library/Application Support on macOS, ~/.local/share everywhere else.
func userDataDir(probe Probe, facts Facts, tool string) string {
	if value := probe.Getenv("XDG_DATA_HOME"); value != "" {
		return filepath.Join(value, tool)
	}
	if facts.GOOS == "darwin" {
		if facts.WSL || facts.Termux {
			return filepath.Join(probe.HomeDir, ".local", "share", tool)
		}
		return filepath.Join(probe.HomeDir, "Library", "Application Support", tool)
	}
	return filepath.Join(probe.HomeDir, ".local", "share", tool)
}

// toolMechanism picks how a store whose owner has no maintenance command is
// cleaned once the owner is installed. Where the owner exists, its documented
// command runs; where it does not, the store is removed directly, because a
// cache left behind by an uninstalled tool is exactly the residue the utility
// is for.
func toolMechanism(program string, args []string, paths []string) Mechanism {
	if program != "" {
		return Mechanism{Kind: MechanismTool, Program: program, Args: args}
	}
	return Mechanism{Kind: MechanismRemove}
}

// removeMechanism is the guarded removal, used where the owner offers no
// command that reclaims its store.
func removeMechanism() Mechanism { return Mechanism{Kind: MechanismRemove} }

// runTool runs a detector's own command under the scan's timeout, returning
// the output or the reason it could not be read.
func runTool(ctx context.Context, probe Probe, opts DetectOptions, name string, args ...string) ([]byte, error) {
	if probe.Run == nil {
		return nil, errors.New("no runner")
	}
	ctx, cancel := context.WithTimeout(ctx, opts.ToolTimeout)
	defer cancel()
	return probe.Run(ctx, name, args...)
}

// detectProjectArtifacts finds the build artifacts and dependency trees inside
// the directories the user added on purpose. Nothing else on the machine is
// searched for project-level stores: the utility does not scan disks looking
// for the user's repositories.
func detectProjectArtifacts(probe Probe, roots []string, facts Facts) []Item {
	var items []Item
	for _, root := range roots {
		if root == "" || !directoryExists(probe, root) {
			continue
		}
		items = append(items, projectArtifactsIn(probe, root)...)
	}
	return items
}

// projectArtifactKinds is the fixed list of directory names a project may hold
// that this utility knows how to explain. A name that is not here is never
// offered, so a directory this utility does not understand cannot be removed
// by a guess.
//
// Each kind names the marker file that has to sit beside it for the directory
// to be believed. The marker is what separates a Rust target from a directory
// that merely happens to be called target.
type projectArtifactKind struct {
	name     string
	markers  []string
	category Category
	risk     Risk
	detail   string
}

var projectArtifactKinds = []projectArtifactKind{
	{
		name: "node_modules", markers: []string{"package.json"},
		category: CategoryDependencies, risk: RiskCareful,
		detail: "the JavaScript packages the project installed. They are not the project's own code: " +
			"the package manager writes them again from the lockfile, which is the same download it made the first time.",
	},
	{
		name: ".yarn", markers: []string{"package.json", ".yarnrc.yml"},
		category: CategoryDependencies, risk: RiskCareful,
		detail: "the packages a Yarn Berry project checks in or caches. The project re-fetches them from its lockfile.",
	},
	{
		name: "bower_components", markers: []string{"bower.json"},
		category: CategoryDependencies, risk: RiskCareful,
		detail: "the packages the project fetched with bower, which are re-fetched by the next install.",
	},
	{
		name: "vendor", markers: []string{"go.mod", "composer.json"},
		category: CategoryDependencies, risk: RiskCareful,
		detail: "the dependency copies the project keeps in its own tree, so the module cache or the package " +
			"manager is asked for them again on the next build or install.",
	},
	{
		name: "target", markers: []string{"Cargo.toml"},
		category: CategoryBuild, risk: RiskCareful,
		detail: "the compiled Rust output. It is rebuilt from the sources by cargo, not downloaded, but the " +
			"rebuild takes as long as the code takes.",
	},
	{
		name: "build", markers: []string{"build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "package.json", "CMakeLists.txt", "pom.xml"},
		category: CategoryBuild, risk: RiskCareful,
		detail: "the project's build output. It is produced again by the build command, from the sources and " +
			"the dependencies.",
	},
	{
		name: "dist", markers: []string{"package.json", "pyproject.toml"},
		category: CategoryBuild, risk: RiskCareful,
		detail: "the packaged output of the project's build, produced again by the build command.",
	},
	{
		name: "out", markers: []string{"package.json", "build.gradle", "build.gradle.kts"},
		category: CategoryBuild, risk: RiskCareful,
		detail: "the project's build output directory, produced again by the build command.",
	},
	{
		name: ".next", markers: []string{"package.json", "next.config.js", "next.config.mjs"},
		category: CategoryBuild, risk: RiskCareful,
		detail: "Next.js's build output and route cache, rebuilt by the next build.",
	},
	{
		name: ".nuxt", markers: []string{"package.json", "nuxt.config.js", "nuxt.config.ts"},
		category: CategoryBuild, risk: RiskCareful,
		detail: "Nuxt's build output, rebuilt by the next build.",
	},
	{
		name: ".gradle", markers: []string{"build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts"},
		category: CategoryBuild, risk: RiskSafe,
		detail: "this project's Gradle metadata and caches, rebuilt by the next Gradle invocation.",
	},
	{
		name: "coverage", markers: []string{"package.json", "pyproject.toml", "go.mod", "Gemfile"},
		category: CategoryBuild, risk: RiskSafe,
		detail: "the coverage reports a test run wrote. Running the tests again writes them again.",
	},
	{
		name: "__pycache__", markers: nil,
		category: CategoryBuild, risk: RiskSafe,
		detail: "Python's compiled bytecode, written again the next time the module is imported.",
	},
	{
		name: ".venv", markers: []string{"pyproject.toml", "requirements.txt", "setup.py", "Pipfile", "poetry.lock", "uv.lock"},
		category: CategoryDependencies, risk: RiskCareful,
		detail: "the project's own Python environment. Its packages are installed again from the lockfile or " +
			"the requirements file.",
	},
	{
		name: "venv", markers: []string{"pyproject.toml", "requirements.txt", "setup.py", "Pipfile"},
		category: CategoryDependencies, risk: RiskCareful,
		detail: "the project's own Python environment, installed again from the requirements or the lockfile.",
	},
	{
		name: ".terraform", markers: []string{"*.tf"},
		category: CategoryDependencies, risk: RiskCareful,
		detail: "the provider plugins Terraform downloaded for this project; terraform init fetches them again.",
	},
}

// artifactMarkerMatches reports whether a marker entry names a file that is
// present. A marker with a '*' is a prefix rule, which is how a directory that
// holds several files of one kind -- Terraform's *.tf -- is recognized.
func artifactMarkerMatches(probe Probe, dir, marker string) bool {
	if marker == "" {
		return false
	}
	if marker == "*" {
		return false
	}
	if marker[0] == '*' {
		suffix := marker[1:]
		entries, err := probe.ReadDir(dir)
		if err != nil {
			return false
		}
		for _, entry := range entries {
			name := entry.Name()
			if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
				return true
			}
		}
		return false
	}
	return pathExists(probe, filepath.Join(dir, marker))
}

// projectArtifactsIn walks one project root, bounded in depth and in count,
// and returns one item per artifact directory found. It never descends into an
// artifact directory, never follows a link and never enters .git.
func projectArtifactsIn(probe Probe, root string) []Item {
	const (
		maxDepth     = 4
		maxArtifacts = 200
	)
	type pending struct {
		path  string
		depth int
	}

	var items []Item
	queue := []pending{{path: root, depth: 0}}
	for len(queue) > 0 && len(items) < maxArtifacts {
		current := queue[0]
		queue = queue[1:]
		if current.depth >= maxDepth {
			continue
		}
		entries, err := probe.ReadDir(current.path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if len(items) >= maxArtifacts {
				break
			}
			name := entry.Name()
			if name == ".git" {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				continue
			}
			child := filepath.Join(current.path, name)
			if kind, ok := artifactKindFor(probe, current.path, name); ok {
				items = append(items, projectArtifactItem(root, child, name, kind))
				continue
			}
			if name[0] == '.' && name != "." {
				// A hidden directory that is not a known artifact is not
				// searched further: the user's dotfiles are not build output.
				continue
			}
			queue = append(queue, pending{path: child, depth: current.depth + 1})
		}
	}
	return items
}

// artifactKindFor matches a directory name against the fixed list, requiring
// the marker that makes the name believable.
func artifactKindFor(probe Probe, parent, name string) (projectArtifactKind, bool) {
	for _, kind := range projectArtifactKinds {
		if kind.name != name {
			continue
		}
		if len(kind.markers) == 0 {
			return kind, true
		}
		for _, marker := range kind.markers {
			if artifactMarkerMatches(probe, parent, marker) {
				return kind, true
			}
		}
		return kind, false
	}
	return projectArtifactKind{}, false
}

// projectArtifactItem builds the finding for one project artifact directory.
func projectArtifactItem(root, path, name string, kind projectArtifactKind) Item {
	return Item{
		ID:       "project:" + path,
		Tool:     "project",
		Title:    name + " in " + shortPath(root),
		Detail:   kind.detail,
		Category: kind.category,
		Risk:     kind.risk,
		Reason:   fmt.Sprintf("%s is inside the project directory %s, which was added to the scope on purpose", name, root),
		Paths:    []string{path},
		// The project directory is the approved scope, not the artifact
		// itself: a removal that only approves the artifact would accept a
		// path the user never saw, while the project root is what was added.
		Roots:     []string{root},
		Mechanism: removeMechanism(),
	}
}

// shortPath shortens a long path for a row title, keeping its tail -- which is
// the part that names the project.
func shortPath(path string) string {
	const keep = 48
	if len(path) <= keep {
		return path
	}
	return "…" + path[len(path)-keep:]
}
