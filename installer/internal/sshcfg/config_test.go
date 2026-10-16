package sshcfg

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// READING THE CONFIGURATION AND RESOLVING ONE ALIAS
// ============================================================================
//
// Two rules are asserted here over and over. The first is that reading never
// normalises: the entries are reported in file order, with the file and the line
// they came from, because that order *is* the client's precedence. The second is
// that this package does not pretend to be the client: the effective
// configuration is the client's answer when it can be asked, and when it cannot,
// the fallback names what it did not evaluate instead of guessing.

// configTestEnv is a manager whose configuration directory is a temporary one.
func configTestEnv(t *testing.T) *testManager {
	t.Helper()
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	// The system file is pointed inside the test's own tree: a test must never
	// read the machine's /etc/ssh/ssh_config, which would make it depend on the
	// host it runs on.
	e.manager.Paths.SystemConfig = filepath.Join(e.home, "etc", "ssh_config")
	return e
}

func TestLoadConfigReportsTheEntriesInFileOrder(t *testing.T) {
	e := configTestEnv(t)
	e.write(t, e.manager.Paths.UserConfig, 0o600, ""+
		"# a comment\n"+
		"User alice\n"+
		"Host github.com\n"+
		"    HostName github.com\n"+
		"    IdentityFile ~/.ssh/id_github\n"+
		"\n"+
		"Host *.internal\n"+
		"    User admin\n")

	config := e.manager.LoadConfig(context.Background())

	if len(config.Hosts) != 2 {
		t.Fatalf("found %d entries, want 2: %+v", len(config.Hosts), config.Hosts)
	}
	first := config.Hosts[0]
	if first.Alias != "github.com" || !equalStrings(first.Patterns, []string{"github.com"}) {
		t.Errorf("first entry = %+v", first)
	}
	if first.File != e.manager.Paths.UserConfig || first.Line != 3 {
		t.Errorf("first entry came from %s:%d, want the config file line 3", first.File, first.Line)
	}
	if len(first.Options) != 2 || first.Options[0].Key != "HostName" {
		t.Errorf("first entry's directives = %+v", first.Options)
	}
	if first.ReadOnly != "" {
		t.Errorf("a plain entry is read-only: %s", first.ReadOnly)
	}
	if config.Hosts[1].ReadOnly == "" {
		t.Error("a wildcard entry is offered as editable")
	}
	if len(config.Files) == 0 || config.Files[0].Path != e.manager.Paths.UserConfig {
		t.Errorf("the files read are %+v, want the user's file first", config.Files)
	}
	// The system file does not exist in this test, and that is reported rather
	// than left as a silent absence.
	var sawMissing bool
	for _, file := range config.Files {
		if file.Path == e.manager.Paths.SystemConfig && file.Missing {
			sawMissing = true
		}
	}
	if !sawMissing {
		t.Errorf("the missing system file is not reported: %+v", config.Files)
	}
}

func TestLoadConfigMarksTheBlockItOwns(t *testing.T) {
	e := configTestEnv(t)
	e.write(t, e.manager.Paths.UserConfig, 0o600, ""+
		"Host mine\n"+
		"    User me\n"+
		"\n"+
		ManagedBlockStart+"\n"+
		"Host theirs\n"+
		"    User them\n"+
		ManagedBlockEnd+"\n"+
		"Host after\n"+
		"    User later\n")

	config := e.manager.LoadConfig(context.Background())
	want := map[string]bool{"mine": false, "theirs": true, "after": false}
	for _, host := range config.Hosts {
		managed, ok := want[host.Alias]
		if !ok {
			t.Fatalf("unexpected entry %q", host.Alias)
		}
		if host.Managed != managed {
			t.Errorf("%s: Managed = %v, want %v", host.Alias, host.Managed, managed)
		}
	}
}

