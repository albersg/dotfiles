package diskclean

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recordingRemover is the destructive double: it records every unlink the
// removal makes and can be told to fail specific paths. No removal test ever
// deletes anything outside its own temporary tree.
type recordingRemover struct {
	probe  Probe
	order  []string
	failed map[string]error
}

func newRecordingRemover(probe Probe) *recordingRemover {
	return &recordingRemover{probe: probe, failed: map[string]error{}}
}

// install puts the recorder in front of the real os.Remove.
func (r *recordingRemover) install() Probe {
	probe := r.probe
	probe.Remove = func(path string) error {
		r.order = append(r.order, path)
		if err, ok := r.failed[path]; ok {
			return err
		}
		return os.Remove(path)
	}
	return probe
}

// TestRemoveTreeEmptiesBottomUpAndCountsWhatItFreed pins the order and the
// accounting: every child is gone before its parent is unlinked, the files are
// counted once, and the directory itself is the last thing removed.
func TestRemoveTreeEmptiesBottomUpAndCountsWhatItFreed(t *testing.T) {
	probe := testProbe(t)
	root := filepath.Join(t.TempDir(), "store")
	writeTestFile(t, filepath.Join(root, "one.bin"), 512)
	writeTestFile(t, filepath.Join(root, "nested", "two.bin"), 512)

	remover := newRecordingRemover(probe)
	result := removeTree(context.Background(), remover.install(), root, noCeilings())

	if result.Err != "" {
		t.Fatalf("the removal failed: %s", result.Err)
	}
	if _, err := os.Lstat(root); !os.IsNotExist(err) {
		t.Fatalf("the tree is still there: %v", err)
	}
	if result.RemovedFiles != 2 {
		t.Errorf("the removal counted %d files, want 2", result.RemovedFiles)
	}
	if result.RemovedBytes <= 0 {
		t.Errorf("the removal freed %d bytes, want a measured allocation", result.RemovedBytes)
	}
	if len(remover.order) == 0 || remover.order[len(remover.order)-1] != root {
		t.Errorf("the directory was not removed last: %v", remover.order)
	}
	for index, path := range remover.order {
		if path == root {
			continue
		}
		if index > 0 && remover.order[index-1] == root {
			t.Errorf("the directory was removed before its child %s", path)
		}
	}
}

// TestRemoveTreeUnlinksALinkAndNotItsTarget pins the link rule at removal time:
// the link goes, the tree it pointed at is untouched.
func TestRemoveTreeUnlinksALinkAndNotItsTarget(t *testing.T) {
	probe := testProbe(t)
	root := filepath.Join(t.TempDir(), "store")
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(root, "mine.bin"), 64)
	writeTestFile(t, filepath.Join(outside, "theirs.bin"), 4096)
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("this platform cannot create a symbolic link: %v", err)
	}

	remover := newRecordingRemover(probe)
	result := removeTree(context.Background(), remover.install(), root, noCeilings())
	if result.Err != "" {
		t.Fatalf("the removal failed: %s", result.Err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "theirs.bin")); err != nil {
		t.Fatalf("the link's target was removed: %v", err)
	}
	for _, path := range remover.order {
		if strings.HasPrefix(path, outside) {
			t.Errorf("the removal reached through the link: %s", path)
		}
	}
}

// TestRemoveTreeRefusesATreeContainingAMountPoint pins the refusal: a tree
// that contains another filesystem is refused whole, and nothing is unlinked.
func TestRemoveTreeRefusesATreeContainingAMountPoint(t *testing.T) {
	probe := testProbe(t)
	root := filepath.Join(t.TempDir(), "store")
	writeTestFile(t, filepath.Join(root, "data.bin"), 128)
	mount := filepath.Join(root, "mounted")
	writeTestFile(t, filepath.Join(mount, "other.bin"), 128)

	simulateMountAt(t, &probe, root, mount)
	remover := newRecordingRemover(probe)
	result := removeTree(context.Background(), remover.install(), root, noCeilings())

	if !strings.Contains(result.Err, "mount point") {
		t.Fatalf("the refusal does not name the mount: %s", result.Err)
	}
	if len(remover.order) != 0 {
		t.Errorf("a refused tree was partially removed: %v", remover.order)
	}
	if _, err := os.Lstat(filepath.Join(root, "data.bin")); err != nil {
		t.Errorf("the refused tree lost a file: %v", err)
	}
}

// TestRemoveTreeRecordsAnEntryItCannotRemove pins the partial-failure rule:
// one locked file is a reason beside the bytes that did come back, not a silent
// success and not the end of the run.
func TestRemoveTreeRecordsAnEntryItCannotRemove(t *testing.T) {
	probe := testProbe(t)
	root := filepath.Join(t.TempDir(), "store")
	locked := writeTestFile(t, filepath.Join(root, "locked.bin"), 256)
	writeTestFile(t, filepath.Join(root, "free.bin"), 256)

	remover := newRecordingRemover(probe)
	remover.failed[locked] = errors.New("permission denied")
	result := removeTree(context.Background(), remover.install(), root, noCeilings())

	if len(result.Skipped) == 0 || !strings.Contains(result.Skipped[0], "locked.bin") {
		t.Errorf("the locked file is not named in %v", result.Skipped)
	}
	if result.SkippedTotal == 0 {
		t.Error("the removal did not count the entries it could not remove")
	}
	if result.Err == "" {
		t.Error("a removal that left entries behind reported no error")
	}
	if result.RemovedBytes <= 0 {
		t.Error("the files that were removed freed nothing")
	}
	if _, err := os.Lstat(filepath.Join(root, "free.bin")); !os.IsNotExist(err) {
		t.Errorf("the file that could be removed is still there: %v", err)
	}
}

// TestRemoveTreeStopsWhenTheContextEnds pins cancellation: a cancelled run
// removes nothing more, and says it was cancelled.
func TestRemoveTreeStopsWhenTheContextEnds(t *testing.T) {
	probe := testProbe(t)
	root := filepath.Join(t.TempDir(), "store")
	writeTestFile(t, filepath.Join(root, "data.bin"), 128)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	remover := newRecordingRemover(probe)
	result := removeTree(ctx, remover.install(), root, noCeilings())

	if !result.Cancelled {
		t.Error("a cancelled removal did not report the cancellation")
	}
	if len(remover.order) != 0 {
		t.Errorf("a cancelled removal unlinked something: %v", remover.order)
	}
}

// TestRemoveTreeRefusesALinkAsItsOwnRoot pins that a tree root which is a link
// is refused: the walk and the guard both refuse it, and so does the removal.
func TestRemoveTreeRefusesALinkAsItsOwnRoot(t *testing.T) {
	probe := testProbe(t)
	target := t.TempDir()
	writeTestFile(t, filepath.Join(target, "data.bin"), 128)
	link := filepath.Join(t.TempDir(), "store")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("this platform cannot create a symbolic link: %v", err)
	}

	result := removeTree(context.Background(), probe, link, noCeilings())
	if result.Err == "" || !strings.Contains(result.Err, "symbolic link") {
		t.Fatalf("a link was accepted as a tree root: %s", result.Err)
	}
	if _, err := os.Lstat(filepath.Join(target, "data.bin")); err != nil {
		t.Errorf("the link's target was removed: %v", err)
	}
}
