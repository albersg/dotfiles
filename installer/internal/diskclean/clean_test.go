package diskclean

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCleanRunsTheOwnerCommandAndMeasuresTheRelease pins the whole path of a
// tool-owned cache: the command that runs is the owner's, the release is
// measured by walking the store again, and the filesystem delta is reported as
// measured.
func TestCleanRunsTheOwnerCommandAndMeasuresTheRelease(t *testing.T) {
	probe := withPrograms(testProbe(t), "npm")
	cache := filepath.Join(probe.HomeDir, ".npm", "_cacache")
	writeTestFile(t, filepath.Join(cache, "one.bin"), 4096)
	writeTestFile(t, filepath.Join(cache, "two.bin"), 4096)

	var ranProgram string
	var ranArgs []string
	probe.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		ranProgram, ranArgs = name, args
		// The owner's command empties its own cache; the test simulates only
		// what that command does to the temporary store.
		return []byte("npm cache cleaned"), os.RemoveAll(cache)
	}

	items := []Item{{
		ID: "npm-cache", Tool: "npm", Title: "The npm cache",
		Paths:     []string{cache},
		Mechanism: Mechanism{Kind: MechanismTool, Program: "npm", Args: []string{"cache", "clean", "--force"}},
	}}
	guard := GuardFor(probe, items)
	report := Clean(context.Background(), probe, guard, BuildPlan(items, guard), CleanOptions{})

	if ranProgram != "npm" || strings.Join(ranArgs, " ") != "cache clean --force" {
		t.Fatalf("the run executed %s %v, want npm cache clean --force", ranProgram, ranArgs)
	}
	result := report.Results[0]
	if !result.Ran || result.Error != "" {
		t.Fatalf("the item failed: ran=%v error=%s", result.Ran, result.Error)
	}
	if result.Freed <= 0 {
		t.Errorf("the result reports %d bytes freed from a store that was emptied", result.Freed)
	}
	if result.FreedSource != SourceWalk {
		t.Errorf("the release source is %v, want a measured walk", result.FreedSource)
	}
	if _, err := os.Lstat(cache); !os.IsNotExist(err) {
		t.Errorf("the store is still there: %v", err)
	}
	// Whether the filesystem delta can be measured depends on the machine: the
	// free space has to grow around the run, and another process writing to the
	// same filesystem while this test runs can hide the release. So the run owes
	// the screen an answer rather than a measurement it may not be able to take --
	// a measured delta, or the reason there is none -- and never a zero printed
	// as an answer or a negative release. The hardlink test below follows the
	// same rule for the same reason.
	if report.FreedMeasured {
		if report.FreedBytes < 0 {
			t.Errorf("the filesystem reported a negative release: %d", report.FreedBytes)
		}
	} else {
		if report.FreedNote == "" {
			t.Error("the filesystem delta was not measured and no reason was given")
		}
		if report.FreedBytes != 0 {
			t.Errorf("an unmeasured report carries %d bytes", report.FreedBytes)
		}
	}
}

// TestCleanAttemptsARootCommandWithoutAPromptAndNamesTheManualOne pins the
// administrator rule: the command is attempted with `sudo -n`, a refusal is a
// reported reason, and the exact command the user can run is in that reason.
func TestCleanAttemptsARootCommandWithoutAPromptAndNamesTheManualOne(t *testing.T) {
	probe := testProbe(t)
	archives := filepath.Join(t.TempDir(), "archives")
	writeTestFile(t, filepath.Join(archives, "pkg.deb"), 1024)

	var executed []string
	probe.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		executed = append([]string{name}, args...)
		return []byte("sudo: a password is required"), errors.New("exit status 1")
	}

	items := []Item{{
		ID: "apt-archives", Tool: "apt", Title: "The apt package cache",
		Paths: []string{archives},
		Mechanism: Mechanism{
			Kind: MechanismTool, Program: "apt-get", Args: []string{"clean"}, RequiresRoot: true,
		},
	}}
	guard := GuardFor(probe, items)
	report := Clean(context.Background(), probe, guard, BuildPlan(items, guard), CleanOptions{})

	if strings.Join(executed, " ") != "sudo -n apt-get clean" {
		t.Fatalf("the run executed %v, want sudo -n apt-get clean", executed)
	}
	result := report.Results[0]
	if !result.Ran {
		t.Error("a refused command is reported as never attempted")
	}
	if !strings.Contains(result.Error, "administrator rights") {
		t.Errorf("the result does not name the missing rights: %s", result.Error)
	}
	if !strings.Contains(result.Error, "sudo apt-get clean") {
		t.Errorf("the result does not name the command the user can run: %s", result.Error)
	}
}

