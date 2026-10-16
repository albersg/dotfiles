package tui

// The policy is described in three places a reader sees -- this screen, the
// template's comments and the by-hand recovery script -- and the suite held the
// arithmetic and never the arithmetic against the words that describe it. That is
// how a sentence came to promise Windows 4 GiB on the hosts where the floor gives
// it less, and how the recovery script came to print the sizes of a fraction that
// had been superseded. These two tests compare the prose with the plans the policy
// produces, and both were red against the tree that shipped them.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// planForMiB is the plan for a host described the way the prose describes one.
func planForMiB(mib uint64, cpus int) WSLResources {
	return PlanWSLResources(system.HostResources{MemoryBytes: mib << 20, LogicalCPUs: cpus})
}

// TestTheScreenDoesNotPromiseWindowsMemoryThePlanDoesNotKeep pins the resolution of
// the two memory rules the screen states. "Never so much that Windows keeps less
// than 4 GiB" and "never below half the host" disagree below 8 GiB, where the floor
// is the larger of the two, and the sentence stated both without saying which wins:
// on a 6 GiB host the plan leaves Windows 3 GiB while the screen promised it 4. The
// template's own comment resolved that and the screen did not, and no test compared
// a sentence with a plan, so the suite stayed green while the sentence was false.
func TestTheScreenDoesNotPromiseWindowsMemoryThePlanDoesNotKeep(t *testing.T) {
	var short []string
	for _, mib := range []uint64{4096, 6144, 7168, 8192, 16384, 32768} {
		plan := planForMiB(mib, 12)
		if keeps := mib - uint64(plan.MemoryMB); keeps < 4096 {
			short = append(short, fmt.Sprintf("host %d MiB -> memory=%dMB, Windows keeps %d MiB",
				mib, plan.MemoryMB, keeps))
		}
	}
	if len(short) == 0 {
		t.Fatal("no host of this set leaves Windows under 4 GiB any more, so this test would " +
			"pass without measuring anything: widen the set or delete the test")
	}

	var sentence string
	for _, paragraph := range wslGuardScreenModel(t, wslGuardState{}).wslResourceDescription() {
		if strings.Contains(paragraph, "Memory is ") {
			sentence = paragraph
		}
	}
	if sentence == "" {
		t.Fatal("the screen no longer states the memory rule at all, so this test measures nothing")
	}

	if !strings.Contains(strings.ToLower(sentence), "floor") {
		t.Errorf("the plan leaves Windows under 4 GiB on ordinary hosts:\n  %s\n"+
			"and the screen states both rules without the one that decides between them:\n  %s",
			strings.Join(short, "\n  "), sentence)
	}
}

// TestTheByHandScriptQuotesThePolicyItApplies pins the recovery script's console
// text against the fraction the policy applies. That script is what a machine which
// never ran the installer follows, and it printed the sizes of the superseded
// fraction -- a 10240 MB VM, ~9.6Gi of RAM, ~8380M of cap -- so its reader was told
// to expect the memory the previous rule planned. What it says about sizes cannot
// be pinned to a plan, because the VM's total is whatever the user's own
// .wslconfig sets; what it must not do is quote one as though the policy owned it.
func TestTheByHandScriptQuotesThePolicyItApplies(t *testing.T) {
	path := filepath.Join(repoRoot(t), "dotfiles-wsl", "apply-wsl-memory-guard.sh")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)

	for _, stale := range []string{"memory=10240MB", "~9.6Gi", "~8380M"} {
		if strings.Contains(script, stale) {
			t.Errorf("%s still tells its reader to expect %q, which is what the superseded fraction planned",
				filepath.Base(path), stale)
		}
	}

	// An expectation carrying a number is an expectation that goes stale the next
	// time the fraction moves, so the rule is that these carry none.
	hardcoded := regexp.MustCompile(`expect ~?[0-9]+`)
	if found := hardcoded.FindAllString(script, -1); len(found) > 0 {
		t.Errorf("%s states a hardcoded size in an expectation (%v); the memory is whatever .wslconfig sets, "+
			"so the expectation has to say that instead", filepath.Base(path), found)
	}
}