func TestLoadConfigFollowsAnIncludeInline(t *testing.T) {
	e := configTestEnv(t)
	dir := filepath.Join(e.manager.Paths.SSHDir, "config.d")
	e.mkdir(t, dir, 0o700)
	e.write(t, filepath.Join(dir, "work.conf"), 0o600, "Host work\n    User alice\n")
	e.write(t, e.manager.Paths.UserConfig, 0o600, ""+
		"Host before\n"+
		"    User a\n"+
		"Include config.d/*.conf\n"+
		"Host after\n"+
		"    User b\n")

	config := e.manager.LoadConfig(context.Background())
	var aliases []string
	for _, host := range config.Hosts {
		aliases = append(aliases, host.Alias)
	}
	// The order is the client's precedence: an included file is read where its
	// Include appeared, not at the end.
	if !equalStrings(aliases, []string{"before", "work", "after"}) {
		t.Errorf("entries = %v, want the included file inlined where its Include was", aliases)
	}
	included := config.Hosts[1]
	if included.File != filepath.Join(dir, "work.conf") {
		t.Errorf("the included entry came from %s, want the included file", included.File)
	}
	for _, file := range config.Files {
		if file.Path == filepath.Join(dir, "work.conf") && file.Depth != 1 {
			t.Errorf("the included file is at depth %d, want 1", file.Depth)
		}
	}
}

func TestLoadConfigReportsAnIncludeThatMatchedNothing(t *testing.T) {
	e := configTestEnv(t)
	e.write(t, e.manager.Paths.UserConfig, 0o600, "Include config.d/*.conf\n")

	config := e.manager.LoadConfig(context.Background())
	if len(config.Failures) == 0 {
		t.Fatal("an Include that matched no file was not reported")
	}
	if !strings.Contains(strings.Join(config.Failures, "\n"), "matched no file") {
		t.Errorf("failures = %v, want the Include named", config.Failures)
	}
}

func TestLoadConfigCountsMatchBlocks(t *testing.T) {
	e := configTestEnv(t)
	e.write(t, e.manager.Paths.UserConfig, 0o600, ""+
		"Host a\n"+
		"    User a\n"+
		"Match host *.internal\n"+
		"    User admin\n"+
		"Match exec \"test -f /tmp/flag\"\n"+
		"    User flagged\n")

	config := e.manager.LoadConfig(context.Background())
	if config.MatchBlocks != 2 {
		t.Errorf("MatchBlocks = %d, want 2", config.MatchBlocks)
	}
	// A Match block's directives belong to no single entry, so the entry before it
	// is refused rather than credited with somebody else's options.
	if len(config.Hosts) != 1 {
		t.Fatalf("entries = %+v, want the one Host block", config.Hosts)
	}
	if config.Hosts[0].ReadOnly == "" {
		t.Error("an entry followed by a Match line is offered as editable")
	}
}

func TestConfigRefusesShapesItCannotRewriteSafely(t *testing.T) {
	cases := []struct {
		name   string
		config string
		reason string
	}{
		{
			name:   "a wildcard pattern",
			config: "Host *.example.com\n    User a\n",
			reason: "wildcard",
		},
		{
			name:   "a negated pattern",
			config: "Host !bad.example.com\n    User a\n",
			reason: "negation",
		},
		{
			name:   "two patterns on one line",
			config: "Host one two\n    User a\n",
			reason: "more than one pattern",
		},
		{
			name:   "an Include inside the entry",
			config: "Host a\n    Include extra.conf\n    User a\n",
			reason: "other files",
		},
		{
			name:   "a Match line after the entry",
			config: "Host a\n    User a\nMatch host *.internal\n    User b\n",
			reason: "Match line follows",
		},
		{
			name:   "a value with a trailing comment",
			config: "Host a\n    User alice # the account\n",
			reason: "not a plain",
		},
		{
			name:   "a directive with no value",
			config: "Host a\n    ForwardAgent\n",
			reason: "not a plain",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := configTestEnv(t)
			e.write(t, e.manager.Paths.UserConfig, 0o600, tt.config)

			config := e.manager.LoadConfig(context.Background())
			if len(config.Hosts) != 1 {
				t.Fatalf("entries = %+v, want one", config.Hosts)
			}
			reason := config.Hosts[0].ReadOnly
			if reason == "" {
				t.Fatalf("%s was offered as editable", tt.name)
			}
			if !strings.Contains(reason, tt.reason) {
				t.Errorf("the reason is %q, want one about %q", reason, tt.reason)
			}
			// The reason is a sentence the screen shows, so it has to read as one.
			if !strings.HasSuffix(reason, ".") && !strings.Contains(reason, ", so") {
				t.Errorf("the reason does not read as a sentence: %q", reason)
			}
		})
	}
}

func TestConfigRefusesAnEntryInAFileItCannotWrite(t *testing.T) {
	e := configTestEnv(t)
	path := e.manager.Paths.UserConfig
	e.write(t, path, 0o600, "Host a\n    User a\n")
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	config := e.manager.LoadConfig(context.Background())
	if len(config.Hosts) != 1 {
		t.Fatalf("entries = %+v, want one", config.Hosts)
	}
	if config.Hosts[0].ReadOnly == "" {
		t.Error("an entry in a file this tool cannot write is offered as editable")
	}
	if !strings.Contains(config.Hosts[0].ReadOnly, "not writable") {
		t.Errorf("the reason is %q, want one about the file's permissions", config.Hosts[0].ReadOnly)
	}
}

