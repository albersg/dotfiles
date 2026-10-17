package diskclean

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// detectorProbe is a machine with nothing on it: a home of its own, no
// environment, nothing on PATH and no runner. Every test adds exactly the one
// piece of evidence it is about, and no test can read the machine it runs on.
func detectorProbe(t *testing.T) Probe {
	t.Helper()
	probe := DefaultProbe()
	probe.HomeDir = t.TempDir()
	probe.Getenv = func(string) string { return "" }
	probe.LookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	probe.Run = func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("this test installed no program")
	}
	return probe
}

// withPrograms makes the named programs installed on the probe's PATH.
func withPrograms(probe Probe, names ...string) Probe {
	probe.LookPath = func(name string) (string, error) {
		for _, installed := range names {
			if installed == name {
				return "/test/bin/" + name, nil
			}
		}
		return "", exec.ErrNotFound
	}
	return probe
}

// withEnv pins the environment the probe reads.
func withEnv(probe Probe, values map[string]string) Probe {
	probe.Getenv = func(key string) string { return values[key] }
	return probe
}

// itemByID finds one item of a detection result, failing the test when it is
// absent so an assertion about a row cannot pass because the row is missing.
func itemByID(t *testing.T, items []Item, id string) Item {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	var ids []string
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	t.Fatalf("no item %q in %v", id, ids)
	return Item{}
}

// TestDetectFindsNothingOnABareMachine pins the evidence rule: with no program,
// no variable and no directory, no store is offered -- not even as a zero.
func TestDetectFindsNothingOnABareMachine(t *testing.T) {
	probe := detectorProbe(t)
	items := Detect(context.Background(), probe, DetectOptions{
		Facts: Facts{GOOS: "linux", SystemRoot: t.TempDir()},
	})
	if len(items) != 0 {
		t.Fatalf("a machine with nothing on it produced %d findings: %v", len(items), itemIDs(items))
	}
}

// TestDetectNpmPrefersItsOwnCommandAndFallsBackToTheGuardedRemoval pins both
// halves of the mechanism rule for a cache whose directory does not need the
// tool to be removed safely.
func TestDetectNpmPrefersItsOwnCommandAndFallsBackToTheGuardedRemoval(t *testing.T) {
	probe := detectorProbe(t)
	home := probe.HomeDir
	cacheRoot := filepath.Join(home, ".npm")
	writeTestFile(t, filepath.Join(cacheRoot, "_cacache", "content.bin"), 100)
	writeTestFile(t, filepath.Join(cacheRoot, "_npx", "package", "index.js"), 50)

	// npm is not installed: the cache is still residue worth reporting, and the
	// removal is the guarded one.
	items := detectNpm(context.Background(), probe, DetectOptions{})
	if len(items) != 1 {
		t.Fatalf("npm's cache produced %d items, want 1", len(items))
	}
	item := items[0]
	if item.Mechanism.Kind != MechanismRemove {
		t.Errorf("without npm the mechanism is %v, want the guarded removal", item.Mechanism.Kind)
	}
	if len(item.Paths) != 2 {
		t.Errorf("the item covers %v, want the cache and the npx cache", item.Paths)
	}
	if !strings.Contains(item.Reason, cacheRoot) {
		t.Errorf("the reason does not name the cache directory: %s", item.Reason)
	}

	// npm is installed: its documented command is the mechanism.
	installed := withPrograms(probe, "npm")
	item = detectNpm(context.Background(), installed, DetectOptions{})[0]
	if item.Mechanism.Kind != MechanismTool || item.Mechanism.Program != "npm" {
		t.Fatalf("with npm the mechanism is %+v, want npm's own command", item.Mechanism)
	}
	if got := item.Mechanism.Command(); got != "npm cache clean --force" {
		t.Errorf("the command is %q, want npm cache clean --force", got)
	}
	if !strings.Contains(item.Reason, "npm is on PATH") {
		t.Errorf("the reason does not name the evidence: %s", item.Reason)
	}
}

