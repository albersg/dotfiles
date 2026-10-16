package sshcfg

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestPlanCreateWritesWhenTheSSHDirectoryDoesNotExist is the first thing a user
// does on a machine that never had a configuration: the tool offers to add an
// entry, and there is no ~/.ssh yet. The diagnosis says the manager creates that
// directory when it first writes a key or an entry, so the write has to make it
// -- with 0700, because the directory is what protects everything inside it.
func TestPlanCreateWritesWhenTheSSHDirectoryDoesNotExist(t *testing.T) {
	e := testManagerSetup(t) // deliberately no e.mkdir: a fresh machine has no ~/.ssh
	plan, err := e.manager.PlanCreate(context.Background(), HostSpec{Alias: "work", HostName: "w.example", User: "a"})
	if err != nil {
		t.Fatalf("PlanCreate: %v", err)
	}
	if _, err := e.manager.Apply(context.Background(), plan); err != nil {
		t.Fatalf("Apply on a fresh machine: %v", err)
	}
	if got := e.read(t, e.manager.Paths.UserConfig); !strings.Contains(got, "Host work") {
		t.Fatalf("the first entry was not written:\n%s", got)
	}
	if info, err := os.Stat(e.manager.Paths.SSHDir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("~/.ssh was not created 0700: %v %v", info, err)
	}
}
