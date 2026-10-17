package tui

import (
	"path/filepath"
	"strings"
	"testing"
)

// The Backup Location paragraph must appear once. The branch's paragraph already carries main's
// "Directory-based configs ..." sentence inside it, so a second copy is a merge-resolution
// duplicate, not a restored line: a whole-line audit cannot see text that is already present as a
// suffix of a longer line, and that is exactly how the duplicate got in.
//
// The ninth pass handed this over after finding the duplicate in the merge result.
func TestTheBackupLocationParagraphAppearsOnce(t *testing.T) {
	text := readRepoFile(t, filepath.Join("docs", "tui-installer.md"))
	// Counted on a flattened copy: the sentence is a fact about the guide, not about where this
	// paragraph happens to be wrapped, and a re-wrap is exactly what produced the duplicate in the
	// first place (L20, L22).
	flat := strings.Join(strings.Fields(text), " ")
	const sentence = "Directory-based configs such as `~/.oh-my-zsh` are backed up recursively"
	if n := strings.Count(flat, sentence); n != 1 {
		t.Fatalf("the Backup Location paragraph appears %d times, want 1 (merge-resolution duplicate)", n)
	}
	if n := strings.Count(flat, "Each backup carries a `backup.json` manifest"); n != 1 {
		t.Fatalf("the manifest paragraph appears %d times, want 1", n)
	}
}
