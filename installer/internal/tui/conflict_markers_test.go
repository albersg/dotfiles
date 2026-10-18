package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A committed merge-conflict marker is a defect no compiler and no test of behavior will ever see:
// it is text, it can live in a file that is not code, and it silently marks one side's content as
// the resolution. The integration that put the two rounds together left one behind -- a diff3 base
// marker, the one an author checks for last -- so this is not a check for the markers I expect: it
// is every marker Git can write, over every file in the tree.
var conflictMarker = regexp.MustCompile(`^(<<<<<<<|=======$|>>>>>>>|\|\|\|\|\|\|\|)`)

func TestNoFileInTheTreeCarriesAMergeConflictMarker(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	offenders := []string{}
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > 1<<20 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		seen[rel] = true
		for i, line := range strings.Split(string(data), "\n") {
			if conflictMarker.MatchString(line) {
				offenders = append(offenders, fmt.Sprintf("%s:%d: %s", rel, i+1, line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// A walk that reaches nothing reports nothing, and reporting nothing is exactly what this test
	// is meant to do: so it first says which files it looked at, by name, before it says it found
	// no marker in any of them.
	for _, must := range []string{
		"odd/tasks/install-revert.md",
		"installer/internal/tui/state.go",
		"installer/internal/tui/revert.go",
		"docs/tui-installer.md",
		"installer/go.mod",
	} {
		if !seen[must] {
			t.Fatalf("the walk never reached %s: it saw %d files, so its silence about markers means nothing", must, len(seen))
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("%d merge-conflict marker(s) are in the tree:\n%s", len(offenders), strings.Join(offenders, "\n"))
	}
}
