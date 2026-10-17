//go:build unix

package diskclean

import (
	"os"
	"path/filepath"
	"testing"
)

// simulateMountAt makes the directory at mount look like a filesystem of its
// own to the walk and the removal survey: its entry in the parent's listing
// reports a device other than the one the root is on. A real mount cannot be
// created inside a temporary directory, and no destructive test may touch one.
func simulateMountAt(t *testing.T, probe *Probe, root, mount string) {
	t.Helper()

	rootInfo, err := os.Lstat(root)
	if err != nil {
		t.Fatalf("stat the root: %v", err)
	}
	device := deviceOf(rootInfo)
	if device == 0 {
		t.Skip("this filesystem does not expose a device, so a mount cannot be simulated")
	}
	mountInfo, err := os.Lstat(mount)
	if err != nil {
		t.Fatalf("stat the simulated mount point: %v", err)
	}

	parent := filepath.Dir(mount)
	name := filepath.Base(mount)
	realReadDir := probe.ReadDir
	probe.ReadDir = func(dir string) ([]os.DirEntry, error) {
		entries, readErr := realReadDir(dir)
		if readErr != nil || dir != parent {
			return entries, readErr
		}
		for i, entry := range entries {
			if entry.Name() == name {
				entries[i] = fakeDeviceEntry{
					DirEntry: entry,
					info:     fakeDeviceInfo{FileInfo: mountInfo, device: device + 1},
				}
			}
		}
		return entries, nil
	}
}