func TestResolveAsksTheClientFirst(t *testing.T) {
	e := configTestEnv(t)
	e.write(t, e.manager.Paths.UserConfig, 0o600, ""+
		"Host web\n"+
		"    HostName declared.example.com\n"+
		"    User deploy\n"+
		"    IdentityFile ~/.ssh/id_work\n")
	e.runner.handler = func(cmd Command) (Result, bool) {
		if equalStrings(cmd.Args, []string{"-G", "--", "web"}) {
			return okResult("" +
				"host web\n" +
				"hostname web.example.com\n" +
				"user deploy\n" +
				"port 2222\n" +
				"identityfile ~/.ssh/id_deploy\n" +
				"identityfile ~/.ssh/id_work\n" +
				"identitiesonly yes\n" +
				"proxyjump bastion\n"), true
		}
		return Result{}, false
	}

	effective := e.manager.Resolve(context.Background(), "web")

	if !effective.FromClient {
		t.Fatal("the client answered and the result does not say so")
	}
	if effective.HostName != "web.example.com" || effective.User != "deploy" || effective.Port != "2222" {
		t.Errorf("the client's answer was not used: %+v", effective)
	}
	// The client prints IdentityFile once per file, and every one of them is kept
	// in order: a host with several identities is the normal case.
	if !equalStrings(effective.IdentityFiles, []string{"~/.ssh/id_deploy", "~/.ssh/id_work"}) {
		t.Errorf("IdentityFiles = %v, want both in the client's order", effective.IdentityFiles)
	}
	if effective.Options["identitiesonly"] != "yes" {
		t.Errorf("a value only the client knows was dropped: %+v", effective.Options)
	}
	if effective.Raw == "" {
		t.Error("the client's own output was not kept")
	}
	// Sources is only filled where this package's own reading agrees with the
	// client: the client does not report provenance, so anything else would be
	// invented. The `ok` form matters here -- an absent entry and an entry whose
	// fields are empty are not the same claim.
	if source, ok := effective.Sources["user"]; !ok || source.File != e.manager.Paths.UserConfig {
		t.Errorf("the user value's source is %+v (present %v), want the file this package read", source, ok)
	}
	if source, ok := effective.Sources["port"]; ok {
		t.Error("a source was invented for a value this package's own reading never saw: " + source.File)
	}
	if source, ok := effective.Sources["hostname"]; ok {
		// The client answered web.example.com while the file declares
		// declared.example.com, so the two disagree and no source may be claimed.
		t.Errorf("a source was claimed for a value the two readings disagree about: %+v", source)
	}
}

func TestResolveFallsBackToItsOwnReadingAndSaysSo(t *testing.T) {
	e := configTestEnv(t)
	e.write(t, e.manager.Paths.UserConfig, 0o600, ""+
		"User global-user\n"+
		"Host web\n"+
		"    HostName web.example.com\n"+
		"    IdentityFile ~/.ssh/id_work\n"+
		"Host web\n"+
		"    Port 2222\n")
	// The client cannot be asked: the runner refuses.
	e.runner.handler = func(cmd Command) (Result, bool) {
		return failResult(1, "ssh: not found\n"), true
	}

	effective := e.manager.Resolve(context.Background(), "web")

	if effective.FromClient {
		t.Fatal("the result claims the client answered when it did not")
	}
	if effective.HostName != "web.example.com" {
		t.Errorf("HostName = %q, want the value from the file", effective.HostName)
	}
	if effective.User != "global-user" {
		t.Errorf("User = %q, want the global directive that applies to every entry", effective.User)
	}
	// Both stanzas match the alias, and the client applies both: for a
	// single-valued key the first *assignment* wins, not the first block. The
	// second block is where port is assigned, so it is the value the client uses
	// -- checked against the client itself, which answers `port 2222` for this
	// file.
	if effective.Port != "2222" {
		t.Errorf("Port = %q, want the second block's value: a key is decided by its first assignment", effective.Port)
	}
	if len(effective.Warnings) == 0 {
		t.Fatal("the fallback did not say it is a fallback")
	}
	if !strings.Contains(strings.Join(effective.Warnings, " "), "own reading") {
		t.Errorf("warnings = %v, want one that names the reading it used", effective.Warnings)
	}
	if source := effective.Sources["hostname"]; source.File != e.manager.Paths.UserConfig || source.Line != 3 {
		t.Errorf("the source is %+v, want the declaring file and line", source)
	}
}

