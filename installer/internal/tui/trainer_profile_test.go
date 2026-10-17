package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
)

// corruptTrainerProfile writes an unreadable profile where the trainer looks for one and
// returns the directory, so a test can see what the next save does with it.
func corruptTrainerProfile(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "dotfiles-trainer")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stats.json"), []byte("{ not a profile"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestTrainerReportsAnUnreadableProfile pins S13 on the screen: entering the trainer
// with a profile that cannot be parsed says so and says where the profile was kept,
// instead of opening on a silent zero that the next save would write over.
func TestTrainerReportsAnUnreadableProfile(t *testing.T) {
	dir := corruptTrainerProfile(t)

	m := NewModel()
	m.Width, m.Height = 80, 24
	res, _ := m.startTrainer()
	got := res.(Model)

	if !strings.Contains(got.TrainerMessage, "could not be read") {
		t.Errorf("the trainer opened on an unreadable profile without a word: TrainerMessage = %q", got.TrainerMessage)
	}
	if !strings.Contains(got.TrainerMessage, "stats.json.corrupt-") {
		t.Errorf("the trainer does not say where the unreadable profile was kept: %q", got.TrainerMessage)
	}
	if got.TrainerStats == nil {
		t.Fatal("the trainer opened without stats at all")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	preserved := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "stats.json.corrupt-") {
			preserved++
		}
	}
	if preserved != 1 {
		t.Errorf("the unreadable profile was not kept beside the file: %v", entries)
	}
}

// TestTrainerOpensQuietlyOnAFirstRun is the other side: a player who has never played
// gets no warning about a profile that was never there.
func TestTrainerOpensQuietlyOnAFirstRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Width, m.Height = 80, 24
	res, _ := m.startTrainer()
	got := res.(Model)

	if got.TrainerMessage != "" {
		t.Errorf("a first run opened with the message %q", got.TrainerMessage)
	}
	if got.TrainerStats == nil {
		t.Fatal("a first run opened without a fresh profile")
	}
	if got.Screen != ScreenTrainerMenu {
		t.Errorf("the trainer opened on screen %v, want the menu", got.Screen)
	}
	if _, err := os.Stat(trainer.GetStatsPath()); !os.IsNotExist(err) {
		t.Errorf("opening the trainer wrote a profile: %v", err)
	}
}
