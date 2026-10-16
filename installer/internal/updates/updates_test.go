package updates

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// Identity: the one rule that must never bend
// ---------------------------------------------------------------------------
//
// Every guard in this file exists because the update center shows package names,
// and package names collide. The identity of an entry is its manager, its scope
// and its name; a test that only checks the name would pass while the interface
// merged two installations of the same program, which is the defect the mission
// names first.

// TestTwoManagersInstallsOfOneProgramStayTwoEntries pins the identity rule: two
// entries with the same name from different managers are different installations
// with different keys, and so are one manager's system and user installs.
func TestTwoManagersInstallsOfOneProgramStayTwoEntries(t *testing.T) {
	brewGit := Item{Manager: ManagerBrew, Name: "git", Scope: ScopeUser, Kind: KindFormula}
	aptGit := Item{Manager: ManagerApt, Name: "git", Scope: ScopeSystem, Kind: KindPackage}
	systemFlatpak := Item{Manager: ManagerFlatpak, Name: "org.gimp.GIMP", Scope: ScopeSystem, Kind: KindFlatpak}
	userFlatpak := Item{Manager: ManagerFlatpak, Name: "org.gimp.GIMP", Scope: ScopeUser, Kind: KindFlatpak}

	keys := map[string]string{
		"brew/user/git":  brewGit.Key(),
		"apt/system/git": aptGit.Key(),
		"flatpak/system": systemFlatpak.Key(),
		"flatpak/user":   userFlatpak.Key(),
	}
	for left, leftKey := range keys {
		for right, rightKey := range keys {
			if left == right {
				continue
			}
			if leftKey == rightKey {
				t.Fatalf("%s and %s share the key %q; two installations would be one entry", left, right, leftKey)
			}
		}
	}
	if brewGit.Origin() != "brew (user)" || aptGit.Origin() != "apt (system)" {
		t.Fatalf("the origin of an entry must name manager and scope, got %q and %q", brewGit.Origin(), aptGit.Origin())
	}
}

// TestSelectionTogglesAndKeepsIdentity pins the selection: toggling is by
// identity, a re-read that returns the same entry does not double it, and Retain
// drops only the keys a fresh answer no longer names.
func TestSelectionTogglesAndKeepsIdentity(t *testing.T) {
	git := Item{Manager: ManagerBrew, Name: "git", Scope: ScopeUser}
	curl := Item{Manager: ManagerBrew, Name: "curl", Scope: ScopeUser}
	selection := NewSelection()

	if selection.Count() != 0 {
		t.Fatalf("a new selection must be empty, got %d", selection.Count())
	}
	if !selection.Toggle(git) {
		t.Fatal("the first toggle must select")
	}
	if selection.Toggle(git) {
		t.Fatal("the second toggle must deselect")
	}
	if !selection.Toggle(git) {
		t.Fatal("a third toggle must select again")
	}
	if !selection.Toggle(curl) {
		t.Fatal("a second entry must be selectable")
	}
	if selection.Count() != 2 {
		t.Fatalf("two distinct entries must count twice, got %d", selection.Count())
	}

	// A fresh answer that no longer lists curl drops it, and keeps git.
	if dropped := selection.Retain([]Item{git}); dropped != 1 {
		t.Fatalf("Retain must drop exactly the entry the answer no longer names, dropped %d", dropped)
	}
	if selection.Has(curl) {
		t.Fatal("an entry the manager no longer reports must not stay selected")
	}
	if !selection.Has(git) {
		t.Fatal("an entry the manager still reports must stay selected")
	}

	selected := selection.Selected([]Item{curl, git})
	if len(selected) != 1 || selected[0].Name != "git" {
		t.Fatalf("Selected must return the entries the answer carries, in its order, got %+v", selected)
	}
	selection.Clear()
	if selection.Count() != 0 {
		t.Fatalf("Clear must empty the selection, got %d", selection.Count())
	}
}

