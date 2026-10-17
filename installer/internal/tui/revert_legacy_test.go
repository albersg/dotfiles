package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// A record written before this branch's per-file manifest carries paths and no
// digests. The revert cannot know what each file changed, so it must say that
// instead of reporting that everything already holds what it held before the run,
// which is exactly the case where it does not.
func TestALegacyRecordIsNotRevertedByClaimingSuccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	zsh := system.ConfigPaths()["zsh"]
	if err := os.WriteFile(zsh, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}

	stateDirPath := filepath.Join(home, "state", stateAppDir)
	if err := os.MkdirAll(stateDirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := lastInstall{
		Timestamp: time.Now().Add(-48 * time.Hour),
		Version:   "old-build",
		Files:     []string{"zsh: " + zsh},
	}
	blob, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDirPath, lastInstallFileName), blob, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := RevertLastRun(RevertOptions{})
	if err == nil {
		t.Fatal("a revert that could act on none of the files it was pointed at reported success")
	}

	if report.Reverted > 0 {
		t.Errorf("the revert claims it put %d file(s) back from a record that says nothing about what changed", report.Reverted)
	}
	if report.Unrevertible == 0 {
		t.Fatalf("the revert reported success (%d unrevertible) for a record that does not say what changed in %s",
			report.Unrevertible, zsh)
	}
	if report.Complete() {
		t.Errorf("the revert reported a complete revert of a record it cannot act on")
	}
	if report.Files[0].Status != RevertUnrevertible {
		t.Errorf("the file from the legacy record is reported as %q", report.Files[0].Status)
	}
	if data, _ := os.ReadFile(zsh); string(data) != "after" {
		t.Errorf("the revert touched a file it could not act on: %q", string(data))
	}
}
