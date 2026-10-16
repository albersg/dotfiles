package sshcfg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// WRITING THE CONFIGURATION
// ============================================================================
//
// The tests below are about two promises. The first is that the user's file is
// not this tool's to reformat: the assertions compare the bytes around the
// change, not just the change. The second is that a refused or stale write
// leaves the file exactly as it was -- which is asserted by reading the file
// back after every failure.

// writeTestSpec is the entry the create tests write.
func writeTestSpec() HostSpec {
	return HostSpec{
		Alias:        "work",
		HostName:     "work.example.com",
		User:         "alice",
		Port:         "2222",
		IdentityFile: "~/.ssh/id_work",
	}
}

// writeTestConfig is a configuration with everything a real one has and this
// tool must not touch: comments, blank lines, a wildcard entry, a Match block,
// a directive this package does not model, and no trailing newline.
const writeTestConfig = `# my ssh config
Host *
    AddKeysToAgent yes

Host github.com
    HostName github.com
    User git
    IdentityFile ~/.ssh/id_github

Match host *.internal
    User admin`

// requireUnchanged asserts that two texts are equal, byte for byte, and prints
// both when they are not.
func requireUnchanged(t *testing.T, what, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s changed.\n--- got ---\n%s\n--- want ---\n%s", what, got, want)
	}
}

func TestPlanCreateKeepsTheUsersFileIntact(t *testing.T) {
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	e.write(t, e.manager.Paths.UserConfig, 0o600, writeTestConfig)

	plan, err := e.manager.PlanCreate(context.Background(), writeTestSpec())
	if err != nil {
		t.Fatalf("PlanCreate: %v", err)
	}

	requireUnchanged(t, "the file the plan read", plan.Before, writeTestConfig)
	if !strings.HasPrefix(plan.After, writeTestConfig+"\n") {
		t.Errorf("the user's own bytes were not preserved at the top of the file:\n%s", plan.After)
	}
	for _, want := range []string{
		ManagedBlockStart, ManagedBlockEnd,
		"Host work\n", "    HostName work.example.com\n", "    User alice\n",
		"    Port 2222\n", "    IdentityFile ~/.ssh/id_work\n",
	} {
		if !strings.Contains(plan.After, want) {
			t.Errorf("the new file does not contain %q:\n%s", want, plan.After)
		}
	}
	if strings.Contains(plan.After, "\n\n\n") {
		t.Errorf("the block was appended with two blank lines instead of one:\n%s", plan.After)
	}
	if !strings.HasSuffix(plan.After, "\n") {
		t.Error("the written file does not end with a newline")
	}
	if plan.Kind != "managed" {
		t.Errorf("Kind = %q, want managed", plan.Kind)
	}
	if !strings.Contains(plan.Summary, "work") {
		t.Errorf("Summary = %q, want it to name the entry", plan.Summary)
	}
	if len(plan.Preview) == 0 {
		t.Error("the plan has no preview, so the confirmation would show nothing")
	}
}

func TestPlanCreateOnAMissingFileWritesTheHeader(t *testing.T) {
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)

	plan, err := e.manager.PlanCreate(context.Background(), writeTestSpec())
	if err != nil {
		t.Fatalf("PlanCreate: %v", err)
	}
	if plan.Before != "" {
		t.Errorf("Before = %q, want empty for a file that does not exist", plan.Before)
	}
	if !strings.HasPrefix(plan.After, managedBlockHeader) {
		t.Errorf("a file this tool creates does not explain its own block:\n%s", plan.After)
	}
	if !strings.Contains(plan.After, ManagedBlockStart) || !strings.Contains(plan.After, ManagedBlockEnd) {
		t.Errorf("the new file has no markers:\n%s", plan.After)
	}
	if plan.Backup != "" {
		t.Errorf("Backup = %q, want empty when there is nothing to back up", plan.Backup)
	}
}