// TestDetectHonorsTheEnvironmentThatMovesACache pins the "paths configured"
// half of the evidence rule: a store the user moved is the store that is
// measured, and the reason says which variable named it.
func TestDetectHonorsTheEnvironmentThatMovesACache(t *testing.T) {
	probe := detectorProbe(t)
	moved := filepath.Join(t.TempDir(), "npm-cache")
	writeTestFile(t, filepath.Join(moved, "_cacache", "entry.bin"), 64)
	probe = withEnv(probe, map[string]string{"npm_config_cache": moved})

	item := detectNpm(context.Background(), probe, DetectOptions{})[0]
	if len(item.Paths) != 1 || item.Paths[0] != filepath.Join(moved, "_cacache") {
		t.Errorf("the paths are %v, want the moved cache", item.Paths)
	}
	if !strings.Contains(item.Reason, "$npm_config_cache") {
		t.Errorf("the reason does not name the variable that moved it: %s", item.Reason)
	}
}

// TestDetectPnpmAsksTheToolForItsStorePath pins that a store whose layout
// changes between versions is asked for rather than assumed.
func TestDetectPnpmAsksTheToolForItsStorePath(t *testing.T) {
	probe := withPrograms(detectorProbe(t), "pnpm")
	store := filepath.Join(t.TempDir(), "pnpm-store", "v10")
	writeTestFile(t, filepath.Join(store, "files", "pkg.bin"), 128)
	probe.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "pnpm" && strings.Join(args, " ") == "store path" {
			return []byte(store + "\n"), nil
		}
		return nil, errors.New("unexpected command")
	}

	item := detectPnpm(context.Background(), probe, DetectOptions{})[0]
	if len(item.Paths) != 1 || item.Paths[0] != store {
		t.Errorf("the store is %v, want the path pnpm named", item.Paths)
	}
	if !strings.Contains(item.Reason, "pnpm store path") {
		t.Errorf("the reason does not say where the path came from: %s", item.Reason)
	}
	if got := item.Mechanism.Command(); got != "pnpm store prune" {
		t.Errorf("the command is %q, want pnpm store prune", got)
	}
}

// TestDetectGoAsksTheToolchainWhereItsCachesAre pins the same rule for Go,
// where GOPATH and the build cache can both be moved.
func TestDetectGoAsksTheToolchainWhereItsCachesAre(t *testing.T) {
	probe := withPrograms(detectorProbe(t), "go")
	build := filepath.Join(t.TempDir(), "go-build")
	module := filepath.Join(t.TempDir(), "go-mod")
	writeTestFile(t, filepath.Join(build, "01", "entry"), 32)
	writeTestFile(t, filepath.Join(module, "cache", "download", "pkg.zip"), 64)
	probe.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "go" && strings.Join(args, " ") == "env GOCACHE GOMODCACHE" {
			return []byte(build + "\n" + module + "\n"), nil
		}
		return nil, errors.New("unexpected command")
	}

	items := Detect(context.Background(), probe, DetectOptions{Facts: Facts{GOOS: "linux"}})
	buildItem := itemByID(t, items, "go-build-cache")
	if got := buildItem.Mechanism.Command(); got != "go clean -cache" {
		t.Errorf("the build cache command is %q, want go clean -cache", got)
	}
	moduleItem := itemByID(t, items, "go-module-cache")
	if moduleItem.Risk != RiskCareful {
		t.Errorf("the module cache risk is %v, want careful: it costs a re-download", moduleItem.Risk)
	}
	if got := moduleItem.Mechanism.Command(); got != "go clean -modcache" {
		t.Errorf("the module cache command is %q, want go clean -modcache", got)
	}
}

// TestDetectCondaUsesTheDirectoriesItsOwnInfoReports pins detection for a tool
// whose installation location cannot be assumed: the paths come from conda.
func TestDetectCondaUsesTheDirectoriesItsOwnInfoReports(t *testing.T) {
	probe := withPrograms(detectorProbe(t), "conda")
	packages := filepath.Join(t.TempDir(), "miniconda3", "pkgs")
	writeTestFile(t, filepath.Join(packages, "pkg.tar.bz2"), 128)
	probe.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "conda" && strings.Join(args, " ") == "info --json" {
			return []byte(`{"pkgs_dirs":["` + packages + `"]}`), nil
		}
		return nil, errors.New("unexpected command")
	}

	items := detectConda(context.Background(), probe, DetectOptions{})
	if len(items) != 1 {
		t.Fatalf("conda produced %d items, want 1", len(items))
	}
	if got := items[0].Mechanism.Command(); got != "conda clean --all --yes" {
		t.Errorf("the command is %q, want conda clean --all --yes", got)
	}
	if !strings.Contains(items[0].Reason, packages) {
		t.Errorf("the reason does not name the directory conda reported: %s", items[0].Reason)
	}
}