// TestCleanRemovesWhatTheGuardApprovedAndReportsTheRelease pins the guarded
// removal end to end, on a temporary tree and with a recording double for the
// unlink.
func TestCleanRemovesWhatTheGuardApprovedAndReportsTheRelease(t *testing.T) {
	probe := testProbe(t)
	store := filepath.Join(t.TempDir(), "store")
	writeTestFile(t, filepath.Join(store, "one.bin"), 4096)
	writeTestFile(t, filepath.Join(store, "nested", "two.bin"), 4096)

	remover := newRecordingRemover(probe)
	recording := remover.install()
	items := []Item{{
		ID: "store", Title: "The store",
		Paths:     []string{store},
		Mechanism: Mechanism{Kind: MechanismRemove},
	}}
	guard := GuardFor(recording, items)
	report := Clean(context.Background(), recording, guard, BuildPlan(items, guard), CleanOptions{})

	result := report.Results[0]
	if !result.Ran || result.Error != "" {
		t.Fatalf("the removal failed: ran=%v error=%s", result.Ran, result.Error)
	}
	if result.Freed <= 0 {
		t.Errorf("the removal reports %d bytes freed", result.Freed)
	}
	if !result.Succeeded() {
		t.Error("a removal that emptied the tree is not reported as a success")
	}
	if _, err := os.Lstat(store); !os.IsNotExist(err) {
		t.Errorf("the tree is still there: %v", err)
	}
	if len(remover.order) == 0 {
		t.Error("nothing was unlinked through the recording double")
	}
}

// TestCleanNeverUnlinksAnEngineOrManagerStore pins the rule that protects the
// engines and the package managers: even when their stores are selected and
// planned, nothing inside them is unlinked by this package -- the owner's own
// command runs, or the finding is reported, and the store is left standing for
// the owner to maintain.
func TestCleanNeverUnlinksAnEngineOrManagerStore(t *testing.T) {
	probe := withPrograms(testProbe(t), "docker")
	// The run removes nothing, so the filesystem cannot have released anything on
	// its behalf. The free-space reading is pinned to one answer rather than taken
	// from the machine: the real reading is the whole device's, and another process
	// writing while the suite runs moves it, which is what made this guard flake
	// once while the release preflight ran. What it pins is the code's rule -- with
	// nothing released, no measured release is claimed -- not the runner's disk.
	probe.FreeSpace = func(string) (int64, error) { return 1 << 30, nil }
	engineStore := filepath.Join(t.TempDir(), "docker")
	writeTestFile(t, filepath.Join(engineStore, "image", "layer.bin"), 8192)
	archives := filepath.Join(t.TempDir(), "archives")
	writeTestFile(t, filepath.Join(archives, "pkg.deb"), 4096)

	var ranProgram string
	probe.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		ranProgram = name + " " + strings.Join(args, " ")
		return []byte("Deleted Images:\nTotal reclaimed space: 1GB\n"), nil
	}

	items := []Item{
		{
			ID: "docker-store", Tool: "docker", Title: "The Docker store",
			Paths:           []string{engineStore},
			VolatileObjects: []string{"images", "containers"},
			Mechanism:       Mechanism{Kind: MechanismTool, Program: "docker", Args: []string{"image", "prune", "-f"}},
		},
		{
			ID: "apt-archives", Tool: "apt", Title: "The apt cache",
			Paths:     []string{archives},
			Mechanism: Mechanism{Kind: MechanismReportOnly, Reason: "apt-get is not on PATH here"},
		},
	}
	remover := newRecordingRemover(probe)
	recording := remover.install()
	guard := GuardFor(recording, items)
	report := Clean(context.Background(), recording, guard, BuildPlan(items, guard), CleanOptions{})

	if ranProgram != "docker image prune -f" {
		t.Fatalf("the engine's own command ran as %q, want docker image prune -f", ranProgram)
	}
	for _, path := range remover.order {
		t.Errorf("a store owned by an engine or a manager was unlinked directly: %s", path)
	}
	if _, err := os.Lstat(filepath.Join(engineStore, "image", "layer.bin")); err != nil {
		t.Errorf("the engine's store was touched outside its own command: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(archives, "pkg.deb")); err != nil {
		t.Errorf("a finding with no mechanism had something removed: %v", err)
	}
	if report.Results[1].Ran {
		t.Errorf("the report-only finding claims it ran: %+v", report.Results[1])
	}
	if report.FreedMeasured && report.FreedBytes > 0 {
		t.Errorf("the run claims the filesystem released %d bytes while nothing was removed", report.FreedBytes)
	}
	if !report.FreedMeasured && report.FreedNote == "" {
		t.Error("the filesystem delta was not measured and no reason was given")
	}
	if !report.FreedMeasured && report.FreedBytes != 0 {
		t.Errorf("an unmeasured report carries %d bytes", report.FreedBytes)
	}
}