func TestPlanCreateAddsToTheExistingBlock(t *testing.T) {
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	const config = "Host first\n    User one\n\n" +
		ManagedBlockStart + "\n" +
		"Host managed-one\n    User managed\n" +
		ManagedBlockEnd + "\n" +
		"# after the block\n"
	e.write(t, e.manager.Paths.UserConfig, 0o600, config)

	plan, err := e.manager.PlanCreate(context.Background(), writeTestSpec())
	if err != nil {
		t.Fatalf("PlanCreate: %v", err)
	}

	requireUnchanged(t, "the file the plan read", plan.Before, config)
	// The new entry belongs inside the block, so it must appear after the entry
	// the block already holds and before the closing marker.
	managed := strings.Index(plan.After, "Host managed-one")
	added := strings.Index(plan.After, "Host work")
	end := strings.Index(plan.After, ManagedBlockEnd)
	if managed < 0 || added < 0 || end < 0 || !(managed < added && added < end) {
		t.Errorf("the entry was not written inside the block:\n%s", plan.After)
	}
	if !strings.HasSuffix(plan.After, "# after the block\n") {
		t.Errorf("the content after the block was not preserved:\n%s", plan.After)
	}
	if strings.Count(plan.After, ManagedBlockStart) != 1 || strings.Count(plan.After, ManagedBlockEnd) != 1 {
		t.Errorf("the file gained a second block:\n%s", plan.After)
	}
}

// writeTestCommentedConfig is the same shape of entry with an inline comment on
// one of its lines. The parser refuses to carry such a value -- rewriting the
// line means deciding what happens to the comment -- so the entry is offered as
// read-only rather than edited.
const writeTestCommentedConfig = "Host work\n" +
	"    HostName work.example.com\n" +
	"    User alice   # the account on the jump host\n"

func TestPlanUpdateRewritesOnlyTheChangedValue(t *testing.T) {
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	const config = "Host work\n" +
		"    HostName work.example.com\n" +
		"    User alice\n" +
		"    Port 22\n" +
		"    ForwardAgent yes\n" +
		"    IdentityFile ~/.ssh/id_work\n" +
		"\n" +
		"Host other\n" +
		"    User somebody\n"
	e.write(t, e.manager.Paths.UserConfig, 0o600, config)

	plan, err := e.manager.PlanUpdate(context.Background(), "work", HostSpec{
		Alias:    "work",
		HostName: "work.example.com",
		User:     "bob",
		Port:     "2222",
	})
	if err != nil {
		t.Fatalf("PlanUpdate: %v", err)
	}

	lines := strings.Split(plan.After, "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "    HostName work.example.com"):
			requireUnchanged(t, "the HostName line", line, "    HostName work.example.com")
		case strings.HasPrefix(line, "    ForwardAgent yes"):
			requireUnchanged(t, "the unmodelled directive", line, "    ForwardAgent yes")
		}
	}
	if !strings.Contains(plan.After, "    User bob\n") {
		t.Errorf("the changed value was not written:\n%s", plan.After)
	}
	if !strings.Contains(plan.After, "    Port 2222\n") {
		t.Errorf("the changed port was not written:\n%s", plan.After)
	}
	// The cleared IdentityFile is removed rather than written empty: `IdentityFile`
	// with no value is a directive the client would refuse.
	if strings.Contains(plan.After, "IdentityFile\n") {
		t.Errorf("a cleared directive was written empty:\n%s", plan.After)
	}
	// The foreign half of the file is not just unmodified, it is identical: the plan
	// is a replacement of one range and nothing else.
	if !strings.Contains(plan.After, "\n\nHost other\n    User somebody\n") {
		t.Errorf("the entry after the edited one was reflowed:\n%s", plan.After)
	}
}

func TestPlanUpdateRefusesAnEntryWithAnInlineComment(t *testing.T) {
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	e.write(t, e.manager.Paths.UserConfig, 0o600, writeTestCommentedConfig)

	plan, err := e.manager.PlanUpdate(context.Background(), "work", HostSpec{Alias: "work", User: "bob"})
	if err == nil {
		t.Fatalf("PlanUpdate rewrote a value it cannot carry; the plan is:\n%s", plan.After)
	}
	requireUnchanged(t, "the file after a refused plan", e.read(t, e.manager.Paths.UserConfig), writeTestCommentedConfig)
}