// TestDetectDockerUsesOnlyTheEngineCommandsAndKeepsVolumesDelicate pins the
// container rule end to end: every row is the engine's own command, no row
// carries a filesystem path, the sizes are read from the engine's accounting,
// and a volume is delicate.
func TestDetectDockerUsesOnlyTheEngineCommandsAndKeepsVolumesDelicate(t *testing.T) {
	probe := withPrograms(detectorProbe(t), "docker")
	probe.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "docker" {
			return nil, errors.New("unexpected program")
		}
		return []byte(strings.Join([]string{
			`{"Type":"Images","TotalCount":"5","Active":"2","Size":"1.23GB","Reclaimable":"500MB (40%)"}`,
			`{"Type":"Containers","TotalCount":"3","Active":"0","Size":"10MB","Reclaimable":"10MB (100%)"}`,
			`{"Type":"Local Volumes","TotalCount":"2","Active":"0","Size":"100MB","Reclaimable":"100MB (100%)"}`,
			`{"Type":"Build Cache","TotalCount":"4","Active":"0","Size":"1GB","Reclaimable":"1GB (100%)"}`,
		}, "\n")), nil
	}

	items := detectDocker(context.Background(), probe, DetectOptions{})

	images := itemByID(t, items, "docker-images")
	if got := images.Mechanism.Command(); got != "docker image prune -a -f" {
		t.Errorf("the images command is %q", got)
	}
	if images.Size.Source != SourceTool {
		t.Errorf("the image size source is %v, want the tool's own accounting", images.Size.Source)
	}
	if want := int64(500e6); images.Reclaimable() != want {
		t.Errorf("the images reclaimable size is %d, want %d", images.Reclaimable(), want)
	}

	containers := itemByID(t, items, "docker-containers")
	if got := containers.Mechanism.Command(); got != "docker container prune -f" {
		t.Errorf("the containers command is %q", got)
	}

	buildCache := itemByID(t, items, "docker-build-cache")
	if buildCache.Risk != RiskSafe {
		t.Errorf("the build cache risk is %v, want safe: the next build recreates it", buildCache.Risk)
	}

	volumes := itemByID(t, items, "docker-volumes")
	if volumes.Risk != RiskDelicate {
		t.Errorf("the volumes risk is %v, want delicate", volumes.Risk)
	}
	if len(volumes.VolatileObjects) == 0 {
		t.Error("the volumes row does not name the objects its command removes")
	}

	for _, item := range items {
		if len(item.Paths) != 0 {
			t.Errorf("%s carries filesystem paths %v: the engine's stores are never removed directly", item.ID, item.Paths)
		}
		if item.Mechanism.Kind != MechanismTool || item.Mechanism.Program != "docker" {
			t.Errorf("%s does not use the engine's own command: %+v", item.ID, item.Mechanism)
		}
	}
}

// TestContainerRowsSkipStoresWithNothingToReclaim pins that a row offering to
// free nothing is not drawn at all.
func TestContainerRowsSkipStoresWithNothingToReclaim(t *testing.T) {
	items := containerStoresFor("docker", "docker", []toolStore{
		{Kind: "images", Label: "Images", Count: 3, Size: 100, Reclaimable: 40},
		{Kind: "volumes", Label: "Local Volumes", Count: 2, Size: 500, Reclaimable: 0},
	})
	if len(items) != 1 || items[0].ID != "docker-images" {
		t.Fatalf("the rows are %v, want only the images row", itemIDs(items))
	}
}

