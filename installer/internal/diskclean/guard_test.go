package diskclean

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// guardFixture builds a store inside a scope the guard approves, all under
// temporary directories, so no guard test can touch anything real.
func guardFixture(t *testing.T) (Guard, Probe, string, string) {
	t.Helper()
	probe := testProbe(t)
	scope := t.TempDir()
	store := filepath.Join(scope, "store")
	writeTestFile(t, filepath.Join(store, "data.bin"), 1024)
	return NewGuard(probe, []string{store}), probe, scope, store
}

// TestGuardApprovesAStorePathAndVerifyKeepsIt is the happy path both ends of
// the flow depend on: a path under its own approved store is approved once,
// and the same object verifies immediately before a removal.
func TestGuardApprovesAStorePathAndVerifyKeepsIt(t *testing.T) {
	guard, probe, _, store := guardFixture(t)
	canonical := canonicalPath(probe, store)

	approval, err := guard.Check(canonical)
	if err != nil {
		t.Fatalf("the store under its own approved root was refused: %v", err)
	}
	if approval.Path != canonical {
		t.Errorf("the approval names %s, want %s", approval.Path, canonical)
	}
	if !approval.Dir {
		t.Error("the approval does not record that the path is a directory")
	}
	if err := guard.Verify(approval); err != nil {
		t.Errorf("the unchanged path did not verify: %v", err)
	}
}

// TestGuardRefusesAnyPathOutsideTheApprovedScope pins the scope rule: a path
// the detector never declared is not removable, even when it exists and is a
// directory.
func TestGuardRefusesAnyPathOutsideTheApprovedScope(t *testing.T) {
	guard, probe, _, _ := guardFixture(t)
	elsewhere := filepath.Join(t.TempDir(), "other-store")
	writeTestFile(t, filepath.Join(elsewhere, "data.bin"), 64)

	_, err := guard.Check(canonicalPath(probe, elsewhere))
	if err == nil {
		t.Fatal("a path outside every approved root was approved")
	}
	if !strings.Contains(err.Error(), "outside the approved scope") {
		t.Errorf("the refusal does not name the scope: %v", err)
	}
}

// TestGuardRefusesTheHomeRootAndTheFilesystemRoot pins the blocked roots: the
// home directory and the filesystem root are never a removal target, whatever
// a detector claimed.
func TestGuardRefusesTheHomeRootAndTheFilesystemRoot(t *testing.T) {
	probe := testProbe(t)
	home := probe.HomeDir
	cache := filepath.Join(home, ".cache", "tool")
	writeTestFile(t, filepath.Join(cache, "data.bin"), 64)
	guard := NewGuard(probe, []string{cache, home})

	for _, refused := range []string{home, "/"} {
		if _, err := guard.Check(refused); err == nil {
			t.Errorf("%s was approved", refused)
		}
	}
	if roots := guard.ApprovedRoots(); len(roots) != 1 || roots[0] != canonicalPath(probe, cache) {
		t.Errorf("the approved roots are %v, want only the cache under the home directory", roots)
	}
}

// TestGuardRefusesAProtectedZoneEvenWhenADetectorApprovedIt pins the second
// line of defense: a path inside the configuration or credential zones is
// refused, and an approval that only exists because a detector was wrong is
// dropped before it can be used.
func TestGuardRefusesAProtectedZoneEvenWhenADetectorApprovedIt(t *testing.T) {
	probe := testProbe(t)
	zone := filepath.Join(probe.HomeDir, ".config", "some-tool")
	writeTestFile(t, filepath.Join(zone, "config.json"), 32)
	guard := NewGuard(probe, []string{zone})

	if _, err := guard.Check(canonicalPath(probe, zone)); err == nil {
		t.Fatal("a store inside the configuration zone was approved")
	}
	if len(guard.ApprovedRoots()) != 0 {
		t.Errorf("the protected zone survived construction: %v", guard.ApprovedRoots())
	}

	// The credential zone behaves the same way.
	keys := filepath.Join(probe.HomeDir, ".ssh", "store")
	writeTestFile(t, filepath.Join(keys, "known_hosts"), 16)
	second := NewGuard(probe, []string{keys})
	if _, err := second.Check(canonicalPath(probe, keys)); err == nil {
		t.Error("a store inside the credential zone was approved")
	}
}

