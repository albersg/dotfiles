package trainer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// statsConfigPath is the directory for config files (can be overridden for testing)
var statsConfigPath = ""

const statsFileName = "stats.json"

// StatsSchemaVersion is the format the profile is written in. It travels in the file so
// a future trainer can migrate a profile instead of guessing at it, and it is read back
// so a profile written by a newer trainer is preserved rather than quietly downgraded
// -- this build cannot see the fields it does not know, and saving it back would delete
// them. Version 0 means a profile written before the field existed, which is this format.
const StatsSchemaVersion = 1

// The suffixes a profile this build cannot use is preserved under. It is renamed beside
// its own path rather than deleted: a corrupt profile is the only evidence of what went
// wrong, and the next save writes a fresh one over the old path either way.
const (
	statsCorruptSuffix = ".corrupt-"
	statsNewerSuffix   = ".newer-"
)

// statsFileJSON represents the JSON structure for stats
type statsFileJSON struct {
	SchemaVersion    int                            `json:"schemaVersion"`
	TotalScore       int                            `json:"totalScore"`
	CurrentStreak    int                            `json:"currentStreak"`
	BestStreak       int                            `json:"bestStreak"`
	TotalTimeSeconds int64                          `json:"totalTimeSeconds"`
	LastPlayed       string                         `json:"lastPlayed"`
	BossesDefeated   []string                       `json:"bossesDefeated"`
	Modules          map[string]*moduleProgressJSON `json:"modules"`
}

type moduleProgressJSON struct {
	LessonsCompleted    int                           `json:"lessonsCompleted"`
	LessonsTotal        int                           `json:"lessonsTotal"`
	PracticeAccuracy    float64                       `json:"practiceAccuracy"`
	PracticeAttempts    int                           `json:"practiceAttempts"`
	PracticeCorrect     int                           `json:"practiceCorrect"`
	RecentResults       []bool                        `json:"recentResults,omitempty"`
	BossDefeated        bool                          `json:"bossDefeated"`
	BossBestTimeSeconds int64                         `json:"bossBestTimeSeconds"`
	BossAttempts        int                           `json:"bossAttempts"`
	BossLivesLeft       int                           `json:"bossLivesLeft"`
	ExerciseStats       map[string]*exerciseStatsJSON `json:"exerciseStats"`
	WeakExercises       []string                      `json:"weakExercises"`
	LastPracticed       string                        `json:"lastPracticed"`
}

type exerciseStatsJSON struct {
	TotalAttempts    int   `json:"totalAttempts"`
	TotalCorrect     int   `json:"totalCorrect"`
	TotalWrong       int   `json:"totalWrong"`
	ConsecutiveRight int   `json:"consecutiveRight"`
	Mastered         bool  `json:"mastered"`
	LastAttempted    int64 `json:"lastAttempted"`
}

// GetStatsPath returns the full path to the stats file
func GetStatsPath() string {
	dir := getConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, statsFileName)
}

// getConfigDir returns the config directory
func getConfigDir() string {
	if statsConfigPath != "" {
		return statsConfigPath
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config", "dotfiles-trainer")
}

// LoadStats loads user stats from file. It is LoadStatsReport without the report, for
// callers that only describe the panel and have nowhere to say what went wrong.
func LoadStats() *UserStats {
	stats, _ := LoadStatsReport()
	return stats
}

// LoadStatsReport loads user stats and says what happened when it could not. A missing
// profile is the ordinary first run and reports nothing; a profile that cannot be parsed
// or that a newer trainer wrote is moved aside -- preserved under a suffix beside its own
// path, because it is the only evidence of what went wrong and the next save would write
// over it -- and reported in one line the trainer can show the player.
func LoadStatsReport() (*UserStats, string) {
	path := GetStatsPath()
	if path == "" {
		return nil, ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ""
		}
		return nil, "The trainer profile could not be read: " + err.Error()
	}

	var fileStats statsFileJSON
	if err := json.Unmarshal(data, &fileStats); err != nil {
		return nil, preserveStatsFile(path, statsCorruptSuffix,
			"The trainer profile could not be read")
	}
	if fileStats.SchemaVersion > StatsSchemaVersion {
		return nil, preserveStatsFile(path, statsNewerSuffix,
			fmt.Sprintf("The trainer profile is from a newer trainer (format %d, this one reads %d)",
				fileStats.SchemaVersion, StatsSchemaVersion))
	}

	return statsFromFile(fileStats), ""
}

// preserveStatsFile renames a profile this build cannot use to a sibling path and
// returns the sentence the trainer shows about it. A rename that fails is reported too:
// the player has to know the profile was not preserved either way.
func preserveStatsFile(path, suffix, reason string) string {
	preserved := fmt.Sprintf("%s%s%d", path, suffix, time.Now().Unix())
	if err := os.Rename(path, preserved); err != nil {
		return reason + ", and it could not be kept: " + err.Error()
	}
	return reason + "; it was kept as " + filepath.Base(preserved)
}