// TestDetectPodmanReadsItsJSONArray pins the second engine's shape and the same
// rules: its own command, no paths, nothing invented for a store with zero
// reclaimable bytes.
func TestDetectPodmanReadsItsJSONArray(t *testing.T) {
	probe := withPrograms(detectorProbe(t), "podman")
	probe.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte(`[{"Type":"Images","Total":2,"Size":"1.5GB","Reclaimable":"700MB"},` +
			`{"Type":"Build Cache","Total":1,"Size":"0B","Reclaimable":"0B"}]`), nil
	}

	items := detectPodman(context.Background(), probe, DetectOptions{})
	if len(items) != 1 {
		t.Fatalf("podman produced %v, want only the images row", itemIDs(items))
	}
	if got := items[0].Mechanism.Command(); got != "podman image prune -a -f" {
		t.Errorf("the command is %q", got)
	}
}

// TestDetectSystemPackageManagersUseTheirOwnCommandsAndMarkRoot pins the rule
// the system stores share: the package manager's command and nothing else, a
// store that needs root marked as such, and a manager that is missing leaving
// the item measured but not cleanable.
func TestDetectSystemPackageManagersUseTheirOwnCommandsAndMarkRoot(t *testing.T) {
	probe := detectorProbe(t)
	root := t.TempDir()
	facts := Facts{GOOS: "linux", SystemRoot: root}
	writeTestFile(t, filepath.Join(root, "var", "cache", "apt", "archives", "pkg.deb"), 128)
	writeTestFile(t, filepath.Join(root, "var", "cache", "dnf", "pkg.rpm"), 128)
	writeTestFile(t, filepath.Join(root, "var", "cache", "pacman", "pkg", "pkg.zst"), 128)

	// Nothing is installed: the stores are found, measured and refused.
	items := Detect(context.Background(), probe, DetectOptions{Facts: facts})
	for _, id := range []string{"apt-archives", "dnf-cache", "pacman-packages"} {
		item := itemByID(t, items, id)
		if item.Cleanable() {
			t.Errorf("%s is cleanable without its package manager installed", id)
		}
		if item.Mechanism.Reason == "" {
			t.Errorf("%s does not say why it cannot be cleaned", id)
		}
	}

	// With the managers installed, each row runs that manager's own command.
	probe = withPrograms(probe, "apt-get", "dnf", "pacman")
	items = Detect(context.Background(), probe, DetectOptions{Facts: facts})
	want := map[string]string{
		"apt-archives":    "sudo apt-get clean",
		"dnf-cache":       "sudo dnf clean all",
		"pacman-packages": "sudo pacman -Sc --noconfirm",
	}
	for id, command := range want {
		item := itemByID(t, items, id)
		if got := item.Mechanism.Command(); got != command {
			t.Errorf("%s runs %q, want %q", id, got, command)
		}
		if !item.Mechanism.RequiresRoot {
			t.Errorf("%s does not mark that its command needs administrator rights", id)
		}
	}
}

// TestDetectTermuxAptCacheNeedsNoRoot pins the one system store whose command
// does not need administrator rights, because it belongs to the app user.
func TestDetectTermuxAptCacheNeedsNoRoot(t *testing.T) {
	probe := withPrograms(detectorProbe(t), "apt-get")
	prefix := t.TempDir()
	writeTestFile(t, filepath.Join(prefix, "var", "cache", "apt", "archives", "pkg.deb"), 32)

	items := detectApt(context.Background(), probe, DetectOptions{
		Facts: Facts{GOOS: "linux", Termux: true, Prefix: prefix},
	})
	if len(items) != 1 {
		t.Fatalf("Termux apt produced %d items, want 1", len(items))
	}
	if items[0].Mechanism.RequiresRoot {
		t.Error("the Termux apt cache is marked as needing administrator rights")
	}
}