// TestGuardRefusesASymlinkLeaf pins the link rule: the path itself being a
// link is refused, because removing the name would free nothing and the link
// could point anywhere.
func TestGuardRefusesASymlinkLeaf(t *testing.T) {
	guard, probe, scope, _ := guardFixture(t)
	target := t.TempDir()
	link := filepath.Join(scope, "linked-store")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("this platform cannot create a symbolic link: %v", err)
	}

	// The raw path is handed over deliberately: the guard is what detects that
	// the final component is a link, and resolving it first would hide that from
	// the test.
	_, err := guard.Check(link)
	if err == nil {
		t.Fatal("a symbolic link was approved as a removal target")
	}
	if !strings.Contains(err.Error(), "symbolic link") {
		t.Errorf("the refusal does not say it was a link: %v", err)
	}
	_ = probe
}

// TestGuardRefusesAPathThatResolvesOutsideItsScope pins the resolved check: a
// directory reached through a link that leaves the approved scope is refused,
// which is what keeps a link from widening a removal.
func TestGuardRefusesAPathThatResolvesOutsideItsScope(t *testing.T) {
	guard, probe, scope, _ := guardFixture(t)
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(outside, "data.bin"), 512)
	if err := os.Symlink(outside, filepath.Join(scope, "escape")); err != nil {
		t.Skipf("this platform cannot create a symbolic link: %v", err)
	}

	escaping := canonicalPath(probe, filepath.Join(scope, "escape"))
	_, err := guard.Check(escaping)
	if err == nil {
		t.Fatal("a path that resolves outside the approved scope was approved")
	}
	if !strings.Contains(err.Error(), "outside the approved scope") {
		t.Errorf("the refusal does not name the scope: %v", err)
	}
}

// TestGuardRefusesGitMetadata pins the repository rule: a .git directory, or
// anything that would contain one, is never removed by this utility.
func TestGuardRefusesGitMetadata(t *testing.T) {
	guard, probe, scope, _ := guardFixture(t)
	gitDir := filepath.Join(scope, "repo", ".git")
	writeTestFile(t, filepath.Join(gitDir, "config"), 128)

	_, err := guard.Check(canonicalPath(probe, gitDir))
	if err == nil {
		t.Fatal(".git was approved for removal")
	}
	if !strings.Contains(err.Error(), "Git repository") {
		t.Errorf("the refusal does not name the repository: %v", err)
	}
}

// TestGuardRefusesAnAmbiguousPath pins the ambiguity rule: a path that is not
// already canonical is refused rather than silently cleaned.
func TestGuardRefusesAnAmbiguousPath(t *testing.T) {
	guard, _, scope, store := guardFixture(t)
	// Spelled rather than built with filepath.Join, which would clean the
	// ambiguity away before the guard ever saw it.
	ambiguous := scope + string(os.PathSeparator) + "other" + string(os.PathSeparator) +
		".." + string(os.PathSeparator) + filepath.Base(store)

	_, err := guard.Check(ambiguous)
	if err == nil {
		t.Fatal("a non-canonical path was approved")
	}
	if !strings.Contains(err.Error(), "canonical") {
		t.Errorf("the refusal does not name the ambiguity: %v", err)
	}
}

// TestGuardRefusesAMissingPath pins the object rule: there is nothing to
// remove where nothing exists. The special-file half of the rule (sockets,
// devices) is measured beside it by the platform-tagged guard test.
func TestGuardRefusesAMissingPath(t *testing.T) {
	guard, probe, scope, _ := guardFixture(t)

	if _, err := guard.Check(canonicalPath(probe, filepath.Join(scope, "never-created"))); err == nil {
		t.Error("a path that does not exist was approved")
	}
}