func TestPlanUpdateRefusesAnEntryItCannotRewrite(t *testing.T) {
	cases := []struct {
		name   string
		config string
		alias  string
	}{
		{"a wildcard", "Host *.example.com\n    User a\n", "*.example.com"},
		{"a negation", "Host !bad.example.com\n    User a\n", "!bad.example.com"},
		{"more than one pattern", "Host one two\n    User a\n", "one"},
		{"a Match block after it", "Host a\n    User a\nMatch host *.internal\n    User b\n", "a"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := testManagerSetup(t)
			e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
			e.write(t, e.manager.Paths.UserConfig, 0o600, tt.config)

			plan, err := e.manager.PlanUpdate(context.Background(), tt.alias, HostSpec{Alias: tt.alias, User: "changed"})
			if err == nil {
				t.Fatalf("PlanUpdate rewrote %s; the plan is:\n%s", tt.name, plan.After)
			}
			if plan.After != "" || plan.Before != "" {
				t.Errorf("a refused plan still carries file text: %+v", plan)
			}
			requireUnchanged(t, "the file after a refused plan", e.read(t, e.manager.Paths.UserConfig), tt.config)
		})
	}
}

func TestPlanUpdateRefusesANameThatIsTaken(t *testing.T) {
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	e.write(t, e.manager.Paths.UserConfig, 0o600, "Host one\n    User a\n\nHost two\n    User b\n")

	_, err := e.manager.PlanUpdate(context.Background(), "one", HostSpec{Alias: "two", User: "c"})
	if err == nil {
		t.Fatal("PlanUpdate renamed an entry onto a name another entry uses")
	}
	if !strings.Contains(err.Error(), "two") {
		t.Errorf("the refusal does not name the entry in the way: %v", err)
	}
}

func TestPlanDeleteTakesTheStanzaWithIt(t *testing.T) {
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	const config = "Host keep\n    User a\n\nHost gone\n    User b\n\n# a comment\n"
	e.write(t, e.manager.Paths.UserConfig, 0o600, config)

	plan, err := e.manager.PlanDelete(context.Background(), "gone")
	if err != nil {
		t.Fatalf("PlanDelete: %v", err)
	}
	if strings.Contains(plan.After, "Host gone") || strings.Contains(plan.After, "User b") {
		t.Errorf("the removed entry is still there:\n%s", plan.After)
	}
	if !strings.Contains(plan.After, "Host keep\n    User a\n") {
		t.Errorf("the entry that stays was changed:\n%s", plan.After)
	}
	if !strings.Contains(plan.After, "# a comment\n") {
		t.Errorf("a comment below the removed entry was removed with it:\n%s", plan.After)
	}
	if strings.Contains(plan.After, "\n\n\n") {
		t.Errorf("the removal left a growing gap:\n%s", plan.After)
	}
}

func TestPlanDuplicateCarriesEveryDirective(t *testing.T) {
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	const config = "Host work\n    HostName work.example.com\n    User alice\n    ForwardX11 yes\n    ProxyJump bastion\n"
	e.write(t, e.manager.Paths.UserConfig, 0o600, config)

	plan, err := e.manager.PlanDuplicate(context.Background(), "work", "work-copy")
	if err != nil {
		t.Fatalf("PlanDuplicate: %v", err)
	}
	if !strings.Contains(plan.After, "Host work-copy\n") {
		t.Errorf("the copy has no name:\n%s", plan.After)
	}
	for _, want := range []string{
		"HostName work.example.com", "User alice", "ForwardX11 yes", "ProxyJump bastion",
	} {
		if strings.Count(plan.After, want) != 2 {
			t.Errorf("the copy did not carry %q: the file now holds it %d times:\n%s", want, strings.Count(plan.After, want), plan.After)
		}
	}
	if strings.Count(plan.After, "Host work\n") != 1 {
		t.Errorf("the source entry was renamed rather than copied:\n%s", plan.After)
	}
	if !strings.Contains(plan.After, ManagedBlockStart) {
		t.Errorf("the copy was not written inside the block this tool owns:\n%s", plan.After)
	}
}