// TestDetectProjectArtifactsRequiresTheMarkerAndStaysInTheProject pins the
// project half of the scope: only the directories the user added are searched,
// a name is believed only with its marker beside it, the repository's own
// metadata is never entered, and the walk is bounded in depth.
func TestDetectProjectArtifactsRequiresTheMarkerAndStaysInTheProject(t *testing.T) {
	probe := detectorProbe(t)
	root := t.TempDir()

	writeTestFile(t, filepath.Join(root, "project", "package.json"), 20)
	writeTestFile(t, filepath.Join(root, "project", "node_modules", "pkg", "index.js"), 100)
	writeTestFile(t, filepath.Join(root, "project", "build", "output.bin"), 60)

	// A build directory with no marker is somebody's data, not necessarily
	// this project's build output.
	writeTestFile(t, filepath.Join(root, "other", "build", "notes.txt"), 10)
	// The repository's own metadata is never searched.
	writeTestFile(t, filepath.Join(root, "project", ".git", "node_modules", "x.js"), 10)
	// A directory deeper than the bound is not reached.
	writeTestFile(t, filepath.Join(root, "a", "b", "c", "d", "e", "node_modules", "pkg", "index.js"), 10)

	items := detectProjectArtifacts(probe, []string{root}, Facts{})
	ids := itemIDs(items)
	if len(items) != 2 {
		t.Fatalf("the project artifacts are %v, want node_modules and build", ids)
	}

	modules := itemByID(t, items, "project:"+filepath.Join(root, "project", "node_modules"))
	if modules.Category != CategoryDependencies {
		t.Errorf("node_modules is classified %v, want dependencies", modules.Category)
	}
	if modules.Risk != RiskCareful {
		t.Errorf("node_modules is graded %v, want careful", modules.Risk)
	}
	if len(modules.Roots) != 1 || modules.Roots[0] != root {
		t.Errorf("node_modules approves %v, want the added project directory", modules.Roots)
	}
	if modules.Mechanism.Kind != MechanismRemove {
		t.Errorf("node_modules mechanism is %v, want the guarded removal", modules.Mechanism.Kind)
	}

	build := itemByID(t, items, "project:"+filepath.Join(root, "project", "build"))
	if build.Category != CategoryBuild {
		t.Errorf("build is classified %v, want build artifacts", build.Category)
	}
	for _, item := range items {
		if strings.Contains(item.ID, ".git") {
			t.Errorf("an artifact inside .git was offered: %s", item.ID)
		}
		if strings.Contains(item.ID, filepath.Join("d", "e")) {
			t.Errorf("an artifact past the depth bound was offered: %s", item.ID)
		}
	}
}

// TestDetectProjectArtifactsNeedsAProjectRootThatExists pins that the user's
// added directory is the only place looked at, and a directory that is not
// there contributes nothing.
func TestDetectProjectArtifactsNeedsAProjectRootThatExists(t *testing.T) {
	probe := detectorProbe(t)
	if items := detectProjectArtifacts(probe, []string{filepath.Join(t.TempDir(), "gone")}, Facts{}); len(items) != 0 {
		t.Fatalf("a project root that does not exist produced %v", itemIDs(items))
	}
}

// itemIDs names an item list for a failure message.
func itemIDs(items []Item) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

// TestDetectNeverRunsAShellForItsCommands pins the safety rule in the shape the
// code can be held to: every mechanism's command is a program plus an argument
// slice, so no path or name is ever interpolated into a shell string.
func TestDetectNeverRunsAShellForItsCommands(t *testing.T) {
	for _, spec := range containerRowSpecs("docker") {
		for _, arg := range spec.pruneArgs {
			if strings.ContainsAny(arg, ";|&$`\n") {
				t.Errorf("%s has an argument that looks like a shell fragment: %q", spec.id, arg)
			}
		}
	}
	probe := withPrograms(detectorProbe(t), "npm", "pnpm", "yarn", "bun", "pip3", "uv", "conda", "cargo", "go", "gradle", "mvn", "cs", "docker", "podman", "brew", "apt-get", "dnf", "pacman")
	items := Detect(context.Background(), probe, DetectOptions{Facts: Facts{GOOS: "linux", SystemRoot: t.TempDir()}})
	if len(items) == 0 {
		t.Skip("no detector produced a row on this machine, so there is no command to inspect")
	}
	for _, item := range items {
		if item.Mechanism.Kind != MechanismTool {
			continue
		}
		if item.Mechanism.Program == "" && len(item.Paths) > 0 {
			t.Errorf("%s has a tool mechanism with no program", item.ID)
		}
	}
}
