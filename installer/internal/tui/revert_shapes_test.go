package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// Every shape of record this repository has ever written, and the answer a revert
// owes it. Two of them keep being confused because they look alike: a record
// written before per-file changes says which paths the run wrote and nothing about
// the files, and a record of a run that changed nothing says it wrote none. Telling
// those apart by recognising a shape works until a third shape appears -- which is
// exactly what happened twice on this branch -- so the rule reads what the record
// says and this table pins every shape that exists.
func TestEveryRecordShapeGetsTheAnswerItsWordsSupport(t *testing.T) {
	shapes := []struct {
		name string
		// record builds the record under test from the path the run wrote and the
		// copy it took.
		record func(zsh, copyPath, beforeDigest, afterDigest string) lastInstall
		// wantUnrevertible is how many files the revert must report as ones it
		// cannot put back.
		wantUnrevertible int
		// wantErr says whether that answer is a failure for the command line.
		wantErr bool
		// wantContent is the content the file must hold afterwards.
		wantContent string
		// wantWayBackDir says whether the reason may name a backup directory: a
		// record that names a copy can hand over a path that works, and a record
		// that does not must not invent one.
		wantWayBackDir bool
	}{
		{
			name: "the current record of a run that wrote a file",
			record: func(zsh, copyPath, before, after string) lastInstall {
				return lastInstall{
					Timestamp: time.Now(),
					Version:   "current",
					Files:     []string{"zsh: " + zsh},
					Entries:   []installEntry{{Key: "zsh", Path: zsh, Action: installActionReplaced, Backup: copyPath}},
					Changes: []fileChange{{
						Key: "zsh", Path: zsh, Action: fileActionModified,
						Before: before, After: after, Backup: copyPath,
					}},
				}
			},
			wantContent: "before-run",
		},
		{
			name: "the current record of a run that changed nothing",
			record: func(zsh, copyPath, before, after string) lastInstall {
				return lastInstall{
					Timestamp: time.Now(),
					Version:   "current",
					Files:     []string{},
					Entries:   []installEntry{{Key: "zsh", Path: zsh, Action: installActionFound}},
					Changes:   []fileChange{},
				}
			},
			wantContent: "after-run",
		},
		{
			name: "the current record of a run that changed nothing and took a copy anyway",
			record: func(zsh, copyPath, before, after string) lastInstall {
				return lastInstall{
					Timestamp: time.Now(),
					Version:   "current",
					Files:     []string{},
					Entries: []installEntry{
						{Key: "zsh", Path: zsh, Action: installActionBackedUp, Backup: copyPath},
						{Key: "gitconfig", Path: filepath.Join(filepath.Dir(zsh), ".gitconfig"), Action: installActionFound},
					},
					Changes: []fileChange{},
				}
			},
			wantContent: "after-run",
		},
		{
			// The shape origin/main writes and every installed machine therefore
			// holds: entries with an action and a copy, files for the paths the run
			// wrote, and no per-file changes at all.
			name: "the record the released build writes for a run that replaced a file",
			record: func(zsh, copyPath, before, after string) lastInstall {
				return lastInstall{
					Timestamp: time.Now(),
					Version:   "released",
					Files:     []string{"zsh: " + zsh},
					Entries:   []installEntry{{Key: "zsh", Path: zsh, Action: installActionReplaced, Backup: copyPath}},
				}
			},
			wantUnrevertible: 1,
			wantErr:          true,
			wantContent:      "after-run",
			wantWayBackDir:   true,
		},
		{
			name: "the record the released build writes for a run that created a file",
			record: func(zsh, copyPath, before, after string) lastInstall {
				return lastInstall{
					Timestamp: time.Now(),
					Version:   "released",
					Files:     []string{"zsh: " + zsh},
					Entries:   []installEntry{{Key: "zsh", Path: zsh, Action: installActionCreated}},
				}
			},
			wantUnrevertible: 1,
			wantErr:          true,
			wantContent:      "after-run",
			wantWayBackDir:   false,
		},
		{
			name: "the record the released build writes for a run that changed nothing",
			record: func(zsh, copyPath, before, after string) lastInstall {
				return lastInstall{
					Timestamp: time.Now(),
					Version:   "released",
					Files:     []string{},
					Entries:   []installEntry{{Key: "zsh", Path: zsh, Action: installActionFound}},
				}
			},
			wantContent: "after-run",
		},
		{
			name: "the oldest record, which names paths and nothing else",
			record: func(zsh, copyPath, before, after string) lastInstall {
				return lastInstall{
					Timestamp: time.Now(),
					Version:   "oldest",
					Files:     []string{"zsh: " + zsh},
				}
			},
			wantUnrevertible: 1,
			wantErr:          true,
			wantContent:      "after-run",
			wantWayBackDir:   false,
		},
		{
			name: "a record that names nothing at all",
			record: func(zsh, copyPath, before, after string) lastInstall {
				return lastInstall{Timestamp: time.Now(), Version: "empty"}
			},
			wantContent: "after-run",
		},
	}

	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

			zsh := system.ConfigPaths()["zsh"]
			if err := os.WriteFile(zsh, []byte("after-run"), 0o644); err != nil {
				t.Fatal(err)
			}
			backupDir := filepath.Join(home, ".dotfiles-backup-2026-01-01-000000")
			if err := os.MkdirAll(backupDir, 0o755); err != nil {
				t.Fatal(err)
			}
			copyPath := filepath.Join(backupDir, "zsh")
			if err := os.WriteFile(copyPath, []byte("before-run"), 0o644); err != nil {
				t.Fatal(err)
			}

			before := sha256Of("before-run")
			after := sha256Of("after-run")
			if err := writeLastInstall(shape.record(zsh, copyPath, before, after)); err != nil {
				t.Fatal(err)
			}

			report, err := RevertLastRun(RevertOptions{})
			if (err != nil) != shape.wantErr {
				t.Fatalf("the revert's error is %v; this shape is %s", err, map[bool]string{true: "one this command cannot act on", false: "one it can account for"}[shape.wantErr])
			}
			if report.Unrevertible != shape.wantUnrevertible {
				t.Errorf("the revert reports %d unrevertible file(s), want %d: %v", report.Unrevertible, shape.wantUnrevertible, err)
			}
			if data, _ := os.ReadFile(zsh); string(data) != shape.wantContent {
				t.Errorf("%s holds %q afterwards, want %q", zsh, string(data), shape.wantContent)
			}

			if shape.wantUnrevertible > 0 {
				// A record this command cannot act on has to hand the reader a way
				// back that works, not a sentence about one.
				reason := report.Files[0].Reason
				if shape.wantWayBackDir {
					if !strings.Contains(reason, backupDir) {
						t.Fatalf("the reason does not name the backup directory %s: %q", backupDir, reason)
					}
					if _, planErr := RestoreBackupDir(backupDir, true); planErr != nil {
						t.Errorf("the directory the reason names cannot be restored: %v", planErr)
					}
				} else if strings.Contains(reason, "--restore="+backupDir) {
					t.Errorf("the reason claims a copy this record never named: %q", reason)
				}
			}
		})
	}
}
