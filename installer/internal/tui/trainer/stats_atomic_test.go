package trainer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// statsDir points the package at a temporary profile directory for one test and puts
// the real one back afterwards, so the three rules below never touch the player's own
// profile.
func statsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	original := statsConfigPath
	statsConfigPath = dir
	t.Cleanup(func() { statsConfigPath = original })
	return dir
}

// TestSaveStatsWritesAtomically pins S13's first rule. The save goes through a file in
// the same directory and a rename, so a crash cannot leave a half-written profile where
// a readable one used to be, and a failed save leaves neither a truncated profile nor a
// temporary file behind.
func TestSaveStatsWritesAtomically(t *testing.T) {
	dir := statsDir(t)
	path := filepath.Join(dir, statsFileName)

	stats := NewUserStats()
	stats.TotalScore = 120
	if err := SaveStats(stats); err != nil {
		t.Fatalf("saving a profile failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the profile directory failed: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != statsFileName {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Errorf("a save left %v in the profile directory, want only %s", names, statsFileName)
	}

	// A save that cannot replace the file reports it and cleans up after itself: the
	// destination is a directory, so the rename cannot succeed.
	if err := os.Remove(path); err != nil {
		t.Fatalf("clearing the profile file failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(path, "in-the-way"), 0755); err != nil {
		t.Fatalf("blocking the destination failed: %v", err)
	}
	if err := SaveStats(stats); err == nil {
		t.Error("a save that could not replace the profile reported success")
	}
	entries, err = os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the profile directory failed: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() != statsFileName {
			t.Errorf("a failed save left the temporary file %s behind", entry.Name())
		}
	}
}

// TestStatsFileCarriesASchemaVersion pins S13's second rule: the JSON says which format
// it is, so a future trainer can migrate a profile instead of guessing at it.
func TestStatsFileCarriesASchemaVersion(t *testing.T) {
	dir := statsDir(t)

	stats := NewUserStats()
	stats.TotalScore = 42
	if err := SaveStats(stats); err != nil {
		t.Fatalf("saving a profile failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, statsFileName))
	if err != nil {
		t.Fatalf("reading the profile back failed: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("the profile is not JSON: %v", err)
	}
	version, ok := raw["schemaVersion"]
	if !ok {
		t.Fatalf("the profile carries no schema version: %s", data)
	}
	if got := int(version.(float64)); got != StatsSchemaVersion {
		t.Errorf("the profile declares schema version %d, want %d", got, StatsSchemaVersion)
	}

	// A profile written before the field existed still loads: version 0 is read as
	// this format, which is what makes the change backward compatible.
	older := strings.Replace(string(data), `"schemaVersion": 1,`, "", 1)
	if err := os.WriteFile(filepath.Join(dir, statsFileName), []byte(older), 0644); err != nil {
		t.Fatalf("writing the older profile failed: %v", err)
	}
	loaded, problem := LoadStatsReport()
	if loaded == nil || problem != "" {
		t.Errorf("a profile with no schema version did not load: stats=%v problem=%q", loaded, problem)
	}
}

// TestCorruptProfileIsPreservedBesideTheFile pins S13's third rule. A profile that
// cannot be parsed is moved aside rather than left where the next save would overwrite
// it, and the caller is told what happened instead of being handed a silent zero.
func TestCorruptProfileIsPreservedBesideTheFile(t *testing.T) {
	dir := statsDir(t)
	path := filepath.Join(dir, statsFileName)

	garbage := []byte("{ this is not a profile")
	if err := os.WriteFile(path, garbage, 0644); err != nil {
		t.Fatalf("writing the corrupt profile failed: %v", err)
	}

	stats, problem := LoadStatsReport()
	if stats != nil {
		t.Error("a corrupt profile loaded as if it were readable")
	}
	if problem == "" {
		t.Fatal("a corrupt profile was discarded without a word")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the profile directory failed: %v", err)
	}
	preserved := ""
	for _, entry := range entries {
		if entry.Name() != statsFileName {
			preserved = entry.Name()
		}
	}
	if preserved == "" {
		t.Fatalf("the corrupt profile was not preserved beside the file: %v", entries)
	}
	kept, err := os.ReadFile(filepath.Join(dir, preserved))
	if err != nil {
		t.Fatalf("reading the preserved profile failed: %v", err)
	}
	if string(kept) != string(garbage) {
		t.Errorf("the preserved copy is %q, want the original bytes %q", kept, garbage)
	}

	// The next save writes a fresh profile over the old path, and the preserved copy
	// is what remains of the unreadable one.
	if err := SaveStats(NewUserStats()); err != nil {
		t.Fatalf("saving over the corrupt profile failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, preserved)); err != nil {
		t.Errorf("the preserved copy did not survive the next save: %v", err)
	}
}

// TestLoadStatsReportIsQuietForAMissingProfile pins the ordinary case: no file is not a
// problem, and a caller that reports problems must not warn about a first run.
func TestLoadStatsReportIsQuietForAMissingProfile(t *testing.T) {
	statsDir(t)

	stats, problem := LoadStatsReport()
	if stats != nil {
		t.Error("a missing profile produced stats")
	}
	if problem != "" {
		t.Errorf("a missing profile was reported as a problem: %q", problem)
	}
	if LoadStats() != nil {
		t.Error("LoadStats produced stats for a missing profile")
	}
}

// TestNewerSchemaIsPreservedRatherThanDowngraded pins the other reason a profile is put
// aside: a file written by a newer trainer carries fields this build does not know, so
// loading it and saving it back would quietly delete them.
func TestNewerSchemaIsPreservedRatherThanDowngraded(t *testing.T) {
	dir := statsDir(t)
	path := filepath.Join(dir, statsFileName)

	newer := `{"schemaVersion": ` + strconv.Itoa(StatsSchemaVersion+1) + `, "totalScore": 9999, "futureField": true}`
	if err := os.WriteFile(path, []byte(newer), 0644); err != nil {
		t.Fatalf("writing the newer profile failed: %v", err)
	}

	stats, problem := LoadStatsReport()
	if stats != nil {
		t.Error("a profile from a newer trainer loaded as if this build understood it")
	}
	if problem == "" {
		t.Fatal("a profile from a newer trainer was discarded without a word")
	}
	// Preserving moves the file, so the preserved copy is the only one left and it
	// holds the newer profile's bytes, unknown fields included.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the profile directory failed: %v", err)
	}
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), statsFileName+statsNewerSuffix) {
		t.Fatalf("the newer profile was not preserved beside the file: %v", entries)
	}
	kept, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatalf("reading the preserved profile failed: %v", err)
	}
	if string(kept) != newer {
		t.Errorf("the preserved copy is %q, want the original bytes %q", kept, newer)
	}
}