func TestApplyWritesAndKeepsThePreviousFile(t *testing.T) {
	e := testManagerSetup(t)
	e.runner.handler = func(cmd Command) (Result, bool) {
		if cmd.Args[0] == "-G" {
			return okResult(""), true
		}
		return Result{}, false
	}
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	e.write(t, e.manager.Paths.UserConfig, 0o600, writeTestConfig)

	plan, err := e.manager.PlanCreate(context.Background(), writeTestSpec())
	if err != nil {
		t.Fatalf("PlanCreate: %v", err)
	}
	result, err := e.manager.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	requireUnchanged(t, "the written file", e.read(t, e.manager.Paths.UserConfig), plan.After)
	if !result.Validated {
		t.Errorf("the write reports it was not validated: %s", result.Validation)
	}
	if result.Backup == "" {
		t.Fatal("the write took no backup of the file it replaced")
	}
	requireUnchanged(t, "the backup", e.read(t, result.Backup), writeTestConfig)
	if !strings.Contains(filepath.Base(result.Backup), ".bak-dotfiles-") {
		t.Errorf("the backup is named %q, which is not the convention the rest of the installer uses", result.Backup)
	}
	if result.FileMode != 0o600 {
		t.Errorf("FileMode = %v, want 0600 for a configuration file", result.FileMode)
	}
	if result.Added == 0 {
		t.Error("the result counts no added lines")
	}
	// The temporary file the client parsed is gone: a leftover in the user's .ssh
	// directory would be a file nobody asked for.
	entries, err := os.ReadDir(e.manager.Paths.SSHDir)
	if err != nil {
		t.Fatalf("read the ssh directory: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".dotfiles-ssh-") {
			t.Errorf("the temporary file %s was left behind", entry.Name())
		}
	}
}

func TestApplyNeverOverwritesABackupThatAppearedSinceThePreview(t *testing.T) {
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	e.write(t, e.manager.Paths.UserConfig, 0o600, writeTestConfig)

	plan, err := e.manager.PlanCreate(context.Background(), writeTestSpec())
	if err != nil {
		t.Fatalf("PlanCreate: %v", err)
	}
	if plan.Backup == "" {
		t.Fatal("the plan takes no backup of the file it would replace")
	}
	// Something took the backup's name between the preview and the confirmation: an
	// earlier write of this tool, or the user's own copy. It is the last copy of
	// something, so the write must choose another name rather than replace it.
	const earlier = "an earlier backup, which is somebody's only copy\n"
	e.write(t, plan.Backup, 0o600, earlier)

	result, err := e.manager.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	requireUnchanged(t, "the backup that was already there", e.read(t, plan.Backup), earlier)
	if result.Backup == plan.Backup {
		t.Fatalf("the write reused the backup name %q", plan.Backup)
	}
	requireUnchanged(t, "the backup the write took", e.read(t, result.Backup), writeTestConfig)
	requireUnchanged(t, "the written file", e.read(t, e.manager.Paths.UserConfig), plan.After)
}

func TestApplyRefusesAFileThatMovedUnderIt(t *testing.T) {
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	e.write(t, e.manager.Paths.UserConfig, 0o600, writeTestConfig)

	plan, err := e.manager.PlanCreate(context.Background(), writeTestSpec())
	if err != nil {
		t.Fatalf("PlanCreate: %v", err)
	}

	const edited = writeTestConfig + "\nHost edited-while-you-were-reading\n    User someone\n"
	e.write(t, e.manager.Paths.UserConfig, 0o600, edited)

	_, err = e.manager.Apply(context.Background(), plan)
	if !errors.Is(err, ErrPlanStale) {
		t.Fatalf("Apply returned %v, want ErrPlanStale", err)
	}
	requireUnchanged(t, "the file after a stale write", e.read(t, e.manager.Paths.UserConfig), edited)
	matches, _ := filepath.Glob(e.manager.Paths.UserConfig + ".bak-dotfiles-*")
	if len(matches) != 0 {
		t.Errorf("a stale write still took a backup: %v", matches)
	}
}

func TestApplyRefusesWhatTheClientRejects(t *testing.T) {
	e := testManagerSetup(t)
	e.runner.handler = func(cmd Command) (Result, bool) {
		if cmd.Args[0] == "-G" {
			return failResult(1, "/tmp/config: line 1: Bad configuration option: nonsense\n"), true
		}
		return Result{}, false
	}
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	e.write(t, e.manager.Paths.UserConfig, 0o600, writeTestConfig)

	plan, err := e.manager.PlanCreate(context.Background(), writeTestSpec())
	if err != nil {
		t.Fatalf("PlanCreate: %v", err)
	}
	_, err = e.manager.Apply(context.Background(), plan)
	if err == nil {
		t.Fatal("Apply wrote a configuration the client refused")
	}
	if !strings.Contains(err.Error(), "Bad configuration option") {
		t.Errorf("the refusal does not quote the client: %v", err)
	}
	requireUnchanged(t, "the file after a rejected write", e.read(t, e.manager.Paths.UserConfig), writeTestConfig)
}