func TestResolveNamesAnUnknownAliasAndAnUnusableMatchBlock(t *testing.T) {
	t.Run("an alias no file mentions", func(t *testing.T) {
		e := configTestEnv(t)
		e.write(t, e.manager.Paths.UserConfig, 0o600, "Host other\n    User a\n")
		e.runner.handler = func(cmd Command) (Result, bool) { return failResult(1, ""), true }

		effective := e.manager.Resolve(context.Background(), "unknown")
		want := "unknown"
		if effective.HostName != want {
			t.Errorf("HostName = %q, want %q: the client connects to the name itself", effective.HostName, want)
		}
		if !strings.Contains(strings.Join(effective.Warnings, " "), "connects to the name itself") {
			t.Errorf("warnings = %v, want the sentence about the alias", effective.Warnings)
		}
	})

	t.Run("a Match criterion this tool does not evaluate", func(t *testing.T) {
		e := configTestEnv(t)
		e.write(t, e.manager.Paths.UserConfig, 0o600, ""+
			"Host web\n"+
			"    User a\n"+
			"Match exec \"test -f /tmp/flag\"\n"+
			"    User flagged\n")
		e.runner.handler = func(cmd Command) (Result, bool) { return failResult(1, ""), true }

		effective := e.manager.Resolve(context.Background(), "web")
		joined := strings.Join(effective.Warnings, " ")
		if !strings.Contains(joined, "does not evaluate") || !strings.Contains(joined, "exec") {
			t.Errorf("warnings = %v, want the unevaluated criterion named", effective.Warnings)
		}
	})
}

func TestResolveIsEmptyForAnEmptyAlias(t *testing.T) {
	e := configTestEnv(t)
	e.write(t, e.manager.Paths.UserConfig, 0o600, "Host web\n    User a\n")
	e.runner.handler = func(cmd Command) (Result, bool) { return failResult(1, ""), true }

	effective := e.manager.Resolve(context.Background(), "")
	if effective.Alias != "" {
		t.Errorf("Alias = %q, want the empty alias it was asked about", effective.Alias)
	}
	if effective.HostName != "" {
		t.Errorf("HostName = %q, want nothing for nothing", effective.HostName)
	}
}

func TestApplyClientOutputKeepsEveryIdentityFileInOrder(t *testing.T) {
	effective := Effective{Options: map[string]string{}, Sources: map[string]Source{}}
	applyClientOutput(&effective, ""+
		"host web\n"+
		"identityfile ~/.ssh/first\n"+
		"identityfile ~/.ssh/second\n"+
		"identityfile ~/.ssh/third\n"+
		"user alice\n")

	if !equalStrings(effective.IdentityFiles, []string{"~/.ssh/first", "~/.ssh/second", "~/.ssh/third"}) {
		t.Errorf("IdentityFiles = %v, want all three in order", effective.IdentityFiles)
	}
	// The first value wins for the single-valued key, like the client does.
	if effective.Options["identityfile"] != "~/.ssh/first" {
		t.Errorf("the first identity was not the one carried: %+v", effective.Options)
	}
	if effective.User != "alice" {
		t.Errorf("User = %q", effective.User)
	}
}

func TestLoadConfigNeverReadsTheWindowsSideConfiguration(t *testing.T) {
	e := configTestEnv(t)
	e.manager.Paths.WSL = true
	e.write(t, e.manager.Paths.UserConfig, 0o600, "Host web\n    User a\n")
	// A path that looks like a Windows-side configuration, inside the test's own
	// tree so nothing outside it is touched.
	windows := filepath.Join(e.home, "mnt", "c", "Users", "alice", ".ssh", "config")
	e.write(t, windows, 0o600, "Host windows-only\n    User windows\n")

	config := e.manager.LoadConfig(context.Background())
	if len(config.Hosts) != 1 || config.Hosts[0].Alias != "web" {
		t.Errorf("entries = %+v, want only the Linux-side configuration", config.Hosts)
	}
	for _, host := range config.Hosts {
		if strings.Contains(host.File, "mnt") {
			t.Errorf("a Windows-side file was read: %s", host.File)
		}
	}
}