// TestGroupsNeedOneCommandEach pins what one command can carry: a Homebrew
// formula and a cask need two, a Flatpak user and system installation need two,
// and the groups keep the adapters' order so the plan list is the same twice.
func TestGroupsNeedOneCommandEach(t *testing.T) {
	items := []Item{
		{Manager: ManagerBrew, Name: "cask-one", Scope: ScopeUser, Kind: KindCask},
		{Manager: ManagerBrew, Name: "formula-one", Scope: ScopeUser, Kind: KindFormula},
		{Manager: ManagerBrew, Name: "formula-two", Scope: ScopeUser, Kind: KindFormula},
		{Manager: ManagerFlatpak, Name: "app.user", Scope: ScopeUser, Kind: KindFlatpak},
		{Manager: ManagerFlatpak, Name: "app.system", Scope: ScopeSystem, Kind: KindFlatpak},
	}

	groups := GroupByKey(items)
	if len(groups) != 4 {
		t.Fatalf("four commands are needed for this selection, got %d groups", len(groups))
	}
	for _, group := range groups {
		for _, item := range group.Items {
			if item.Manager != group.Key.Manager || item.Scope != group.Key.Scope || item.Kind != group.Key.Kind {
				t.Fatalf("group %+v carries %+v, which belongs to another command", group.Key, item)
			}
		}
	}

	// The two formulae share one group, in the order they arrived, and the group
	// is found by its key rather than by its position: a test that counted
	// positions would fail on a reordering that broke no rule.
	formulaGroup, ok := groupFor(groups, GroupKey{Manager: ManagerBrew, Scope: ScopeUser, Kind: KindFormula})
	if !ok {
		t.Fatal("the two formulae must share one group")
	}
	if len(formulaGroup.Items) != 2 || formulaGroup.Items[0].Name != "formula-one" || formulaGroup.Items[1].Name != "formula-two" {
		t.Fatalf("a group keeps the entries in the order they arrived, got %+v", formulaGroup.Items)
	}

	// The formulae come before the cask, and the user's Flatpak installation
	// before the machine's: the least privileged command is planned first.
	if groups[0].Key.Kind != KindFormula || groups[1].Key.Kind != KindCask {
		t.Fatalf("Homebrew's kinds are planned in their own order, got %+v then %+v", groups[0].Key, groups[1].Key)
	}
	flatpakUser := indexOfGroup(groups, GroupKey{Manager: ManagerFlatpak, Scope: ScopeUser, Kind: KindFlatpak})
	flatpakSystem := indexOfGroup(groups, GroupKey{Manager: ManagerFlatpak, Scope: ScopeSystem, Kind: KindFlatpak})
	if flatpakUser < 0 || flatpakSystem < 0 || flatpakUser > flatpakSystem {
		t.Fatalf("the user's installation must be planned before the machine's, got %d and %d", flatpakUser, flatpakSystem)
	}
}

// TestQuoteArgvRendersForTheEyeOnly pins the one place a command is written for a
// reader: a plain argument is spelled as itself, one with a space is quoted, and a
// quote inside it is escaped.
func TestQuoteArgvRendersForTheEyeOnly(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want string
	}{
		{"plain", []string{"apt-get", "install", "-y"}, "apt-get install -y"},
		{"with a space", []string{"flatpak", "update", "--user", "org.example.App"}, "flatpak update --user org.example.App"},
		{"empty argument", []string{"cmd", ""}, "cmd ''"},
		{"a quote inside", []string{"cmd", "it's"}, `cmd 'it'\''s'`},
		{"a dollar sign inside", []string{"cmd", "$PATH"}, "cmd '$PATH'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := QuoteArgv(tt.argv); got != tt.want {
				t.Fatalf("QuoteArgv(%q) = %q, want %q", tt.argv, got, tt.want)
			}
		})
	}
}

