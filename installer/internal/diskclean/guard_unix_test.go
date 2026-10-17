//go:build unix

package diskclean

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGuardRefusesAMountPoint pins the filesystem rule: a path that is itself
// the root of another filesystem is refused, because removing it would either
// fail or empty a disk that belongs to something else.
//
// A real mount cannot be created inside a temporary directory, so the path's
// stat data is replaced with one that reports a device of its own -- which is
// exactly what a mount point looks like to the guard.
func TestGuardRefusesAMountPoint(t *testing.T) {
	probe := testProbe(t)
	scope := t.TempDir()
	store := filepath.Join(scope, "store")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatalf("create the store: %v", err)
	}
	rootInfo, err := os.Lstat(scope)
	if err != nil {
		t.Fatalf("stat the scope: %v", err)
	}
	device := deviceOf(rootInfo)
	if device == 0 {
		t.Skip("this filesystem does not expose a device, so a mount point cannot be simulated")
	}
	storeInfo, err := os.Lstat(store)
	if err != nil {
		t.Fatalf("stat the store: %v", err)
	}

	// The guard judges the canonical path, and this probe's own path can have
	// been reached through a link (macOS resolves /var to /private/var), so the
	// replacement is matched on the canonical form of both names: a path that
	// only looks different because of a link in an ancestor is the same path.
	canonicalStore := canonicalPath(probe, store)
	realLstat := probe.Lstat
	probe.Lstat = func(path string) (os.FileInfo, error) {
		info, statErr := realLstat(path)
		if statErr != nil || canonicalPath(probe, path) != canonicalStore {
			return info, statErr
		}
		return fakeDeviceInfo{FileInfo: storeInfo, device: device + 1}, nil
	}

	guard := NewGuard(probe, []string{store})
	_, err = guard.Check(canonicalPath(probe, store))
	if err == nil {
		t.Fatal("a mount point was approved for removal")
	}
	if !strings.Contains(err.Error(), "mount point") {
		t.Errorf("the refusal does not say it is a mount point: %v", err)
	}
}

// TestGuardRefusesASocketOrDevice pins the object rule for the special files a
// plain directory walk can meet: a socket is not a store, and removing it is
// not what this utility does.
func TestGuardRefusesASocketOrDevice(t *testing.T) {
	guard, probe, scope, _ := guardFixture(t)
	socketPath := filepath.Join(scope, "tool.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Skipf("this platform cannot create a unix socket: %v", err)
	}
	defer listener.Close()

	_, err = guard.Check(canonicalPath(probe, socketPath))
	if err == nil {
		t.Fatal("a socket was approved for removal")
	}
	if !strings.Contains(err.Error(), "neither a directory nor a regular file") {
		t.Errorf("the refusal does not name the object kind: %v", err)
	}
}