func TestApplyWritesAndSaysSoWhenTheClientCannotCheck(t *testing.T) {
	e := testManagerSetup(t)
	e.manager.Caps.SSH = ""
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)

	plan, err := e.manager.PlanCreate(context.Background(), writeTestSpec())
	if err != nil {
		t.Fatalf("PlanCreate: %v", err)
	}
	result, err := e.manager.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("Apply refused to write without a client: %v", err)
	}
	if result.Validated {
		t.Error("the write claims the client parsed it, and there is no client")
	}
	if result.Validation == "" {
		t.Error("the write does not say why it was not validated")
	}
	requireUnchanged(t, "the written file", e.read(t, e.manager.Paths.UserConfig), plan.After)
}

func TestApplyKeepsFileModesItDidNotHaveToChange(t *testing.T) {
	cases := []struct {
		name     string
		before   os.FileMode
		wantMode os.FileMode
	}{
		{"a private file stays private", 0o600, 0o600},
		{"a readable file keeps its read bits", 0o644, 0o644},
		{"a file others can write is tightened", 0o666, 0o600},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := testManagerSetup(t)
			e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
			e.write(t, e.manager.Paths.UserConfig, tt.before, "Host keep\n    User a\n")

			plan, err := e.manager.PlanCreate(context.Background(), writeTestSpec())
			if err != nil {
				t.Fatalf("PlanCreate: %v", err)
			}
			result, err := e.manager.Apply(context.Background(), plan)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if result.FileMode != tt.wantMode {
				t.Errorf("FileMode = %v, want %v", result.FileMode, tt.wantMode)
			}
			info, err := os.Stat(e.manager.Paths.UserConfig)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			if info.Mode().Perm() != tt.wantMode {
				t.Errorf("the file on disk is %v, want %v", info.Mode().Perm(), tt.wantMode)
			}
		})
	}
}

func TestBackupPathIsNeverReused(t *testing.T) {
	e := testManagerSetup(t)
	path := filepath.Join(e.home, "config")
	first := e.manager.backupPathFor(path)
	// A backup exists already, so the next write must not choose the same name:
	// that file may be the last copy of what the user wrote.
	e.write(t, first, 0o600, "an earlier backup")

	second := e.manager.backupPathFor(path)
	if second == first {
		t.Fatalf("backupPathFor returned the existing backup %q twice", first)
	}
	if !strings.HasPrefix(second, first) {
		t.Errorf("the second backup %q is not derived from %q", second, first)
	}
}

func TestPlanCreateRefusesANameItCannotOwn(t *testing.T) {
	cases := []struct {
		name  string
		alias string
	}{
		{"an empty name", "  "},
		{"a wildcard", "*.example.com"},
		{"a negation", "!bad"},
		{"a space", "two words"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := testManagerSetup(t)
			e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
			if _, err := e.manager.PlanCreate(context.Background(), HostSpec{Alias: tt.alias, User: "a"}); err == nil {
				t.Fatalf("PlanCreate accepted %s", tt.name)
			}
			if _, err := os.Stat(e.manager.Paths.UserConfig); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("a refused plan created the file: %v", err)
			}
		})
	}
}

func TestPlanCreateRefusesToRedefineAManagedEntry(t *testing.T) {
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	config := ManagedBlockStart + "\nHost work\n    User alice\n" + ManagedBlockEnd + "\n"
	e.write(t, e.manager.Paths.UserConfig, 0o600, config)

	_, err := e.manager.PlanCreate(context.Background(), writeTestSpec())
	if err == nil {
		t.Fatal("PlanCreate added a second entry with a name the managed block already holds")
	}
	if !strings.Contains(err.Error(), "already managed") {
		t.Errorf("the refusal does not say the entry is already managed: %v", err)
	}
}