// TestRedactLineKeepsCredentialsOutOfLogs pins the redaction of the one secret a
// package manager's own output can carry: the token a private registry puts in a
// URL's userinfo, or the value of a key whose name says it is one.
func TestRedactLineKeepsCredentialsOutOfLogs(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			"a token in a URL's userinfo",
			"Fetching https://user:s3cr3t@registry.example.com/pkg.deb",
			"Fetching https://***@registry.example.com/pkg.deb",
		},
		{
			"a token in an environment-style pair",
			"REGISTRY_TOKEN=abcdef123456",
			"REGISTRY_TOKEN=***",
		},
		{
			"a password in an environment-style pair",
			"reading password=hunter2 now",
			"reading password=*** now",
		},
		{
			"a line with no secret is left alone",
			"Installing 3 packages: git curl make",
			"Installing 3 packages: git curl make",
		},
		{
			"a pair whose key is not a secret is left alone",
			"Installed-Size=1024",
			"Installed-Size=1024",
		},
		{
			"a URL with no userinfo is left alone",
			"https://registry.example.com/pkg.deb",
			"https://registry.example.com/pkg.deb",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactLine(tt.line); got != tt.want {
				t.Fatalf("RedactLine(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

// TestTailAndFirstLineBoundWhatAScreenDraws pins the two helpers a result row
// uses: Tail keeps the end of the log, FirstLine the first real line of it.
func TestTailAndFirstLineBoundWhatAScreenDraws(t *testing.T) {
	output := "\n\nline one\nline two\n\nline three\n"
	if got := Tail(output, 2); len(got) != 2 || got[0] != "line two" || got[1] != "line three" {
		t.Fatalf("Tail(output, 2) = %q, want the last two non-empty lines", got)
	}
	if got := Tail(output, 0); got != nil {
		t.Fatalf("Tail(output, 0) = %q, want nothing", got)
	}
	if got := FirstLine(output); got != "line one" {
		t.Fatalf("FirstLine = %q, want the first non-empty line", got)
	}
	long := strings.Repeat("x", 300)
	if got := FirstLine(long); utf8.RuneCountInString(got) != 201 || !strings.HasSuffix(got, "…") {
		t.Fatalf("a long first line must be bounded and marked, got %d characters", utf8.RuneCountInString(got))
	}
	// The bound counts runes, so a line of multi-byte characters is cut between
	// characters rather than inside one.
	accented := strings.Repeat("á", 300)
	if got := FirstLine(accented); utf8.RuneCountInString(got) != 201 || !utf8.ValidString(got) {
		t.Fatalf("a multi-byte line must be cut at a character, got %d runes of %q", utf8.RuneCountInString(got), got)
	}
	if got := FirstLine("\n \n"); got != "" {
		t.Fatalf("FirstLine of a blank answer = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// Classification: what the manager said, in one of the five states
// ---------------------------------------------------------------------------

// TestClassifyNamesTheStateAManagerRefusedIn pins the states the mission asks to
// tell apart: a refused permission, a lock held elsewhere, an unreachable source,
// a dependency conflict, a plain failure -- and success, which is an answer and
// not a failure.
func TestClassifyNamesTheStateAManagerRefusedIn(t *testing.T) {
	tests := []struct {
		name   string
		output Output
		want   Status
		// wantIn is a phrase the reason must carry: either the lead sentence this
		// package writes, or the manager's own line.
		wantIn []string
	}{
		{
			name:   "a command that exited zero is an answer",
			output: Output{ExitCode: 0},
			want:   StatusCurrent,
		},
		{
			name:   "sudo refusing without a password",
			output: Output{ExitCode: 1, Err: exitError{1}, Stderr: "sudo: a password is required\n"},
			want:   StatusPermission,
			wantIn: []string{"administrator permission", "sudo: a password is required"},
		},
		{
			name:   "dpkg's own lock held by another process",
			output: Output{ExitCode: 100, Err: exitError{100}, Stderr: "E: Could not get lock /var/lib/dpkg/lock-frontend. It is held by process 1234 (unattended-upgr)\n"},
			want:   StatusLocked,
			wantIn: []string{"lock", "Could not get lock"},
		},
		{
			name:   "a source that cannot be reached",
			output: Output{ExitCode: 1, Err: exitError{1}, Stderr: "Failed to fetch http://archive.example.com/x.deb  Temporary failure in name resolution\n"},
			want:   StatusOffline,
			wantIn: []string{"could not reach its source", "Temporary failure in name resolution"},
		},
		{
			name:   "a dependency conflict",
			output: Output{ExitCode: 1, Err: exitError{1}, Stdout: "Error: conflicting dependencies: libfoo requires libbar < 2\n"},
			want:   StatusConflict,
			wantIn: []string{"dependency", "conflicting dependencies"},
		},
		{
			name:   "a lock whose message also mentions a permission, classified as the permission",
			output: Output{ExitCode: 1, Err: exitError{1}, Stderr: "error: unable to lock database: Permission denied\n"},
			want:   StatusPermission,
			wantIn: []string{"administrator permission", "Permission denied"},
		},
		{
			name:   "an answer this package cannot classify is a failure with the manager's words",
			output: Output{ExitCode: 2, Err: exitError{2}, Stderr: "error: unrecognised option '--simulate'\n"},
			want:   StatusFailed,
			wantIn: []string{"refused", "unrecognised option"},
		},
		{
			name:   "a command that never started",
			output: Output{ExitCode: -1, Err: exitError{-1}},
			want:   StatusFailed,
			wantIn: []string{"could not be run"},
		},
		{
			name:   "a cancelled run is reported as cancelled, not as a failure",
			output: Output{ExitCode: -1, Err: exitError{-1}, Cancelled: true, Stderr: "E: Could not get lock\n"},
			want:   StatusCancelled,
			wantIn: []string{"stopped at your request"},
		},
		{
			name:   "a timed-out run says so",
			output: Output{ExitCode: -1, Err: exitError{-1}, TimedOut: true},
			want:   StatusFailed,
			wantIn: []string{"timeout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason := Classify(tt.output)
			if status != tt.want {
				t.Fatalf("Classify = %s (%q), want %s", status, reason, tt.want)
			}
			for _, want := range tt.wantIn {
				if !strings.Contains(reason, want) {
					t.Fatalf("the reason %q must name %q", reason, want)
				}
			}
			if tt.want == StatusCurrent && reason != "" {
				t.Fatalf("a successful command must carry no reason, got %q", reason)
			}
		})
	}
}

// TestClassifyRedactsTheEvidenceItQuotes pins the credential rule on the path a
// reason takes to a screen and a log: the manager's own line is quoted, and a
// token in it is not.
func TestClassifyRedactsTheEvidenceItQuotes(t *testing.T) {
	_, reason := Classify(Output{
		ExitCode: 1,
		Err:      exitError{1},
		Stdout:   "Failed to fetch https://token:abcdef@registry.example.com/pkg.deb: 401 Unauthorized\n",
	})
	if strings.Contains(reason, "abcdef") {
		t.Fatalf("the reason kept a credential: %q", reason)
	}
	if !strings.Contains(reason, "***") {
		t.Fatalf("the reason must show that something was redacted, got %q", reason)
	}
}

// TestTheEnvironmentMergeReplacesRatherThanAppends pins the environment rule: an
// entry the adapter set is the only entry its child sees under that key, so a
// parseable-answer variable cannot be shadowed by the caller's.
func TestTheEnvironmentMergeReplacesRatherThanAppends(t *testing.T) {
	merged := mergeCommandEnv(
		[]string{"PATH=/usr/bin", "LC_ALL=es_ES.UTF-8", "HOME=/root"},
		[]string{"LC_ALL=C", "NO_COLOR=1"},
	)
	if got := countEnvKey(merged, "LC_ALL"); got != 1 {
		t.Fatalf("LC_ALL must appear once in the merged environment, appeared %d times", got)
	}
	if got := envValue(merged, "LC_ALL"); got != "C" {
		t.Fatalf("the adapter's LC_ALL must win, got %q", got)
	}
	if got := envValue(merged, "PATH"); got != "/usr/bin" {
		t.Fatalf("an untouched entry must survive, got %q", got)
	}
	if got := envValue(merged, "NO_COLOR"); got != "1" {
		t.Fatalf("a new entry must be added, got %q", got)
	}
	if len(mergeCommandEnv([]string{"A=1"}, nil)) != 1 {
		t.Fatal("merging nothing must return the base environment")
	}
}

// TestTheTailBufferKeepsTheEndOfAStream pins the bound on a manager's output: it
// is the end that is kept, because the end is what happened.
func TestTheTailBufferKeepsTheEndOfAStream(t *testing.T) {
	var buffer tailBuffer
	for i := 0; i < 200; i++ {
		buffer.WriteLine(fmt.Sprintf("line %04d %s", i, strings.Repeat("x", 1024)))
	}
	if len(buffer.String()) > maxKeptOutputBytes {
		t.Fatalf("the buffer kept %d bytes, more than the %d it promises", len(buffer.String()), maxKeptOutputBytes)
	}
	buffer.WriteLine("the last line")
	if !strings.HasSuffix(buffer.String(), "the last line") {
		t.Fatal("the buffer must keep the end of the stream")
	}
	if strings.Contains(buffer.String(), "line 0000") {
		t.Fatal("the buffer must drop the beginning of the stream once it is full")
	}
	if !strings.Contains(buffer.String(), "line 0199") {
		t.Fatal("the buffer must keep the lines next to the end")
	}

	var single tailBuffer
	single.WriteLine("short")
	if single.String() != "short" {
		t.Fatalf("a small stream must be kept whole, got %q", single.String())
	}
}

// groupFor returns the group with one key, or false when it is not there.
func groupFor(groups []struct {
	Key   GroupKey
	Items []Item
}, key GroupKey) (struct {
	Key   GroupKey
	Items []Item
}, bool) {
	for _, group := range groups {
		if group.Key == key {
			return group, true
		}
	}
	return struct {
		Key   GroupKey
		Items []Item
	}{}, false
}

// indexOfGroup returns the position of one group, or -1 when it is not there.
func indexOfGroup(groups []struct {
	Key   GroupKey
	Items []Item
}, key GroupKey) int {
	for index, group := range groups {
		if group.Key == key {
			return index
		}
	}
	return -1
}

// TestStatusSettledTellsAnAnswerFromARetry pins which states a retry can improve:
// an answer is settled, and a refusal is not, because the network may be back and
// the lock may be free.
func TestStatusSettledTellsAnAnswerFromARetry(t *testing.T) {
	settled := []Status{StatusUpdates, StatusCurrent, StatusUnavailable, StatusUnsupported}
	for _, status := range settled {
		if !status.Settled() {
			t.Fatalf("%s is an answer and must be settled", status)
		}
	}
	for _, status := range []Status{StatusFailed, StatusOffline, StatusPermission, StatusLocked, StatusConflict, StatusCancelled} {
		if status.Settled() {
			t.Fatalf("%s may change on a retry and must not be settled", status)
		}
	}
}

// exitError is the smallest error a command's exit status can carry, so a
// classification case can be written without a process.
type exitError struct{ code int }

func (e exitError) Error() string { return "exit status " + itoa(e.code) }

// countEnvKey counts how many entries an environment carries under one key.
func countEnvKey(env []string, key string) int {
	count := 0
	for _, entry := range env {
		if name, _, ok := strings.Cut(entry, "="); ok && name == key {
			count++
		}
	}
	return count
}

// envValue reads one key out of an environment, or "" when it is absent.
func envValue(env []string, key string) string {
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok && name == key {
			return value
		}
	}
	return ""
}
