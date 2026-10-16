package sshcfg

import (
	"context"
	"testing"
)

// TestPlanCreateRefusesANameAForeignEntryOwns pins the rule the diagnosis
// states: the client uses the first entry it finds for a name, so a second one
// never applies. The interface refuses that name before it plans anything, but
// the planner is the layer that has to hold the rule: a caller that reached it
// directly would otherwise write an entry the user could never use.
func TestPlanCreateRefusesANameAForeignEntryOwns(t *testing.T) {
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	e.write(t, e.manager.Paths.UserConfig, 0o600, "Host work\n    User alice\n")
	if _, err := e.manager.PlanCreate(context.Background(), HostSpec{Alias: "work", HostName: "w.example", User: "bob"}); err == nil {
		t.Fatal("PlanCreate added a second entry the client would ignore")
	}
}
