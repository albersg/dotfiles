// Package release holds guards about the artifacts a release publishes: the
// Homebrew formula and the things that read it.
//
// It exists because of a real defect. The formula lost four `end` tokens in the
// 0.6.4 tap commit, when a script that rewrote the sha256 lines replaced them
// whole instead of keeping the terminator that had been glued to their end. The
// file stopped parsing, and `brew install` and `brew upgrade` failed for three
// releases before anyone noticed, because nothing in this repository ever read
// the formula.
//
// The guard asks Ruby rather than guessing. A first attempt counted openers and
// terminators with a regular expression and failed on the correct file, because
// `elsif` is part of the `if` it continues and needs no `end` of its own: a guard
// that fails on correct input is worse than no guard. Ruby's own compiler is the
// only thing that knows Ruby, and Homebrew ships it, which is what `brew` runs
// before it will resolve anything.
package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// formulaPath is the formula in this repository. The same file lives in the tap
// repository and the two are kept byte-identical, so a guard here catches the
// file before it is copied there.
const formulaPath = "../../../homebrew-tap/Formula/dotfiles.rb"

// compileScript asks Homebrew's Ruby to compile the file, which is exactly what
// brew does before it resolves a formula.
const compileScript = `RubyVM::InstructionSequence.compile_file(ARGV[0]); puts "compiles"`

// TestTheFormulaCompiles is the guard that would have caught the defect. It uses
// Homebrew's own Ruby, because that is the interpreter that reads this file and
// the one whose answer matters.
//
// When brew is not installed the test says what it did not check rather than
// passing quietly. In CI brew is present, so this runs there on every change.
func TestTheFormulaCompiles(t *testing.T) {
	path, err := filepath.Abs(formulaPath)
	if err != nil {
		t.Fatalf("resolving %s: %v", formulaPath, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the formula is not where this guard looks: %v", err)
	}

	brew, err := exec.LookPath("brew")
	if err != nil {
		// A skip is honest on a machine that has no Homebrew; in CI it would be a
		// silent hole, so there it fails. GitHub's ubuntu runners ship Homebrew,
		// which is why the check belongs in the Go job rather than in a step of
		// its own.
		if os.Getenv("CI") != "" {
			t.Fatalf("brew is not installed in CI, so %s was not compiled and this guard checked nothing", formulaPath)
		}
		t.Skipf("brew is not installed here, so %s was NOT compiled; CI runs this check with Homebrew present", formulaPath)
	}

	out, err := exec.Command(brew, "ruby", "-e", compileScript, path).CombinedOutput()
	if err != nil {
		t.Errorf("%s does not compile, and brew will refuse to install or upgrade from it:\n%s", formulaPath, out)
		return
	}
	if !strings.Contains(string(out), "compiles") {
		t.Errorf("brew ruby said %q, which does not prove the formula compiled", out)
	}
}

// TestTheFormulaNamesTheVersionAndTheFourHashes pins what a formula has to be:
// one version and one sha256 per platform. A release updates all five together
// or it does not update them, and this is the half that needs nothing installed.
func TestTheFormulaNamesTheVersionAndTheFourHashes(t *testing.T) {
	body, err := os.ReadFile(formulaPath)
	if err != nil {
		t.Fatalf("reading %s: %v", formulaPath, err)
	}
	text := string(body)

	if !regexp.MustCompile(`(?m)^\s*version "[0-9]+\.[0-9]+\.[0-9]+"$`).MatchString(text) {
		t.Error("the formula does not name a version in the form version \"x.y.z\"")
	}
	hashes := regexp.MustCompile(`(?m)^\s*sha256 "[0-9a-f]{64}"$`).FindAllString(text, -1)
	if len(hashes) != 4 {
		t.Errorf("the formula carries %d sha256 lines, and it must carry exactly four: one per platform", len(hashes))
	}
	for _, asset := range []string{"dotfiles-darwin-arm64", "dotfiles-darwin-amd64", "dotfiles-linux-arm64", "dotfiles-linux-amd64"} {
		if !strings.Contains(text, asset) {
			t.Errorf("the formula never names %s", asset)
		}
	}
	// Every sha256 must sit on its own line, with no terminator glued to its end.
	// The defect this package exists for came from exactly that shape: the four
	// terminators rode along at the end of these lines, so a script that replaced
	// the lines deleted them.
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "sha256") && strings.Contains(line, "end") {
			t.Errorf("a sha256 line carries a terminator: %q\nEvery `end` belongs on its own line, or the next edit that rewrites this line will delete it.", strings.TrimSpace(line))
		}
	}
}
