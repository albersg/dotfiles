package tui

import (
	"errors"
	"strings"
	"testing"
)

// gaiVerifierBlock builds a marked block from the exact marker constants, so
// these tests cannot drift from the strings the step writes.
func gaiVerifierBlock(inner string) string {
	return gaiMarkerStart + "\n" + inner + gaiMarkerEnd + "\n"
}

// TestGAIStepDoesNotDeleteContentAfterAnUnterminatedBlock pins F1: a start
// marker with no end marker used to run to the end of the file, so replacing the
// "block" deleted every line after it -- including a precedence the owner had
// set. The step must write nothing and say why.
func TestGAIStepDoesNotDeleteContentAfterAnUnterminatedBlock(t *testing.T) {
	logs := captureGAILogs(t)
	stubGAIDpkg(t, false, "", nil)

	unterminated := gaiMarkerStart + "\n" +
		"precedence ::ffff:0:0/96 100\n" +
		"precedence ::ffff:0:0/96 20\n"
	path := gaiFixture(t, unterminated)

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("an unterminated block must not fail the step: %v", err)
	}
	if got := string(readGAIFile(t, path)); got != unterminated {
		t.Fatalf("the file changed under an unterminated block:\n got:\n%s\nwant:\n%s", got, unterminated)
	}
	if !strings.Contains(string(readGAIFile(t, path)), "precedence ::ffff:0:0/96 20") {
		t.Errorf("the line after the unterminated block was deleted")
	}
	if joined := strings.Join(*logs, "\n"); !strings.Contains(joined, "missing its end marker") {
		t.Errorf("the step did not name the missing end marker; logs: %v", *logs)
	}
}

// TestGAIStepLeavesAForeignPrecedenceOutsideTheMarkedBlock pins F2: a precedence
// the owner set outside the block governs the file, so even though the block
// itself is present and stale, the step must write nothing and report the line
// it found rather than replacing the block over the owner's choice.
func TestGAIStepLeavesAForeignPrecedenceOutsideTheMarkedBlock(t *testing.T) {
	logs := captureGAILogs(t)
	stubGAIDpkg(t, false, "", nil)

	foreign := "precedence ::ffff:0:0/96 20\n" + gaiVerifierBlock("precedence ::ffff:0:0/96 10\n")
	path := gaiFixture(t, foreign)

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("a foreign precedence must not fail the step: %v", err)
	}
	if got := string(readGAIFile(t, path)); got != foreign {
		t.Fatalf("the file changed despite the foreign precedence:\n got:\n%s\nwant:\n%s", got, foreign)
	}
	joined := strings.Join(*logs, "\n")
	if !strings.Contains(joined, "precedence ::ffff:0:0/96 20") {
		t.Errorf("the step did not name the precedence it found; logs: %v", *logs)
	}
	if !strings.Contains(joined, "outside the managed block") {
		t.Errorf("the step did not say the precedence is outside the block; logs: %v", *logs)
	}
	if strings.Contains(joined, "Restored") {
		t.Errorf("the step restored a value despite the foreign precedence; logs: %v", *logs)
	}
}

// TestGAIStepLeavesADuplicatedBlockAlone pins F3: two start markers are not a
// shape the step owns, so it must not pick one and delete the other's contents.
func TestGAIStepLeavesADuplicatedBlockAlone(t *testing.T) {
	logs := captureGAILogs(t)
	stubGAIDpkg(t, false, "", nil)

	duplicated := gaiVerifierBlock("precedence ::ffff:0:0/96 100\n") +
		gaiVerifierBlock("precedence ::ffff:0:0/96 100\n")
	path := gaiFixture(t, duplicated)

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("a duplicated block must not fail the step: %v", err)
	}
	if got := string(readGAIFile(t, path)); got != duplicated {
		t.Fatalf("the file changed under a duplicated block:\n got:\n%s\nwant:\n%s", got, duplicated)
	}
	if joined := strings.Join(*logs, "\n"); !strings.Contains(joined, "more than one marked block") {
		t.Errorf("the step did not name the duplicated block; logs: %v", *logs)
	}
}

// TestGAIStepRestoresAValueInsideTheBlockAndNamesIt pins F4: the block is the
// installer's, so a stale value inside it is restored -- and the log names the
// value it replaced, so the restore is never a silent edit.
func TestGAIStepRestoresAValueInsideTheBlockAndNamesIt(t *testing.T) {
	logs := captureGAILogs(t)
	stubGAIDpkg(t, false, "", nil)

	path := gaiFixture(t, gaiVerifierBlock("precedence ::ffff:0:0/96 20\n"))

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("the gai step failed: %v", err)
	}
	if got, want := string(readGAIFile(t, path)), string(gaiBlock()); got != want {
		t.Fatalf("the block was not restored to the deciding line:\n got:\n%s\nwant:\n%s", got, want)
	}
	joined := strings.Join(*logs, "\n")
	if !strings.Contains(joined, "Restored") {
		t.Errorf("the step restored the value without saying so; logs: %v", *logs)
	}
	if !strings.Contains(joined, "precedence ::ffff:0:0/96 20") {
		t.Errorf("the step did not name the value it replaced; logs: %v", *logs)
	}
	if !strings.Contains(joined, gaiPrecedenceLine) {
		t.Errorf("the step did not name the value it wrote; logs: %v", *logs)
	}
}

// TestGAIStepWarnsAndContinuesWhenTheWriteFails pins F5: a refused write to
// /etc/gai.conf must be reported and the run must continue, the way a missing
// dpkg is, rather than aborting an interactive install or exiting non-zero.
func TestGAIStepWarnsAndContinuesWhenTheWriteFails(t *testing.T) {
	logs := captureGAILogs(t)
	stubGAIDpkg(t, false, "", nil)

	previous := gaiApplyArtifactContent
	gaiApplyArtifactContent = func([]byte, string, string) error {
		return errors.New("read-only file system")
	}
	t.Cleanup(func() { gaiApplyArtifactContent = previous })

	before := gaiFactoryFile
	path := gaiFixture(t, before)

	if err := stepInstallGAIConf(&Model{}); err != nil {
		t.Fatalf("a failed write must not fail the step: %v", err)
	}
	if got := string(readGAIFile(t, path)); got != before {
		t.Errorf("the file changed although the write failed:\n got:\n%s\nwant:\n%s", got, before)
	}
	joined := strings.Join(*logs, "\n")
	if !strings.Contains(joined, "Could not write") {
		t.Errorf("the step did not report the failed write; logs: %v", *logs)
	}
	if !strings.Contains(joined, "read-only file system") {
		t.Errorf("the step did not carry the writer's error; logs: %v", *logs)
	}
}