// statsFromFile converts the profile's own shape into the one the trainer works with.
// Every field is read here and nowhere else, so a format change has one place to land.
func statsFromFile(fileStats statsFileJSON) *UserStats {
	// Convert JSON struct to UserStats
	stats := &UserStats{
		TotalScore:     fileStats.TotalScore,
		CurrentStreak:  fileStats.CurrentStreak,
		BestStreak:     fileStats.BestStreak,
		TotalTime:      time.Duration(fileStats.TotalTimeSeconds) * time.Second,
		ModuleProgress: make(map[ModuleID]*ModuleProgress),
		BossesDefeated: make([]ModuleID, 0),
	}

	if fileStats.LastPlayed != "" {
		stats.LastPlayed, _ = time.Parse(time.RFC3339, fileStats.LastPlayed)
	}

	for _, boss := range fileStats.BossesDefeated {
		stats.BossesDefeated = append(stats.BossesDefeated, ModuleID(boss))
	}

	for modID, modProgress := range fileStats.Modules {
		mp := &ModuleProgress{
			LessonsCompleted: modProgress.LessonsCompleted,
			LessonsTotal:     modProgress.LessonsTotal,
			PracticeAccuracy: modProgress.PracticeAccuracy,
			PracticeAttempts: modProgress.PracticeAttempts,
			PracticeCorrect:  modProgress.PracticeCorrect,
			RecentResults:    modProgress.RecentResults,
			BossDefeated:     modProgress.BossDefeated,
			BossBestTime:     time.Duration(modProgress.BossBestTimeSeconds) * time.Second,
			BossAttempts:     modProgress.BossAttempts,
			BossLivesLeft:    modProgress.BossLivesLeft,
			ExerciseStats:    make(map[string]*ExerciseStats),
			WeakExercises:    modProgress.WeakExercises,
		}
		if modProgress.LastPracticed != "" {
			mp.LastPracticed, _ = time.Parse(time.RFC3339, modProgress.LastPracticed)
		}
		// Load exercise stats
		for exID, exStats := range modProgress.ExerciseStats {
			mp.ExerciseStats[exID] = &ExerciseStats{
				TotalAttempts:    exStats.TotalAttempts,
				TotalCorrect:     exStats.TotalCorrect,
				TotalWrong:       exStats.TotalWrong,
				ConsecutiveRight: exStats.ConsecutiveRight,
				Mastered:         exStats.Mastered,
				LastAttempted:    exStats.LastAttempted,
			}
		}
		stats.ModuleProgress[ModuleID(modID)] = mp
	}

	return stats
}

// SaveStats saves user stats to file
func SaveStats(stats *UserStats) error {
	if stats == nil {
		return errors.New("stats cannot be nil")
	}

	path := GetStatsPath()
	if path == "" {
		return errors.New("could not determine stats path")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Convert to JSON struct
	lastPlayed := ""
	if !stats.LastPlayed.IsZero() {
		lastPlayed = stats.LastPlayed.Format(time.RFC3339)
	}

	fileStats := statsFileJSON{
		SchemaVersion:    StatsSchemaVersion,
		TotalScore:       stats.TotalScore,
		CurrentStreak:    stats.CurrentStreak,
		BestStreak:       stats.BestStreak,
		TotalTimeSeconds: int64(stats.TotalTime.Seconds()),
		LastPlayed:       lastPlayed,
		BossesDefeated:   make([]string, 0),
		Modules:          make(map[string]*moduleProgressJSON),
	}

	for _, boss := range stats.BossesDefeated {
		fileStats.BossesDefeated = append(fileStats.BossesDefeated, string(boss))
	}

	for modID, modProgress := range stats.ModuleProgress {
		lastPracticed := ""
		if !modProgress.LastPracticed.IsZero() {
			lastPracticed = modProgress.LastPracticed.Format(time.RFC3339)
		}
		weakExercises := modProgress.WeakExercises
		if weakExercises == nil {
			weakExercises = []string{}
		}

		// Convert exercise stats
		exerciseStats := make(map[string]*exerciseStatsJSON)
		for exID, exStats := range modProgress.ExerciseStats {
			exerciseStats[exID] = &exerciseStatsJSON{
				TotalAttempts:    exStats.TotalAttempts,
				TotalCorrect:     exStats.TotalCorrect,
				TotalWrong:       exStats.TotalWrong,
				ConsecutiveRight: exStats.ConsecutiveRight,
				Mastered:         exStats.Mastered,
				LastAttempted:    exStats.LastAttempted,
			}
		}

		fileStats.Modules[string(modID)] = &moduleProgressJSON{
			LessonsCompleted:    modProgress.LessonsCompleted,
			LessonsTotal:        modProgress.LessonsTotal,
			PracticeAccuracy:    modProgress.PracticeAccuracy,
			PracticeAttempts:    modProgress.PracticeAttempts,
			PracticeCorrect:     modProgress.PracticeCorrect,
			RecentResults:       modProgress.RecentResults,
			BossDefeated:        modProgress.BossDefeated,
			BossBestTimeSeconds: int64(modProgress.BossBestTime.Seconds()),
			BossAttempts:        modProgress.BossAttempts,
			BossLivesLeft:       modProgress.BossLivesLeft,
			ExerciseStats:       exerciseStats,
			WeakExercises:       weakExercises,
			LastPracticed:       lastPracticed,
		}
	}

	data, err := json.MarshalIndent(fileStats, "", "  ")
	if err != nil {
		return err
	}

	// The profile is written beside itself and renamed into place, so a crash -- or a
	// full disk -- cannot leave a half-written file where a readable profile used to
	// be. The temporary file lives in the same directory as its target, which is what
	// makes the rename atomic: a rename across filesystems is a copy.
	tmp, err := os.CreateTemp(dir, statsFileName+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	// A save that fails at any point removes its own temporary file: a leftover would
	// be mistaken for a profile by the next run, and the player would never see it.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, path)
}

// ResetStats deletes the stats file
func ResetStats() error {
	path := GetStatsPath()
	if path == "" {
		return nil
	}
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil // Not an error if file doesn't exist
	}
	return err
}
