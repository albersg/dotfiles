//go:build unix

package diskclean

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// fakeDeviceInfo is a file's real stat data with one fact replaced: the
// filesystem it reports living on. It is how a mount point is simulated inside
// a temporary directory, where no test may create a real one.
type fakeDeviceInfo struct {
	os.FileInfo
	device uint64
}

func (f fakeDeviceInfo) Sys() any {
	return &syscall.Stat_t{Dev: uint64(f.device), Ino: uint64(4242)}
}

// fakeDeviceEntry is a directory entry whose Info reports that simulated
// device.
type fakeDeviceEntry struct {
	os.DirEntry
	info os.FileInfo
}

func (e fakeDeviceEntry) Info() (os.FileInfo, error) { return e.info, nil }

// TestWalkStopsAtAFilesystemBoundary pins the rule that keeps a store's number
// honest on a host where something is mounted inside it: the walk names the
// boundary, does not descend past it, and never counts another filesystem's
// bytes as this store's.
func TestWalkStopsAtAFilesystemBoundary(t *testing.T) {
	probe := testProbe(t)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "counted.bin"), 2048)

	mount := filepath.Join(root, "mounted-store")
	writeTestFile(t, filepath.Join(mount, "not-counted.bin"), 1<<20)
	mountInfo, err := os.Lstat(mount)
	if err != nil {
		t.Fatalf("stat the simulated mount point: %v", err)
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		t.Fatalf("stat the root: %v", err)
	}
	rootDevice := deviceOf(rootInfo)
	if rootDevice == 0 {
		t.Skip("this filesystem does not expose a device, so a boundary cannot be simulated")
	}

	// The entry for the subdirectory reports a device of its own, which is
	// exactly what a mount point looks like to the walk. The real entry is kept
	// underneath so the name and the type keep answering.
	probe.ReadDir = func(dir string) ([]os.DirEntry, error) {
		entries, readErr := os.ReadDir(dir)
		if readErr != nil || dir != root {
			return entries, readErr
		}
		for i, entry := range entries {
			if entry.Name() == "mounted-store" {
				entries[i] = fakeDeviceEntry{
					DirEntry: entry,
					info:     fakeDeviceInfo{FileInfo: mountInfo, device: rootDevice + 1},
				}
			}
		}
		return entries, nil
	}

	result := Walk(context.Background(), probe, root, "item", NewDedupe(), DefaultLimits())
	if result.Logical != 2048 || result.Files != 1 {
		t.Errorf("the walk counted %d bytes in %d files, want 2048 in 1: it crossed the boundary",
			result.Logical, result.Files)
	}
	if len(result.Mounts) != 1 || !strings.Contains(result.Mounts[0], "mounted-store") {
		t.Errorf("the boundary is not named in %v", result.Mounts)
	}
}

// TestWalkDoesNotFollowADirectorySymlinkInsideTheTree is the boundary case a
// real mount cannot cover: a link to a directory of another tree is neither
// descended into nor counted.
func TestWalkDoesNotFollowADirectorySymlinkInsideTheTree(t *testing.T) {
	probe := testProbe(t)
	root := t.TempDir()
	other := t.TempDir()
	writeTestFile(t, filepath.Join(root, "mine.bin"), 1024)
	writeTestFile(t, filepath.Join(other, "theirs.bin"), 8192)
	if err := os.Symlink(other, filepath.Join(root, "linked")); err != nil {
		t.Fatalf("create the link: %v", err)
	}

	result := Walk(context.Background(), probe, root, "item", NewDedupe(), DefaultLimits())
	if result.Logical != 1024 {
		t.Errorf("the walk counted %d bytes, want 1024: it followed the directory link", result.Logical)
	}
	if result.Symlinks != 1 {
		t.Errorf("the walk counted %d links, want 1", result.Symlinks)
	}
}