// TestCleanRefusesAPathReplacedAfterThePlan pins the TOCTOU guard at the last
// moment: the plan approved a directory, the unlink meets a link, and nothing
// is removed.
func TestCleanRefusesAPathReplacedAfterThePlan(t *testing.T) {
	probe := testProbe(t)
	store := filepath.Join(t.TempDir(), "store")
	writeTestFile(t, filepath.Join(store, "data.bin"), 512)
	items := []Item{{
		ID: "store", Title: "The store",
		Paths:     []string{store},
		Mechanism: Mechanism{Kind: MechanismRemove},
	}}
	guard := GuardFor(probe, items)
	plan := BuildPlan(items, guard)
	if !plan.Items[0].Runs() {
		t.Fatalf("the fixture plan is blocked: %s", plan.Items[0].Blocked)
	}

	if err := os.RemoveAll(store); err != nil {
		t.Fatalf("replace the store: %v", err)
	}
	if err := os.Symlink(t.TempDir(), store); err != nil {
		t.Skipf("this platform cannot create a symbolic link: %v", err)
	}

	report := Clean(context.Background(), probe, guard, plan, CleanOptions{})
	result := report.Results[0]
	if !result.Refused {
		t.Errorf("a path replaced after the plan was not refused: %+v", result)
	}
	if result.Freed != 0 {
		t.Errorf("the refused item reports %d bytes freed", result.Freed)
	}
	if _, err := os.Lstat(store); err != nil {
		t.Errorf("the replaced path was removed anyway: %v", err)
	}
}

// TestCleanReportsAPartialRemovalAndStillCountsWhatCameBack pins the partial
// rule: a locked file is named, the rest of the tree is still removed, and the
// bytes that came back are reported beside the failure.
func TestCleanReportsAPartialRemovalAndStillCountsWhatCameBack(t *testing.T) {
	probe := testProbe(t)
	store := filepath.Join(t.TempDir(), "store")
	locked := writeTestFile(t, filepath.Join(store, "locked.bin"), 4096)
	writeTestFile(t, filepath.Join(store, "free.bin"), 4096)

	remover := newRecordingRemover(probe)
	remover.failed[locked] = errors.New("operation not permitted")
	recording := remover.install()
	items := []Item{{
		ID: "store", Title: "The store",
		Paths:     []string{store},
		Mechanism: Mechanism{Kind: MechanismRemove},
	}}
	guard := GuardFor(recording, items)
	report := Clean(context.Background(), recording, guard, BuildPlan(items, guard), CleanOptions{})

	result := report.Results[0]
	if result.Error == "" {
		t.Fatal("a partial removal reported no error")
	}
	if len(result.Skipped) == 0 {
		t.Error("the partial removal names none of the entries it could not remove")
	}
	if result.Freed <= 0 {
		t.Error("the files that were removed freed nothing")
	}
	if failed := report.Failed(); len(failed) != 1 || failed[0].Item.ID != "store" {
		t.Errorf("the failed list is %v, want the partial item", failed)
	}
}

// TestCleanAsksTheEngineAgainForWhatItsCommandFreed pins the container result:
// there is no tree to walk, so the same store's number is asked for again and
// the difference is the release, labelled as the engine's own accounting.
func TestCleanAsksTheEngineAgainForWhatItsCommandFreed(t *testing.T) {
	probe := withPrograms(testProbe(t), "docker")
	pruned := false
	probe.Run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "docker" {
			return nil, errors.New("unexpected program")
		}
		switch {
		case len(args) > 0 && args[0] == "system":
			if pruned {
				return []byte(`{"Type":"Images","TotalCount":"0","Size":"0B","Reclaimable":"0B"}`), nil
			}
			return []byte(`{"Type":"Images","TotalCount":"3","Size":"1.5GB","Reclaimable":"700MB"}`), nil
		case len(args) > 0 && args[0] == "image":
			pruned = true
			return []byte("Deleted Images"), nil
		}
		return nil, errors.New("unexpected command")
	}

	usage := ParseSystemDF([]byte(`{"Type":"Images","TotalCount":"3","Size":"1.5GB","Reclaimable":"700MB"}`))
	items := containerStoresFor("docker", "docker", usage)
	if len(items) != 1 {
		t.Fatalf("the fixture produced %d rows, want the images row", len(items))
	}
	guard := GuardFor(probe, items)
	report := Clean(context.Background(), probe, guard, BuildPlan(items, guard), CleanOptions{})

	result := report.Results[0]
	if !result.Succeeded() {
		t.Fatalf("the engine command failed: %+v", result)
	}
	if want := int64(700e6); result.Freed != want {
		t.Errorf("the release is %d, want the engine's %d", result.Freed, want)
	}
	if result.FreedSource != SourceTool {
		t.Errorf("the release source is %v, want the engine's own accounting", result.FreedSource)
	}
}