// TestVerifyRefusesAPathReplacedAfterThePreview is the TOCTOU guard: the preview
// approved one object, and the unlink is about to meet another.
//
// The replacement's identity is supplied rather than read: a filesystem that
// recycles inode numbers -- ext4 in its default layout, for one -- hands a
// directory recreated at the same path the very same device and inode pair, so a
// test that recreated the directory and hoped for a new pair would pass on tmpfs
// and fail on ext4. The identity the guard must refuse is the one this test
// states, and the honest limit that remains is pinned by its sibling below.
func TestVerifyRefusesAPathReplacedAfterThePreview(t *testing.T) {
	guard, probe, _, store := guardFixture(t)
	approval, err := guard.Check(canonicalPath(probe, store))
	if err != nil {
		t.Fatalf("the fixture store was refused: %v", err)
	}

	if err := os.RemoveAll(store); err != nil {
		t.Fatalf("replace the store: %v", err)
	}
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatalf("recreate the store: %v", err)
	}

	// The filesystem gave the replacement a different inode: the guard has to see
	// that and refuse.
	trueIdentity := probe.Identity
	guard.probe.Identity = func(path string, info os.FileInfo) fileID {
		if trueIdentity != nil {
			return trueIdentity(path, info)
		}
		id := fileIDOf(info)
		if canonicalPath(probe, path) == approval.Path {
			id.inode++
		}
		return id
	}

	if err := guard.Verify(approval); err == nil {
		t.Fatal("a directory recreated after the preview verified as the approved object")
	} else if !strings.Contains(err.Error(), "no longer") && !strings.Contains(err.Error(), "replaced") {
		t.Errorf("the refusal does not say the object changed: %v", err)
	}
}

// TestVerifyCannotTellASwapTheFilesystemHides pins the residual risk of the
// identity check instead of pretending it away: when a filesystem hands the
// directory recreated at the approved path the same device and inode pair, the
// pair is all this check has, and it verifies. Everything else is still
// re-established before an unlink -- the scope, the blocklist, the kind of
// object, no symlink leaf, no mount point -- and the window is the one between
// the preview and the removal.
func TestVerifyCannotTellASwapTheFilesystemHides(t *testing.T) {
	guard, probe, _, store := guardFixture(t)
	approval, err := guard.Check(canonicalPath(probe, store))
	if err != nil {
		t.Fatalf("the fixture store was refused: %v", err)
	}

	if err := os.RemoveAll(store); err != nil {
		t.Fatalf("replace the store: %v", err)
	}
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatalf("recreate the store: %v", err)
	}

	// The recreated directory answers with the pair the approval recorded.
	samePair := fileID{device: approval.Device, inode: approval.Inode}
	guard.probe.Identity = func(string, os.FileInfo) fileID { return samePair }

	if err := guard.Verify(approval); err != nil {
		t.Fatalf("the guard refused a path whose pair still matches, so this test no longer states the limit: %v", err)
	}
}

// TestVerifyRefusesAPathThatLeftItsScopeAfterThePreview pins the other half of
// the same race: the scope is re-established at the unlink, not trusted from
// the preview.
func TestVerifyRefusesAPathThatLeftItsScopeAfterThePreview(t *testing.T) {
	guard, probe, _, store := guardFixture(t)
	approval, err := guard.Check(canonicalPath(probe, store))
	if err != nil {
		t.Fatalf("the fixture store was refused: %v", err)
	}
	if err := os.RemoveAll(store); err != nil {
		t.Fatalf("remove the store: %v", err)
	}
	if err := os.Symlink(t.TempDir(), store); err != nil {
		t.Skipf("this platform cannot create a symbolic link: %v", err)
	}

	if err := guard.Verify(approval); err == nil {
		t.Fatal("a path replaced by a symbolic link verified")
	}
}

// TestCanonicalPathResolvesWhatExistsAndKeepsTheRest pins the helper the guard
// compares scopes with: a store that does not exist yet still has a canonical
// name.
func TestCanonicalPathResolvesWhatExistsAndKeepsTheRest(t *testing.T) {
	probe := testProbe(t)
	scope := t.TempDir()
	missing := filepath.Join(scope, "not-created-yet", "store")

	got := canonicalPath(probe, missing)
	want := filepath.Join(canonicalPath(probe, scope), "not-created-yet", "store")
	if got != want {
		t.Errorf("canonicalPath(%s) = %s, want %s", missing, got, want)
	}
	if canonicalPath(probe, "relative/path") != "" {
		t.Error("a relative path was canonicalized")
	}
}
