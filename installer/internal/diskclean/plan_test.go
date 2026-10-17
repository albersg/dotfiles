package diskclean

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildPlanNamesTheExactCommandItWouldRun pins the preview's honesty: the
// plan carries the command, not a description of it.
func TestBuildPlanNamesTheExactCommandItWouldRun(t *testing.T) {
	probe := withPrograms(testProbe(t), "npm")
	cache := filepath.Join(probe.HomeDir, ".npm", "_cacache")
	writeTestFile(t, filepath.Join(cache, "entry.bin"), 128)

	items := []Item{{
		ID:        "npm-cache",
		Title:     "The npm cache",
		Paths:     []string{cache},
		Mechanism: Mechanism{Kind: MechanismTool, Program: "npm", Args: []string{"cache", "clean", "--force"}},
	}}

	plan := BuildPlan(items, GuardFor(probe, items))
	if len(plan.Items) != 1 {
		t.Fatalf("the plan holds %d items, want 1", len(plan.Items))
	}
	planned := plan.Items[0]
	if planned.Action != "npm cache clean --force" {
		t.Errorf("the action is %q, want the exact command", planned.Action)
	}
	if !planned.Runs() {
		t.Errorf("a runnable item is blocked: %s", planned.Blocked)
	}
}

// TestBuildPlanBlocksWhatTheGuardRefuses pins that a path the plan cannot
// legally remove is refused at preview time, with the reason, and contributes
// nothing to the expected release.
func TestBuildPlanBlocksWhatTheGuardRefuses(t *testing.T) {
	probe := testProbe(t)
	inside := filepath.Join(t.TempDir(), "inside")
	outside := filepath.Join(t.TempDir(), "outside")
	writeTestFile(t, filepath.Join(inside, "data.bin"), 256)
	writeTestFile(t, filepath.Join(outside, "data.bin"), 256)

	items := []Item{
		{ID: "inside", Title: "inside", Paths: []string{inside}, Mechanism: Mechanism{Kind: MechanismRemove},
			Size: Size{Physical: 256, Logical: 256}},
		{ID: "outside", Title: "outside", Paths: []string{outside}, Mechanism: Mechanism{Kind: MechanismRemove},
			Size: Size{Physical: 256, Logical: 256}},
	}
	// Only the first item's path is approved, so the second is refused.
	guard := GuardFor(probe, items[:1])
	plan := BuildPlan(items, guard)

	if plan.Items[0].Blocked != "" {
		t.Errorf("the approved item is blocked: %s", plan.Items[0].Blocked)
	}
	if !strings.Contains(plan.Items[1].Blocked, "outside the approved scope") {
		t.Errorf("the refused item's reason is %q, want it to name the scope", plan.Items[1].Blocked)
	}
	if plan.Expected != 256 {
		t.Errorf("the expected release is %d, want only the approved item's 256", plan.Expected)
	}
	if len(plan.ApprovedRoots) != 1 || plan.ApprovedRoots[0] != canonicalPath(probe, inside) {
		t.Errorf("the approved roots are %v, want only the inside item", plan.ApprovedRoots)
	}
}

// TestBuildPlanRefusesAReportOnlyFinding pins that a store this utility
// refuses to clean is shown with its reason rather than silently dropped.
func TestBuildPlanRefusesAReportOnlyFinding(t *testing.T) {
	probe := testProbe(t)
	items := []Item{{
		ID:        "unknown-store",
		Title:     "A store nothing can clean",
		Mechanism: Mechanism{Kind: MechanismReportOnly, Reason: "the owner has no maintenance command"},
		Size:      Size{Physical: 1024},
	}}

	plan := BuildPlan(items, GuardFor(probe, items))
	if plan.Items[0].Runs() {
		t.Error("a report-only finding runs")
	}
	if !strings.Contains(plan.Items[0].Blocked, "the owner has no maintenance command") {
		t.Errorf("the refusal does not carry the reason: %s", plan.Items[0].Blocked)
	}
	if plan.Expected != 0 {
		t.Errorf("a refused item contributes %d to the expected release", plan.Expected)
	}
}

// TestGuardForApprovesTheDeclaredRootsNotTheArtifactPath pins the project
// scope: what is approved is the directory the user added, not the artifact
// path a detector computed.
func TestGuardForApprovesTheDeclaredRootsNotTheArtifactPath(t *testing.T) {
	probe := testProbe(t)
	root := t.TempDir()
	artifact := filepath.Join(root, "project", "node_modules")
	writeTestFile(t, filepath.Join(artifact, "pkg", "index.js"), 64)

	items := []Item{{
		ID:        "project-artifact",
		Paths:     []string{artifact},
		Roots:     []string{root},
		Mechanism: Mechanism{Kind: MechanismRemove},
	}}
	guard := GuardFor(probe, items)
	roots := guard.ApprovedRoots()
	if len(roots) != 1 || roots[0] != canonicalPath(probe, root) {
		t.Fatalf("the approved roots are %v, want the project directory", roots)
	}
	if _, err := guard.Check(canonicalPath(probe, artifact)); err != nil {
		t.Errorf("the artifact under the approved project is refused: %v", err)
	}
}

// TestPlanSummarySaysWhatWouldRunAndWhatWouldNot pins the dry run as text: one
// line per item, with the command or the paths that would be removed, and the
// reason for the ones that would not run.
func TestPlanSummarySaysWhatWouldRunAndWhatWouldNot(t *testing.T) {
	probe := testProbe(t)
	dir := filepath.Join(t.TempDir(), "store")
	writeTestFile(t, filepath.Join(dir, "data.bin"), 32)
	items := []Item{
		{ID: "remove", Title: "The store", Paths: []string{dir}, Mechanism: Mechanism{Kind: MechanismRemove}},
		{ID: "tool", Title: "The tool cache", Mechanism: Mechanism{Kind: MechanismTool, Program: "npm", Args: []string{"cache", "clean"}}},
	}
	plan := BuildPlan(items, GuardFor(probe, items[:1]))
	summary := strings.Join(plan.Summary(), "\n")

	if !strings.Contains(summary, "The store: remove "+dir) {
		t.Errorf("the summary does not name the removal path:\n%s", summary)
	}
	if !strings.Contains(summary, "The tool cache: npm cache clean") {
		t.Errorf("the summary does not name the command:\n%s", summary)
	}
}