// TestCleanCancelsBeforeItTouchesAnything pins cancellation as a run-level
// fact: every item is reported as not run, and no command is executed.
func TestCleanCancelsBeforeItTouchesAnything(t *testing.T) {
	probe := testProbe(t)
	probe.Run = func(context.Context, string, ...string) ([]byte, error) {
		t.Error("a cancelled run executed a command")
		return nil, nil
	}
	store := filepath.Join(t.TempDir(), "store")
	writeTestFile(t, filepath.Join(store, "data.bin"), 128)
	items := []Item{
		{ID: "remove", Title: "The store", Paths: []string{store}, Mechanism: Mechanism{Kind: MechanismRemove}},
		{ID: "tool", Title: "A cache", Mechanism: Mechanism{Kind: MechanismTool, Program: "npm", Args: []string{"cache", "clean"}}},
	}
	guard := GuardFor(probe, items)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report := Clean(ctx, probe, guard, BuildPlan(items, guard), CleanOptions{})

	if !report.Cancelled {
		t.Error("the report does not say the run was cancelled")
	}
	for _, result := range report.Results {
		if result.Ran {
			t.Errorf("%s ran in a cancelled run", result.Item.ID)
		}
		if !result.Cancelled {
			t.Errorf("%s is not marked as cancelled", result.Item.ID)
		}
	}
	if _, err := os.Lstat(store); err != nil {
		t.Errorf("a cancelled run removed something: %v", err)
	}
}

// TestCleanReportsProgressPerItem pins the progress line a screen shows while a
// run is in flight.
func TestCleanReportsProgressPerItem(t *testing.T) {
	probe := testProbe(t)
	items := []Item{
		{ID: "one", Title: "One", Mechanism: Mechanism{Kind: MechanismReportOnly, Reason: "nothing here"}},
		{ID: "two", Title: "Two", Mechanism: Mechanism{Kind: MechanismReportOnly, Reason: "nothing here"}},
	}
	guard := GuardFor(probe, items)

	var seen []string
	Clean(context.Background(), probe, guard, BuildPlan(items, guard), CleanOptions{
		Progress: func(done, total int, item Item) {
			seen = append(seen, item.ID)
			if done > total {
				t.Errorf("progress ran past the end: %d of %d", done, total)
			}
		},
	})
	if len(seen) != 2 || seen[0] != "one" || seen[1] != "two" {
		t.Errorf("the progress reports are %v, want one per item in order", seen)
	}
}

// TestRemovingOneOfTwoHardlinksDoesNotClaimTheOther pin the honest share of the
// hardlink problem: the report leads with the filesystem's own delta, which is
// zero when a name still holds the file, and no invented release is printed.
func TestCleanReportsTheFilesystemDeltaAsTheRealRelease(t *testing.T) {
	probe := testProbe(t)
	root := filepath.Join(t.TempDir(), "store")
	first := writeTestFile(t, filepath.Join(root, "one.bin"), 1<<20)
	second := filepath.Join(t.TempDir(), "keep.bin")
	if err := os.Link(first, second); err != nil {
		t.Skipf("this filesystem does not support hardlinks: %v", err)
	}

	items := []Item{{
		ID: "store", Title: "The store",
		Paths:     []string{root},
		Mechanism: Mechanism{Kind: MechanismRemove},
	}}
	guard := GuardFor(probe, items)
	report := Clean(context.Background(), probe, guard, BuildPlan(items, guard), CleanOptions{})

	if report.FreedMeasured {
		// The other name still holds the file, so the filesystem cannot have
		// released a megabyte. The bound is generous because other processes write
		// during a test, and it is the direction that matters.
		if report.FreedBytes >= 1<<20 {
			t.Errorf("the filesystem claims %d bytes freed while the file is still linked at %s",
				report.FreedBytes, second)
		}
	} else {
		// Nothing came back because the other name still holds the bytes. That is
		// the honest answer for this run, and it is a reason rather than a zero: a
		// screen that printed "released 0 B" here would be claiming a measurement
		// of a release that never happened.
		if report.FreedNote == "" {
			t.Error("the report has no release and no reason for one")
		}
		if report.FreedBytes != 0 {
			t.Errorf("an unmeasured report carries %d bytes", report.FreedBytes)
		}
	}
	if report.Results[0].Freed <= 0 {
		t.Errorf("the run did not measure the bytes of the name it unlinked: %+v", report.Results[0])
	}
	if _, err := os.Lstat(second); err != nil {
		t.Fatalf("the other hardlink disappeared: %v", err)
	}
}
