package tui

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/sshcfg"
	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// THE SSH MANAGER'S OWN TESTS
// ============================================================================
//
// These tests are about the screens: which rows a state offers, what a key does
// to it, and what the body says about a fact. They build the model by hand and
// never run the client, the key generator or the agent: every fact here arrives
// the way it arrives at runtime, as state a command filled, so no test in this
// file can touch the developer's own configuration, keys or agent.

// sshTestManagerState is a filled manager state: two hosts, one managed and one
// read-only, two keys, an agent holding one identity, three findings and a
// finished test. It is deliberately bigger than one screen so the windowing and
// the row arithmetic are exercised.
func sshTestManagerState() sshManagerState {
	managed := sshcfg.Host{
		Alias:   "web",
		File:    "/home/testuser/.ssh/config",
		Line:    12,
		Managed: true,
		Options: []sshcfg.Option{
			{Key: "HostName", Value: "web.example.com", File: "/home/testuser/.ssh/config", Line: 13},
			{Key: "User", Value: "deploy", File: "/home/testuser/.ssh/config", Line: 14},
			{Key: "Port", Value: "2222", File: "/home/testuser/.ssh/config", Line: 15},
			{Key: "IdentityFile", Value: "~/.ssh/id_ed25519", File: "/home/testuser/.ssh/config", Line: 16},
		},
	}
	readOnly := sshcfg.Host{
		Alias:    "*.internal",
		Patterns: []string{"*.internal"},
		File:     "/home/testuser/.ssh/config.d/extra",
		Line:     3,
		ReadOnly: "The entry's pattern is a wildcard, so it covers more than one host and this tool will not rewrite it.",
		Options:  []sshcfg.Option{{Key: "User", Value: "ops", File: "/home/testuser/.ssh/config.d/extra", Line: 4}},
	}

	return sshManagerState{
		Manager: &sshcfg.Manager{
			Paths: sshcfg.Paths{
				Home:       "/home/testuser",
				SSHDir:     "/home/testuser/.ssh",
				UserConfig: "/home/testuser/.ssh/config",
				KnownHosts: "/home/testuser/.ssh/known_hosts",
			},
			Caps: sshcfg.Caps{
				SSH:            "/usr/bin/ssh",
				Keygen:         "/usr/bin/ssh-keygen",
				Add:            "/usr/bin/ssh-add",
				SSHVersion:     "OpenSSH_10.0p2, OpenSSL 3.5.6",
				KeygenSupports: []string{"ssh-ed25519", "ecdsa-sha2-nistp256", "sk-ssh-ed25519@openssh.com", "sk-ecdsa-sha2-nistp256@openssh.com", "ssh-rsa"},
			},
		},
		Hosts:       []sshcfg.Host{managed, readOnly},
		HostsLoaded: true,
		ConfigFiles: []sshcfg.ConfigFile{
			{Path: "/home/testuser/.ssh/config"},
			{Path: "/etc/ssh/ssh_config", Missing: true},
		},
		Detail: sshcfg.Effective{
			Alias:         "web",
			HostName:      "web.example.com",
			User:          "deploy",
			Port:          "2222",
			IdentityFiles: []string{"~/.ssh/id_ed25519"},
			FromClient:    true,
			Sources:       map[string]sshcfg.Source{"hostname": {Key: "hostname", Value: "web.example.com", File: "/home/testuser/.ssh/config", Line: 13}},
		},
		DetailAlias: "web",
		Keys: []sshcfg.Key{
			{
				Path:          "/home/testuser/.ssh/id_ed25519",
				PubPath:       "/home/testuser/.ssh/id_ed25519.pub",
				Type:          "ssh-ed25519",
				Bits:          256,
				Fingerprint:   "SHA256:aaaa",
				Comment:       "testuser@host",
				Public:        "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIaaaa testuser@host",
				PublicKnown:   true,
				Mode:          0o600,
				PermissionsOK: true,
			},
			{
				Path:          "/home/testuser/.ssh/id_rsa",
				PubPath:       "/home/testuser/.ssh/id_rsa.pub",
				Type:          "ssh-rsa",
				Bits:          4096,
				Fingerprint:   "SHA256:bbbb",
				Public:        "ssh-rsa AAAAB3NzaC1yc2Ebbbb",
				PublicKnown:   true,
				Mode:          0o644,
				PermissionsOK: false,
			},
		},
		KnownHosts: []sshcfg.KnownHost{
			{Hosts: []string{"github.com"}, Type: "ssh-ed25519", Fingerprint: "SHA256:cccc", File: "/home/testuser/.ssh/known_hosts", Line: 1},
			{Hashed: true, Type: "ssh-ed25519", Fingerprint: "SHA256:dddd", File: "/home/testuser/.ssh/known_hosts", Line: 2},
		},
		KeysLoaded:  true,
		AgentLoaded: true,
		Agent: sshcfg.AgentState{
			Advertised:   true,
			Socket:       "/tmp/ssh-XXXX/agent.1234",
			Reachable:    true,
			Kind:         "OpenSSH ssh-agent",
			Identities:   []sshcfg.AgentIdentity{{Bits: 256, Fingerprint: "SHA256:aaaa", Comment: "testuser@host", Type: "ED25519"}},
			CanAdd:       true,
			CanRemove:    true,
			CanRemoveAll: true,
		},
		Findings: []sshcfg.Finding{
			{Severity: sshcfg.SeverityProblem, Code: "private-key-permissions", Title: "id_rsa is readable by other users", Detail: "The private key id_rsa is mode 0644.", Fix: "chmod 600 ~/.ssh/id_rsa", File: "/home/testuser/.ssh/id_rsa"},
			{Severity: sshcfg.SeverityWarning, Code: "alias-duplicate", Title: "The alias web is declared twice", Detail: "The client stops at the first one.", File: "/home/testuser/.ssh/config", Line: 12},
			{Severity: sshcfg.SeverityInfo, Code: "wsl-separate-config", Title: "This is WSL", Detail: "The Windows-side configuration is not read here."},
		},
		DiagnosticsLoaded: true,
		TestAlias:         "web",
		Test: sshcfg.ConnectResult{
			Alias:       "web",
			Outcome:     sshcfg.OutcomeOK,
			Explanation: "The connection opened and the remote command `true` ran.",
			Args:        []string{"/usr/bin/ssh", "-o", "BatchMode=yes", "--", "web", "true"},
		},
		Tested: true,
	}
}

// sshTestModel is a model parked on one of the manager's screens with the filled
// state, at the documented frame floor.
func sshTestModel(t *testing.T, screen Screen) Model {
	t.Helper()
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = trainerFrameWidth
	m.Height = trainerFrameHeight
	m.Screen = screen
	m.SSH = sshTestManagerState()
	return m
}

// TestSSHManagerRowsAreExactlyWhatTheHandlersMatch is the guard that the rows a
// screen draws and the rows its handler acts on are the same strings: every row
// is either a host, an action this test knows, or the way back.
func TestSSHManagerRowsAreExactlyWhatTheHandlersMatch(t *testing.T) {
	m := sshTestModel(t, ScreenSSHManager)

	rows := m.GetCurrentOptions()
	// Two hosts, the separator above the actions, the five actions, the separator
	// above the way back and the way back itself.
	if len(rows) != len(m.SSH.Hosts)+8 {
		t.Fatalf("the hub draws %d rows for %d hosts, want %d", len(rows), len(m.SSH.Hosts), len(m.SSH.Hosts)+8)
	}
	for i, host := range m.SSH.Hosts {
		if !strings.HasPrefix(rows[i], utilitiesScreenMark+host.Alias) {
			t.Errorf("row %d is %q, want it to open on the alias %q with the arrow that says so", i, rows[i], host.Alias)
		}
	}
	for _, want := range []string{sshAddHostRow, sshKeysRow, sshAgentRow, sshDiagnoseRow, sshTestRow} {
		if !containsRow(rows, utilitiesScreenRow(want)) {
			t.Errorf("the hub does not offer %q: %v", utilitiesScreenRow(want), rows)
		}
	}
	if !containsRow(rows, sshBackRow) {
		t.Errorf("the hub does not offer the way back %q: %v", sshBackRow, rows)
	}
}

// containsRow reports whether a row list holds a label exactly.
func containsRow(rows []string, want string) bool {
	for _, row := range rows {
		if row == want {
			return true
		}
	}
	return false
}

// TestSSHHostRowsFollowTheEntrysOwnRules covers the rule the spec makes explicit:
// an entry that cannot be edited safely has no edit row and no delete row, and
// says why in its body instead.
func TestSSHHostRowsFollowTheEntrysOwnRules(t *testing.T) {
	editable := sshTestModel(t, ScreenSSHHost)
	editable.SSH.HostIndex = 0
	rows := editable.GetCurrentOptions()
	for _, want := range []string{utilitiesScreenRow(sshEditRow), utilitiesScreenRow(sshDuplicateRow), sshDeleteRow, utilitiesScreenRow(sshTestThisRow), sshCopyPubRow, sshBackToHostsRow} {
		if !containsRow(rows, want) {
			t.Errorf("an editable entry does not offer %q: %v", want, rows)
		}
	}

	readOnly := sshTestModel(t, ScreenSSHHost)
	readOnly.SSH.HostIndex = 1
	rows = readOnly.GetCurrentOptions()
	for _, unwanted := range []string{utilitiesScreenRow(sshEditRow), sshDeleteRow} {
		if containsRow(rows, unwanted) {
			t.Errorf("a read-only entry offers %q: %v", unwanted, rows)
		}
	}
	if !containsRow(rows, utilitiesScreenRow(sshDuplicateRow)) {
		t.Errorf("a read-only entry cannot be duplicated, which is the one safe edit: %v", rows)
	}
	body := strings.Join(readOnly.sshHostDescription(), "\n")
	if !strings.Contains(body, "Shown read-only:") || !strings.Contains(body, "wildcard") {
		t.Errorf("the read-only reason is not on the screen:\n%s", body)
	}
}

// TestSSHHostDetailNamesTheClientsAnswer covers the spec's first requirement:
// what the screen shows is the effective configuration, and where a value came
// from is on the screen beside it.
func TestSSHHostDetailNamesTheClientsAnswer(t *testing.T) {
	m := sshTestModel(t, ScreenSSHHost)
	m.SSH.HostIndex = 0

	body := strings.Join(m.sshHostDescription(), "\n")
	for _, want := range []string{
		"web.example.com", "deploy", "2222", "~/.ssh/id_ed25519",
		"the client's answer", // the values came from ssh -G, not from this package
		"Declared in ~/.ssh/config:12",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the detail body does not name %q:\n%s", want, body)
		}
	}
}

// TestSSHFormRefusesWhatTheWriterWouldRefuse covers the form's own validation:
// the reasons are stated before the plan is built, so a press produces a reason
// rather than a failed write.
func TestSSHFormRefusesWhatTheWriterWouldRefuse(t *testing.T) {
	tests := []struct {
		name   string
		form   sshHostForm
		refuse string // a fragment of the reason the form must state
	}{
		{name: "empty alias", form: sshHostForm{HostName: "example.com"}, refuse: "alias is required"},
		{name: "wildcard alias", form: sshHostForm{Alias: "*.corp"}, refuse: "single literal name"},
		{name: "space in the alias", form: sshHostForm{Alias: "two words"}, refuse: "cannot contain a space"},
		{name: "port out of range", form: sshHostForm{Alias: "ok", Port: "70000"}, refuse: "between 1 and 65535"},
		{name: "port not a number", form: sshHostForm{Alias: "ok", Port: "ssh"}, refuse: "between 1 and 65535"},
		{name: "alias already taken", form: sshHostForm{Alias: "web"}, refuse: "already uses the alias"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := sshTestModel(t, ScreenSSHForm)
			m.SSH.Form = tt.form
			m.SSH.FormMode = "create"

			refusals := strings.Join(m.sshFormRefusals(), "\n")
			if !strings.Contains(refusals, tt.refuse) {
				t.Errorf("the form states %q, want a refusal naming %q", refusals, tt.refuse)
			}
			if m.sshFormReady() {
				t.Error("the form is ready to write an entry the writer would refuse")
			}
			if containsRow(m.GetCurrentOptions(), sshSaveRow) {
				t.Error("the save row is offered for a form that would be refused")
			}
		})
	}

	good := sshTestModel(t, ScreenSSHForm)
	good.SSH.Form = sshHostForm{Alias: "staging", HostName: "staging.example.com", User: "deploy", Port: "22"}
	good.SSH.FormMode = "create"
	if !good.sshFormReady() {
		t.Fatalf("a valid form is refused: %v", good.sshFormRefusals())
	}
	if !containsRow(good.GetCurrentOptions(), sshSaveRow) {
		t.Error("a valid form offers no save row")
	}
}

// TestSSHFormTypingReachesOnlyTheFocusedField covers the input rule: a character
// lands in the field under the cursor, and every navigation key moves rather than
// typing.
func TestSSHFormTypingReachesOnlyTheFocusedField(t *testing.T) {
	m := sshTestModel(t, ScreenSSHForm)
	m.SSH.Form = sshHostForm{}
	m.SSH.FormMode = "create"
	m.Cursor = 0

	for _, key := range []string{"w", "e", "b"} {
		next, _ := m.handleSSHFormKeys(key)
		m = next.(Model)
	}
	if m.SSH.Form.Alias != "web" {
		t.Fatalf("the alias field holds %q, want %q", m.SSH.Form.Alias, "web")
	}

	next, _ := m.handleSSHFormKeys("down")
	m = next.(Model)
	if m.Cursor != 1 {
		t.Fatalf("the cursor is on row %d after down, want 1", m.Cursor)
	}
	next, _ = m.handleSSHFormKeys("h")
	m = next.(Model)
	if m.SSH.Form.HostName != "h" {
		t.Errorf("the host name field holds %q, want %q", m.SSH.Form.HostName, "h")
	}
	if m.SSH.Form.Alias != "web" {
		t.Errorf("typing in the host name changed the alias to %q", m.SSH.Form.Alias)
	}

	next, _ = m.handleSSHFormKeys("backspace")
	m = next.(Model)
	if m.SSH.Form.HostName != "" {
		t.Errorf("backspace left the host name as %q, want empty", m.SSH.Form.HostName)
	}
}

// TestSSHFormGivesUpThePlanBeforeTheScreen covers the two-step Escape the footer
// names: a previewed change is discarded first, and only a second Escape leaves
// the form.
func TestSSHFormGivesUpThePlanBeforeTheScreen(t *testing.T) {
	m := sshTestModel(t, ScreenSSHForm)
	m.SSH.Form = sshHostForm{Alias: "staging"}
	m.SSH.FormMode = "create"
	m.SSH.Plan = &sshcfg.WritePlan{Path: "/home/testuser/.ssh/config", Summary: "add staging"}

	next, _ := m.handleSSHFormKeys("esc")
	m = next.(Model)
	if m.Screen != ScreenSSHForm {
		t.Errorf("the first Escape left the form for screen %v", m.Screen)
	}
	if m.SSH.Plan != nil {
		t.Error("the first Escape did not discard the plan")
	}
	if !strings.Contains(m.SSH.Notice, "Nothing was written") {
		t.Errorf("the screen does not say nothing was written: %q", m.SSH.Notice)
	}

	next, _ = m.handleSSHFormKeys("esc")
	m = next.(Model)
	if m.Screen != ScreenSSHManager {
		t.Errorf("the second Escape left the form for screen %v, want the manager", m.Screen)
	}
	if m.SSH.FormMode != "" {
		t.Errorf("the form mode survived the way out: %q", m.SSH.FormMode)
	}
}

// TestSSHPlanConfirmationOffersOnlyTheTwoWaysOut covers the confirmation the
// write layer's plan produces: the rows are the write and the refusal, and the
// plan is what the body shows.
func TestSSHPlanConfirmationOffersOnlyTheTwoWaysOut(t *testing.T) {
	m := sshTestModel(t, ScreenSSHForm)
	m.SSH.Plan = &sshcfg.WritePlan{
		Path:    "/home/testuser/.ssh/config",
		Backup:  "/home/testuser/.ssh/config.bak-dotfiles-20260101-120000",
		Summary: "Add the entry staging inside the block this tool manages.",
		Preview: []string{"  Host web", "+ Host staging", "+     HostName staging.example.com"},
	}

	rows := m.GetCurrentOptions()
	if len(rows) != 2 || rows[0] != sshSaveRow || rows[1] != sshDiscardRow {
		t.Fatalf("the confirmation offers %v, want exactly the write and the refusal", rows)
	}
	body := strings.Join(m.sshFormDescription(), "\n")
	for _, want := range []string{"staging", "backup", "+ Host staging"} {
		if !strings.Contains(body, want) {
			t.Errorf("the confirmation does not show %q:\n%s", want, body)
		}
	}
}

// TestSSHWriteResultReturnsToTheListAndReReadsTheFile covers the outcome of a
// write: the screen goes back to the list, the notice names the backup and the
// validation, and the command that follows re-reads the configuration.
func TestSSHWriteResultReturnsToTheListAndReReadsTheFile(t *testing.T) {
	m := sshTestModel(t, ScreenSSHForm)
	m.SSH.Plan = &sshcfg.WritePlan{Path: "/home/testuser/.ssh/config"}
	m.SSH.FormMode = "create"

	next, cmd, handled := m.handleSSHMessage(sshWrittenMsg{
		result: sshcfg.WriteResult{
			Path:      "/home/testuser/.ssh/config",
			Backup:    "/home/testuser/.ssh/config.bak-dotfiles-20260101-120000",
			Validated: true,
		},
	})
	if !handled {
		t.Fatal("the write result was not handled")
	}
	got := next.(Model)
	if got.Screen != ScreenSSHManager {
		t.Errorf("the write left the screen on %v, want the manager", got.Screen)
	}
	if got.SSH.Plan != nil {
		t.Error("the plan survived the write")
	}
	if !strings.Contains(got.SSH.Notice, "backup") || !strings.Contains(got.SSH.Notice, "validated") {
		t.Errorf("the notice does not name the backup and the validation: %q", got.SSH.Notice)
	}
	if cmd == nil {
		t.Error("nothing re-reads the configuration after a write")
	}

	failed, _, _ := m.handleSSHMessage(sshWrittenMsg{err: errTestWrite})
	if !strings.Contains(failed.(Model).SSH.Notice, "Nothing was written") {
		t.Errorf("a refused write does not say nothing was written: %q", failed.(Model).SSH.Notice)
	}
	if failed.(Model).SSH.Plan == nil {
		t.Error("a refused write discarded the plan, so there is nothing left to retry")
	}
}

// errTestWrite is the refusal a refused write carries.
var errTestWrite = testError("the client refused the file")

// testError is a minimal error for the tests here: it carries one sentence and no
// dependency on the sshcfg package's own error values.
type testError string

func (e testError) Error() string { return string(e) }

// TestSSHStaleTestAnswerIsDropped covers the cancellation rule: an answer from a
// run the user abandoned must not be drawn over a newer one.
func TestSSHStaleTestAnswerIsDropped(t *testing.T) {
	m := sshTestModel(t, ScreenSSHTest)
	m.SSH.Testing = true
	m.SSH.TestSeq = 2
	m.SSH.Tested = false

	stale, _, handled := m.handleSSHMessage(sshTestedMsg{seq: 1, result: sshcfg.ConnectResult{Outcome: sshcfg.OutcomeTimeout}})
	if !handled {
		t.Fatal("the stale answer was not handled")
	}
	if stale.(Model).SSH.Tested {
		t.Error("a stale answer was drawn as the current result")
	}

	fresh, _, _ := m.handleSSHMessage(sshTestedMsg{seq: 2, result: sshcfg.ConnectResult{Outcome: sshcfg.OutcomeOK}})
	got := fresh.(Model)
	if !got.SSH.Tested || got.SSH.Testing {
		t.Errorf("the current answer was not applied: tested=%v testing=%v", got.SSH.Tested, got.SSH.Testing)
	}
}

// TestSSHTestScreensaysWhatItWillRunBeforeItRuns covers the spec's rule that a
// connection is tested only on request: with nothing pressed, the screen says the
// test has not run and what it would do.
func TestSSHTestScreenSaysWhatItWillRunBeforeItRuns(t *testing.T) {
	m := sshTestModel(t, ScreenSSHTest)
	m.SSH.Tested = false
	m.SSH.Testing = false

	body := strings.Join(m.sshTestDescription(), "\n")
	for _, want := range []string{"Nothing is sent anywhere until that press", "Host keys", "verified", "never prompted for"} {
		if !strings.Contains(body, want) {
			t.Errorf("the test screen does not state %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "The connection opened") {
		t.Error("the screen claims a result before any test ran")
	}
}

// TestSSHHostKeyChangeIsNeverHidden covers the loudest rule in the spec: a changed
// host key is named as such, and the screen says nothing was changed.
func TestSSHHostKeyChangeIsNeverHidden(t *testing.T) {
	m := sshTestModel(t, ScreenSSHTest)
	m.SSH.Test = sshcfg.ConnectResult{
		Alias:          "web",
		Outcome:        sshcfg.OutcomeHostKeyChanged,
		Explanation:    "The host key changed, and nothing was accepted, replaced or removed.",
		HostKeyChanged: true,
		Stderr:         "@@@@@@@@@@ WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED! @@@@@@@@@@",
	}

	body := strings.Join(m.sshTestDescription(), "\n")
	if !strings.Contains(body, "The host key changed") {
		t.Errorf("the outcome is not named in words:\n%s", body)
	}
	if !strings.Contains(body, "nothing was accepted") {
		t.Errorf("the screen does not say that nothing was changed:\n%s", body)
	}
	if !strings.Contains(body, "REMOTE HOST IDENTIFICATION HAS CHANGED") {
		t.Errorf("the client's own warning is hidden:\n%s", body)
	}
}

// TestSSHAgentScreenNamesWhatTheAgentIsAndIsNot covers the honesty the spec
// demands of the agent integration: the kind, the differences and the reason it
// could not be reached are all on the screen.
func TestSSHAgentScreenNamesWhatTheAgentIsAndIsNot(t *testing.T) {
	m := sshTestModel(t, ScreenSSHAgent)
	m.SSH.Agent = sshcfg.AgentState{
		Advertised: true,
		Socket:     "/run/user/1000/gnupg/S.gpg-agent.ssh",
		Reachable:  true,
		Kind:       "gpg-agent (SSH support)",
		Notes:      []string{"gpg-agent ignores ssh-add's lifetime option, so a key added here has gpg-agent's own expiry."},
		Identities: []sshcfg.AgentIdentity{{Fingerprint: "SHA256:aaaa", Comment: "card key", Type: "ED25519"}},
		CanAdd:     true,
		CanRemove:  true,
	}
	body := strings.Join(m.sshAgentDescription(), "\n")
	for _, want := range []string{"gpg-agent (SSH support)", "ignores ssh-add's lifetime option"} {
		if !strings.Contains(body, want) {
			t.Errorf("the agent screen does not state %q:\n%s", want, body)
		}
	}
	rows := m.GetCurrentOptions()
	if !containsRow(rows, sshCommandRow("x", sshRemoveIdentityPrefix+"1.  SHA256:aaaa  card key  (ED25519)")) {
		t.Errorf("the identity is not offered as a removable row: %v", rows)
	}

	unreachable := sshTestModel(t, ScreenSSHAgent)
	unreachable.SSH.Agent = sshcfg.AgentState{
		Advertised: true,
		Socket:     "/tmp/ssh-XXXX/agent.9",
		Reachable:  false,
		Kind:       "OpenSSH ssh-agent",
		Reason:     "Could not open a connection to your authentication agent.",
	}
	body = strings.Join(unreachable.sshAgentDescription(), "\n")
	if !strings.Contains(body, "Could not open a connection") {
		t.Errorf("the refusal reason is not on the screen:\n%s", body)
	}
}

// TestSSHAgentRemoveAllTakesTwoPresses covers the consent rule for the one
// destructive agent action: the first press of its own key arms it and says what
// it does, and only the second runs it. Enter is not one of the presses: the row
// carries its letter, so Enter on it leaves the agent alone.
func TestSSHAgentRemoveAllTakesTwoPresses(t *testing.T) {
	m := sshTestModel(t, ScreenSSHAgent)
	rows := m.GetCurrentOptions()
	cursor := -1
	for i, row := range rows {
		if row == sshRemoveEveryIdentityRow {
			cursor = i
		}
	}
	if cursor < 0 {
		t.Fatalf("the removal row is not offered: %v", rows)
	}
	m.Cursor = cursor

	next, cmd := m.handleSSHAgentKeys("enter")
	if cmd != nil || next.(Model).SSH.AgentConfirm {
		t.Error("Enter ran a row that carries its own key")
	}

	next, cmd = m.handleSSHAgentKeys("r")
	armed := next.(Model)
	if cmd != nil {
		t.Error("the first press ran the removal instead of arming it")
	}
	if !armed.SSH.AgentConfirm {
		t.Error("the first press did not arm the removal")
	}
	if !strings.Contains(armed.SSH.Notice, "Press [r] again") {
		t.Errorf("the first press does not say what the second will do: %q", armed.SSH.Notice)
	}

	next, cmd = armed.handleSSHAgentKeys("r")
	run := next.(Model)
	if cmd == nil {
		t.Error("the second press did not run the removal")
	}
	if run.SSH.AgentConfirm {
		t.Error("the confirmation survived the run")
	}
}

// TestSSHRemoveIdentityNeedsAPublicFile covers the refusal that would otherwise be
// a tool error: ssh-add removes by public key file, so an identity with no
// matching file is refused with that reason before anything is run.
func TestSSHRemoveIdentityNeedsAPublicFile(t *testing.T) {
	m := sshTestModel(t, ScreenSSHAgent)
	m.SSH.Keys = nil // the agent holds an identity no file on this machine matches

	next, cmd := m.removeAgentIdentity(0)
	if cmd != nil {
		t.Error("the removal ran without a public key file to name")
	}
	if !strings.Contains(next.(Model).SSH.Notice, "ssh-add removes an identity by its public key file") {
		t.Errorf("the refusal does not name the mechanism: %q", next.(Model).SSH.Notice)
	}
}

// TestSSHKeyRowsNameWhatTheListNeeds covers the key list: the file, the type and
// size, the fingerprint, and the mode only when it is wrong.
func TestSSHKeyRowsNameWhatTheListNeeds(t *testing.T) {
	m := sshTestModel(t, ScreenSSHKeys)
	rows := m.GetCurrentOptions()
	if !strings.Contains(rows[0], "id_ed25519") || !strings.Contains(rows[0], "ed25519 256 bits") || !strings.Contains(rows[0], "SHA256:aaaa") {
		t.Errorf("the first key row is %q", rows[0])
	}
	if strings.Contains(rows[0], "mode") {
		t.Errorf("a key with correct permissions names a mode: %q", rows[0])
	}
	if !strings.Contains(rows[1], "mode 0644") {
		t.Errorf("a key that is readable by others does not name its mode: %q", rows[1])
	}
	body := strings.Join(m.sshKeyDescription(), "\n")
	if !strings.Contains(body, "A private key is never read by this screen") {
		t.Errorf("the key screen does not state the rule it follows:\n%s", body)
	}
	if !strings.Contains(body, "github.com") {
		t.Errorf("the known hosts are not named:\n%s", body)
	}
	if !strings.Contains(body, "hashed") {
		t.Errorf("the hashed known-host entry is not accounted for:\n%s", body)
	}
}

// TestSSHKeygenFormKeepsThePassphraseOffTheScreen covers the passphrase rule on
// the generation form: the field draws bullets, the form is ready only when the
// two entries agree, and the type cycles through what the client supports.
func TestSSHKeygenFormKeepsThePassphraseOffTheScreen(t *testing.T) {
	m := sshTestModel(t, ScreenSSHKeys)
	m.SSH.KeysMode = "generate"
	m.SSH.Keygen = sshKeygenForm{Path: "/home/testuser/.ssh/id_ed25519", Type: "ed25519", Passphrase: "hunter2", Confirm: "hunter"}

	rows := m.GetCurrentOptions()
	joined := strings.Join(rows, "\n")
	if strings.Contains(joined, "hunter2") {
		t.Errorf("the passphrase is on the screen:\n%s", joined)
	}
	if !strings.Contains(joined, sshMask("hunter2")) {
		t.Errorf("the passphrase is not masked:\n%s", joined)
	}
	if m.sshKeygenReady() {
		t.Error("the form is ready while the two passphrases differ")
	}
	if containsRow(rows, sshGenerateKeyRow) {
		t.Error("the generate row is offered for a form that would be refused")
	}
	if !containsRow(rows, sshReplaceRowOff) {
		t.Errorf("the replace switch is not offered with its state: %v", rows)
	}

	m.SSH.Keygen.Confirm = "hunter2"
	if !m.sshKeygenReady() {
		t.Fatal("the form is refused with two matching passphrases")
	}

	if got := sshCycleType("ed25519", m.sshCaps(), true); got != "sk-ssh-ed25519@openssh.com" {
		t.Errorf("the type cycles to %q, want the same algorithm on a token", got)
	}
	if got := sshCycleType("ed25519", m.sshCaps(), false); got != "rsa" {
		t.Errorf("cycling backwards from the first type reaches %q, want the last", got)
	}
	if got := sshCycleType("ecdsa", m.sshCaps(), true); got != "sk-ecdsa-sha2-nistp256@openssh.com" {
		t.Errorf("the type cycles to %q, want the token form of ECDSA", got)
	}
}

// TestSSHManagerSaysWhatTheHostCannotDo covers the degradation rule: a host with
// no client and no key generator says so instead of offering rows that fail.
func TestSSHManagerSaysWhatTheHostCannotDo(t *testing.T) {
	m := sshTestModel(t, ScreenSSHManager)
	m.SSH.Manager.Caps = sshcfg.Caps{}
	m.SSH.Manager.Caps.AgentSocket = ""

	body := strings.Join(m.sshManagerDescription(), "\n")
	for _, want := range []string{"ssh: not found on PATH", "ssh-keygen: not found", "no agent is advertised"} {
		if !strings.Contains(body, want) {
			t.Errorf("the manager does not state %q:\n%s", want, body)
		}
	}

	test := sshTestModel(t, ScreenSSHTest)
	test.SSH.Manager.Caps = sshcfg.Caps{}
	if body := strings.Join(test.sshTestDescription(), "\n"); !strings.Contains(body, "ssh was not found on PATH") {
		t.Errorf("the test screen offers a test it cannot run:\n%s", body)
	}
	if containsRow(test.GetCurrentOptions(), sshRunTestRow) {
		t.Error("the test screen offers the run row with no client to run")
	}
}

// TestSSHClipboardCopyNeverUsesAnArgumentList covers the copying rule: the public
// key goes to the tool's standard input, and a host with no clipboard tool says so
// instead of claiming the copy happened.
func TestSSHClipboardCopyNeverUsesAnArgumentList(t *testing.T) {
	previous := lookPathFunc
	defer func() { lookPathFunc = previous }()

	lookPathFunc = func(name string) (string, error) {
		if name == "xclip" {
			return "/usr/bin/xclip", nil
		}
		return "", exec.ErrNotFound
	}
	mechanism, command, err := sshClipboardTool()
	if err != nil {
		t.Fatalf("a host with xclip has no clipboard: %v", err)
	}
	if mechanism != "xclip" {
		t.Errorf("the mechanism is %q, want xclip", mechanism)
	}
	for _, arg := range command {
		if strings.Contains(arg, "ssh-ed25519") {
			t.Errorf("the key is in the argument list: %v", command)
		}
	}

	lookPathFunc = func(string) (string, error) { return "", exec.ErrNotFound }
	if _, _, err := sshClipboardTool(); err == nil {
		t.Error("a host with no clipboard tool reports one")
	}
}

// TestSSHToolOutputIsBounded covers the rule that evidence is shown but a wall of
// it is not: the cut is marked and counted.
func TestSSHToolOutputIsBounded(t *testing.T) {
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "line")
	}
	bounded := sshBoundedLines(strings.Join(lines, "\n"), 6)
	if len(bounded) != 6 {
		t.Fatalf("the output is %d rows, want 6", len(bounded))
	}
	if !strings.Contains(bounded[5], "and 15 more") {
		t.Errorf("the cut is not marked and counted: %q", bounded[5])
	}
	if got := sshBoundedLines("", 6); got != nil {
		t.Errorf("empty output produced %v", got)
	}
}

// TestSSHFindingsAreOrderedAndNamed covers the diagnosis screen: the severity is a
// word on the row, the counts are on the body, and the selected finding's own fix
// is named.
func TestSSHFindingsAreOrderedAndNamed(t *testing.T) {
	m := sshTestModel(t, ScreenSSHDiagnostics)
	rows := m.GetCurrentOptions()
	if !strings.HasPrefix(rows[0], "[problem]") {
		t.Errorf("the first row is %q, want the problem first", rows[0])
	}
	if !strings.Contains(rows[1], "[warning]") || !strings.Contains(rows[2], "[info]") {
		t.Errorf("the severities are not named in order: %v", rows[:3])
	}

	body := strings.Join(m.sshDiagnosticDescription(), "\n")
	for _, want := range []string{"1 problem", "1 warning", "1 note", "chmod 600 ~/.ssh/id_rsa"} {
		if !strings.Contains(body, want) {
			t.Errorf("the diagnosis body does not state %q:\n%s", want, body)
		}
	}
}

// TestSSHManagerEscapesStepBackOneLevel covers the navigation rule: every screen
// of the manager steps back to the one that opened it, and the hub steps back to
// the utilities section.
func TestSSHManagerEscapesStepBackOneLevel(t *testing.T) {
	tests := []struct {
		screen Screen
		want   Screen
	}{
		{ScreenSSHManager, ScreenUtilities},
		{ScreenSSHHost, ScreenSSHManager},
		{ScreenSSHForm, ScreenSSHManager},
		{ScreenSSHKeys, ScreenSSHManager},
		{ScreenSSHAgent, ScreenSSHManager},
		{ScreenSSHDiagnostics, ScreenSSHManager},
		{ScreenSSHTest, ScreenSSHManager},
	}
	for _, tt := range tests {
		t.Run(tt.screen.String(), func(t *testing.T) {
			m := sshTestModel(t, tt.screen)
			next, _ := m.handleKeyPressSSH(tea.KeyMsg{Type: tea.KeyEsc})
			got := next.(Model)
			if got.Screen != tt.want {
				t.Errorf("Escape on %v reached %v, want %v", tt.screen, got.Screen, tt.want)
			}
		})
	}
}

// TestSSHScreensRenderInsideTheFloorFrame is the local frame guard for the
// manager's seven screens. The shared guard in teatest_test.go measures the same
// screens through installerFrameScreenNames; this one keeps the manager's own
// cases measurable while that list is being extended.
func TestSSHScreensRenderInsideTheFloorFrame(t *testing.T) {
	screens := []Screen{
		ScreenSSHManager, ScreenSSHHost, ScreenSSHForm, ScreenSSHKeys,
		ScreenSSHAgent, ScreenSSHDiagnostics, ScreenSSHTest,
	}
	for _, screen := range screens {
		t.Run(screen.String(), func(t *testing.T) {
			m := sshTestModel(t, screen)
			assertInstallerScreenFits(t, screen.String(), m)
		})
	}
}

// TestSSHManagerStateNeverCarriesAPrivateKey is the guard that the manager's
// screens hold public material only: every string in the state is scanned for the
// first line of a private key, which is the one shape that must never appear.
func TestSSHManagerStateNeverCarriesAPrivateKey(t *testing.T) {
	m := sshTestModel(t, ScreenSSHManager)
	for _, text := range []string{
		strings.Join(m.sshManagerDescription(), "\n"),
		strings.Join(m.sshKeyDescription(), "\n"),
		strings.Join(m.sshHostDescription(), "\n"),
		strings.Join(m.sshAgentDescription(), "\n"),
		strings.Join(m.sshDiagnosticDescription(), "\n"),
		strings.Join(m.sshTestDescription(), "\n"),
	} {
		if strings.Contains(text, "BEGIN OPENSSH PRIVATE KEY") || strings.Contains(text, "BEGIN RSA PRIVATE KEY") {
			t.Fatalf("a screen holds private key material:\n%s", text)
		}
	}
	for _, key := range m.SSH.Keys {
		if strings.Contains(key.Public, "PRIVATE KEY") {
			t.Errorf("the key inventory carries private material for %s", key.Path)
		}
	}
}

// TestSSHFormFromHostPrefersTheClientAnswer covers the rule behind the form's
// initial values: what the entry declares wins, and the client's effective answer
// fills the rest, so editing an entry that inherits `User` from a wildcard block
// shows the user that would really be used.
func TestSSHFormFromHostPrefersTheClientAnswer(t *testing.T) {
	host := sshcfg.Host{
		Alias:   "web",
		Options: []sshcfg.Option{{Key: "HostName", Value: "declared.example.com", File: "f", Line: 1}},
	}
	effective := sshcfg.Effective{
		Alias:         "web",
		HostName:      "effective.example.com",
		User:          "deploy",
		Port:          "2222",
		IdentityFiles: []string{"~/.ssh/id_ed25519"},
	}
	form := sshFormFromHost(host, effective)
	if form.HostName != "declared.example.com" {
		t.Errorf("the form's host name is %q, want the declared one", form.HostName)
	}
	if form.User != "deploy" || form.Port != "2222" || form.Identity != "~/.ssh/id_ed25519" {
		t.Errorf("the form did not take the effective values: %+v", form)
	}
}

// TestSSHPathsAreComparedInOneForm covers the matching rule behind the detail
// screen's public key: ~/.ssh/id_ed25519 and /home/user/.ssh/id_ed25519 are the
// same key, and a path under another home is not. Both sides are reduced to their
// form relative to the home the paths were resolved for, so no absolute path is
// rebuilt from `~`.
func TestSSHPathsAreComparedInOneForm(t *testing.T) {
	home := sshcfg.DefaultPaths().Home
	if home == "" {
		t.Skip("no home directory to compare against")
	}
	if !sshSamePath("~/.ssh/id_ed25519", filepath.Join(home, ".ssh", "id_ed25519"), home) {
		t.Error("the same key under ~ and under the home directory is treated as two")
	}
	if sshSamePath("~/.ssh/id_ed25519", "/home/someone-else/.ssh/id_ed25519", home) {
		t.Error("another user's key is treated as this one's")
	}
	if sshSamePath("", "~/.ssh/id_ed25519", home) {
		t.Error("an empty path is treated as a key")
	}
}

// TestSSHManagerMessageFillsTheState covers the load path: the configuration the
// command read is what the screens then draw.
func TestSSHManagerMessageFillsTheState(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Screen = ScreenSSHManager

	manager := &sshcfg.Manager{Paths: sshcfg.Paths{Home: "/home/testuser", UserConfig: "/home/testuser/.ssh/config"}}
	next, _, handled := m.handleSSHMessage(sshManagerLoadedMsg{
		manager: manager,
		config: sshcfg.Config{
			Hosts:       []sshcfg.Host{{Alias: "web", Managed: true}},
			Files:       []sshcfg.ConfigFile{{Path: "/home/testuser/.ssh/config"}},
			Failures:    []string{"the Include on line 3 matched nothing"},
			MatchBlocks: 2,
		},
	})
	if !handled {
		t.Fatal("the load result was not handled")
	}
	got := next.(Model)
	if got.SSH.Manager != manager || !got.SSH.HostsLoaded {
		t.Fatalf("the manager or the loaded flag was not set: %+v", got.SSH)
	}
	if len(got.SSH.Hosts) != 1 || got.SSH.Hosts[0].Alias != "web" {
		t.Errorf("the hosts are %+v", got.SSH.Hosts)
	}
	if got.SSH.MatchBlocks != 2 {
		t.Errorf("the Match count is %d, want 2", got.SSH.MatchBlocks)
	}
	body := strings.Join(got.sshManagerDescription(), "\n")
	for _, want := range []string{"~/.ssh/config", "matched nothing", "Match block"} {
		if !strings.Contains(body, want) {
			t.Errorf("the manager body does not state %q:\n%s", want, body)
		}
	}
}

// TestSSHContextIsNotHeldByARender is the compile-time half of the render rule:
// the description functions take no context and return no error, so a render
// cannot run a command. This test fails to compile if one of them grows a
// context parameter.
func TestSSHContextIsNotHeldByARender(t *testing.T) {
	m := sshTestModel(t, ScreenSSHManager)
	var _ []string = m.sshManagerDescription()
	var _ []string = m.sshKeyDescription()
	var _ []string = m.sshAgentDescription()
	var _ []string = m.sshDiagnosticDescription()
	var _ []string = m.sshTestDescription()
	var _ []string = m.sshHostDescription()
	var _ []string = m.sshFormDescription()
	var _ context.Context = context.Background()
}

// ============================================================================
// THE KEY GRAMMAR, AS THESE SCREENS FOLLOW IT
// ============================================================================
//
// odd/tasks/key-grammar.md fixes one grammar for every installer list: Enter
// opens a screen or a row's detail and never runs a command; Space toggles a
// switch and nothing else; every command runs on its own letter, said on the row
// and in the footer, and never on a move key. These tests pin that grammar for
// the seven SSH screens from both sides: every row is pressed with the key it
// advertises and must do something, and every command row is pressed with Enter
// and must do nothing.

// sshGrammarState prepares one state of one of the manager's screens, so the
// rows that state exists to cover are offered.
type sshGrammarState struct {
	name    string
	screen  Screen
	prepare func(m *Model)
}

// sshGrammarStates is every state the manager's screens reach in normal use,
// including the two sub-states of the key screen and the two of the test screen:
// the rows and the footer differ in each, and a grammar read from one state would
// miss a letter that only the other offers.
func sshGrammarStates() []sshGrammarState {
	return []sshGrammarState{
		{name: "manager", screen: ScreenSSHManager},
		{name: "host", screen: ScreenSSHHost, prepare: func(m *Model) { m.SSH.HostIndex = 0 }},
		{name: "host read-only", screen: ScreenSSHHost, prepare: func(m *Model) { m.SSH.HostIndex = 1 }},
		{name: "form", screen: ScreenSSHForm, prepare: func(m *Model) {
			m.SSH.FormMode = "create"
			m.SSH.Form = sshHostForm{Alias: "staging", HostName: "staging.example.com", User: "deploy", Port: "22"}
		}},
		{name: "form confirmation", screen: ScreenSSHForm, prepare: func(m *Model) {
			m.SSH.FormMode = "create"
			m.SSH.Form = sshHostForm{Alias: "staging", HostName: "staging.example.com"}
			m.SSH.Plan = &sshcfg.WritePlan{Path: "/home/testuser/.ssh/config", Summary: "add staging"}
		}},
		{name: "keys with the selection in the agent", screen: ScreenSSHKeys},
		{name: "keys with the selection not in the agent", screen: ScreenSSHKeys, prepare: func(m *Model) {
			// The agent holds the first key, so selecting the second one swaps the
			// remove row for the add row: both rows have to be walked.
			m.Cursor = 1
			m.SSH.KeyIndex = 1
		}},
		{name: "key generation form", screen: ScreenSSHKeys, prepare: func(m *Model) {
			m.SSH.KeysMode = "generate"
			m.SSH.Keygen = sshKeygenForm{Path: "/home/testuser/.ssh/id_ed25519", Type: "ed25519"}
		}},
		{name: "key generation form with its switch on", screen: ScreenSSHKeys, prepare: func(m *Model) {
			m.SSH.KeysMode = "generate"
			m.SSH.Keygen = sshKeygenForm{Path: "/home/testuser/.ssh/id_ed25519", Type: "ed25519", Replace: true}
		}},
		{name: "add-to-agent prompt", screen: ScreenSSHKeys, prepare: func(m *Model) {
			m.SSH.KeysMode = "add"
		}},
		{name: "agent", screen: ScreenSSHAgent},
		{name: "agent holding two identities", screen: ScreenSSHAgent, prepare: func(m *Model) {
			// The identity rows carry the same letter, because it is one command --
			// remove the identity under the cursor -- drawn on each of them.
			m.SSH.Agent.Identities = append(m.SSH.Agent.Identities,
				sshcfg.AgentIdentity{Bits: 4096, Fingerprint: "SHA256:bbbb", Comment: "testuser@host", Type: "RSA"})
		}},
		{name: "agent with nothing to remove", screen: ScreenSSHAgent, prepare: func(m *Model) {
			m.SSH.Agent.Identities = nil
		}},
		{name: "diagnosis", screen: ScreenSSHDiagnostics},
		{name: "connection test", screen: ScreenSSHTest},
		{name: "connection test running", screen: ScreenSSHTest, prepare: func(m *Model) {
			m.SSH.Testing = true
		}},
	}
}

// sshGrammarModel is one prepared state at the frame floor.
func sshGrammarModel(t *testing.T, state sshGrammarState) Model {
	t.Helper()
	m := sshTestModel(t, state.screen)
	if state.prepare != nil {
		state.prepare(&m)
	}
	return m
}

// The kinds a drawn SSH row can be, read from the mark at its front.
const (
	sshRowData    = iota // a fact in a list -- a key, a finding, a field
	sshRowOpen           // the arrow: Enter opens a screen or a row's detail
	sshRowCommand        // a letter in brackets: that letter runs it
	sshRowSwitch         // a state in brackets: Space toggles it
)

// sshRowKey reads the key a drawn row advertises and the kind that key belongs
// to. A row that carries none of the marks is a fact in a list, and a fact has no
// key: the walk leaves those alone rather than guessing.
func sshRowKey(row string) (string, int) {
	switch {
	case strings.HasPrefix(row, utilitiesScreenMark):
		return "enter", sshRowOpen
	case strings.HasPrefix(row, "← "):
		return "enter", sshRowOpen
	case sshIsReplaceRow(row):
		// The switch is read before the bracket rule: "(x) Replace" carries the
		// state, not a command called x.
		return " ", sshRowSwitch
	case len(row) >= 4 && row[0] == '[' && row[2] == ']' && row[3] == ' ' && row[1] >= 'a' && row[1] <= 'z':
		return row[1:2], sshRowCommand
	}
	return "", sshRowData
}

// sshGrammarKey is the key message for a key the walk read off a row.
func sshGrammarKey(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

// sshFooterNames reports whether the footer of a screen offers one key. It reads
// the same hints the frame draws, so the row and the footer cannot name different
// keys without failing here.
func sshFooterNames(m Model, key string) bool {
	for _, hint := range m.sshHints() {
		for _, token := range hintKeyTokens(hint.keys) {
			if token == key {
				return true
			}
		}
	}
	return false
}

// TestEverySSHRowAnswersTheKeyItAdvertises walks every row of every state of the
// manager's screens and presses exactly the key the row itself carries. A row
// that names a key nothing answers is a dead row: the user presses what the row
// offers and nothing happens, which is indistinguishable from a key that was
// never delivered. #372 was exactly that, and the key screen's action rows were
// the same shape before this pass: they were drawn from the key under the cursor,
// so walking down onto them made them disappear.
//
// The same walk pins the grammar's other side: Enter must not run a command or
// toggle a switch, and the footer must name the keys the rows carry -- and Enter
// only where a row opens something.
func TestEverySSHRowAnswersTheKeyItAdvertises(t *testing.T) {
	for _, state := range sshGrammarStates() {
		t.Run(state.name, func(t *testing.T) {
			m := sshGrammarModel(t, state)
			rows := m.GetCurrentOptions()
			if len(rows) == 0 {
				t.Fatal("the state offers no rows, so this walk would pass by looking at nothing")
			}

			openers := 0
			switches := 0
			for index, row := range rows {
				if strings.HasPrefix(row, menuSeparatorPrefix) {
					continue
				}
				key, kind := sshRowKey(row)
				if kind == sshRowData {
					continue
				}

				switch kind {
				case sshRowOpen:
					openers++
				case sshRowSwitch:
					switches++
				case sshRowCommand:
					for _, move := range []string{"j", "k", "h", "l"} {
						if key == move {
							t.Errorf("the row %q runs a command on %q, which is a move key", row, key)
						}
					}
				}

				// The footer names the same key the row carries. The way back is the
				// exception the interface has always had: Esc is its key and the
				// footer names Esc, while Enter also answers it on every screen the
				// way the Utilities section's own back row does.
				named := key
				switch {
				case kind == sshRowOpen && strings.HasPrefix(row, "← "):
					named = "Esc"
				case key == " ":
					named = "Space"
				case key == "enter":
					named = "Enter"
				}
				if !sshFooterNames(m, named) {
					t.Errorf("the row %q carries %q and the footer does not name it", row, named)
				}

				// The row's own key does something: it changes the state or hands
				// back the command that carries the change out. The way back is
				// pressed with the Esc the footer names for it.
				walked := key
				if kind == sshRowOpen && strings.HasPrefix(row, "← ") {
					walked = "esc"
				}
				probe := sshGrammarModel(t, state)
				probe.Cursor = index
				before := probe.View()
				next, cmd := probe.handleKeyPressSSH(sshGrammarKey(walked))
				if landed := next.(Model); cmd == nil && landed.View() == before {
					t.Errorf("the row %q names %q and pressing it changed nothing", row, walked)
				}

				// Enter never runs a command or toggles a switch.
				if kind == sshRowCommand || kind == sshRowSwitch {
					probe := sshGrammarModel(t, state)
					probe.Cursor = index
					before := probe.View()
					next, cmd := probe.handleKeyPressSSH(tea.KeyMsg{Type: tea.KeyEnter})
					if landed := next.(Model); cmd != nil || landed.View() != before {
						t.Errorf("the row %q acts where it stands and still answers Enter", row)
					}
				}
			}

			// One letter per command: the footer names each command's key once. It is
			// read from the footer rather than from the rows because a list row per
			// item -- an identity, say -- carries the same single command on each of
			// its rows.
			counts := map[string]int{}
			for _, hint := range m.sshHints() {
				for _, token := range hintKeyTokens(hint.keys) {
					if len(token) == 1 && token[0] >= 'a' && token[0] <= 'z' {
						counts[token]++
					}
				}
			}
			for key, count := range counts {
				if count > 1 {
					t.Errorf("the state offers %q for more than one command: %+v", key, m.sshHints())
				}
			}
			if sshFooterNames(m, "Enter") && openers == 0 {
				t.Errorf("the footer offers Enter and no row opens anything: %v", rows)
			}
			if sshFooterNames(m, "Space") && switches == 0 {
				t.Errorf("the footer offers Space and no row is a switch: %v", rows)
			}
		})
	}
}

// TestSSHRowsCarryTheGrammarTheyFollow is the pin the other test leans on: the
// exact mark on every action row of every screen, so a change to one of them is
// a change to this list and to the footer beside it rather than a quiet drift.
func TestSSHRowsCarryTheGrammarTheyFollow(t *testing.T) {
	tests := []struct {
		name   string
		state  sshGrammarState
		want   []string
		absent []string
	}{
		{
			name:  "the hub",
			state: sshGrammarState{screen: ScreenSSHManager},
			want: []string{
				utilitiesScreenRow(sshAddHostRow),
				utilitiesScreenRow(sshKeysRow),
				utilitiesScreenRow(sshAgentRow),
				utilitiesScreenRow(sshDiagnoseRow),
				utilitiesScreenRow(sshTestRow),
				sshBackRow,
			},
		},
		{
			name:  "an entry that may be edited",
			state: sshGrammarState{screen: ScreenSSHHost, prepare: func(m *Model) { m.SSH.HostIndex = 0 }},
			want: []string{
				utilitiesScreenRow(sshEditRow),
				utilitiesScreenRow(sshDuplicateRow),
				sshDeleteRow,
				utilitiesScreenRow(sshTestThisRow),
				sshCopyPubRow,
				sshBackToHostsRow,
			},
		},
		{
			name:   "an entry that is read-only",
			state:  sshGrammarState{screen: ScreenSSHHost, prepare: func(m *Model) { m.SSH.HostIndex = 1 }},
			want:   []string{utilitiesScreenRow(sshDuplicateRow)},
			absent: []string{utilitiesScreenRow(sshEditRow), sshDeleteRow},
		},
		{
			name: "the form",
			state: sshGrammarState{screen: ScreenSSHForm, prepare: func(m *Model) {
				m.SSH.FormMode = "create"
				m.SSH.Form = sshHostForm{Alias: "staging", HostName: "staging.example.com"}
			}},
			want: []string{sshSaveRow, sshCancelRow},
		},
		{
			name: "the confirmation of a write",
			state: sshGrammarState{screen: ScreenSSHForm, prepare: func(m *Model) {
				m.SSH.Plan = &sshcfg.WritePlan{Path: "/home/testuser/.ssh/config"}
			}},
			want: []string{sshSaveRow, sshDiscardRow},
		},
		{
			name:  "the key list with the selection in the agent",
			state: sshGrammarState{screen: ScreenSSHKeys},
			want: []string{
				utilitiesScreenRow(sshGenerateKeyRow),
				sshCopyKeyRow,
				sshRemoveKeyRow,
				sshBackRow,
			},
			absent: []string{utilitiesScreenRow(sshAddKeyRow)},
		},
		{
			name:  "the key list with the selection not in the agent",
			state: sshGrammarState{screen: ScreenSSHKeys, prepare: func(m *Model) { m.Cursor = 1; m.SSH.KeyIndex = 1 }},
			want: []string{
				utilitiesScreenRow(sshAddKeyRow),
			},
			absent: []string{sshRemoveKeyRow},
		},
		{
			name: "the generation form",
			state: sshGrammarState{screen: ScreenSSHKeys, prepare: func(m *Model) {
				m.SSH.KeysMode = "generate"
				m.SSH.Keygen = sshKeygenForm{Path: "/home/testuser/.ssh/id_ed25519", Type: "ed25519"}
			}},
			want: []string{sshReplaceRowOff, sshCommandRow("g", sshGenerateKeyRow), sshCancelRow},
		},
		{
			name: "the generation form with the replace switch on",
			state: sshGrammarState{screen: ScreenSSHKeys, prepare: func(m *Model) {
				m.SSH.KeysMode = "generate"
				m.SSH.Keygen = sshKeygenForm{Path: "/home/testuser/.ssh/id_ed25519", Type: "ed25519", Replace: true}
			}},
			want: []string{sshReplaceRowOn},
		},
		{
			name:  "the add-to-agent prompt",
			state: sshGrammarState{screen: ScreenSSHKeys, prepare: func(m *Model) { m.SSH.KeysMode = "add" }},
			want:  []string{sshAddToAgentRow, sshCancelRow},
		},
		{
			name:  "the agent",
			state: sshGrammarState{screen: ScreenSSHAgent},
			want: []string{
				sshCommandRow("x", sshRemoveIdentityPrefix+"1.  SHA256:aaaa  testuser@host  (ED25519)"),
				sshRemoveEveryIdentityRow,
				sshBackRow,
			},
		},
		{
			name: "an agent holding nothing",
			state: sshGrammarState{screen: ScreenSSHAgent, prepare: func(m *Model) {
				m.SSH.Agent.Identities = nil
			}},
			absent: []string{sshRemoveEveryIdentityRow},
		},
		{
			name:   "the connection test",
			state:  sshGrammarState{screen: ScreenSSHTest},
			want:   []string{sshRunTestRow, sshBackRow},
			absent: []string{sshCancelTestRow},
		},
		{
			name: "a running connection test",
			state: sshGrammarState{screen: ScreenSSHTest, prepare: func(m *Model) {
				m.SSH.Testing = true
			}},
			want:   []string{sshCancelTestRow, sshBackRow},
			absent: []string{sshRunTestRow},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := sshGrammarModel(t, tt.state)
			rows := m.GetCurrentOptions()
			for _, want := range tt.want {
				if !containsRow(rows, want) {
					t.Errorf("the state does not offer %q: %v", want, rows)
				}
			}
			for _, absent := range tt.absent {
				if containsRow(rows, absent) {
					t.Errorf("the state offers %q: %v", absent, rows)
				}
			}
		})
	}
}

// TestSSHTextFieldsKeepTheirLetters covers the grammar's sixth rule from the
// screen that makes it hard: a screen with a field and a command letter offers
// both, and the letter is text while the cursor is on the field. The command
// waits until the cursor has left it, which is the only way one letter can be
// both a character and a command without ambiguity.
func TestSSHTextFieldsKeepTheirLetters(t *testing.T) {
	test := sshTestModel(t, ScreenSSHTest)
	test.Cursor = 0
	test.SSH.TestAlias = ""
	next, cmd := test.handleSSHTestKeys("r")
	typed := next.(Model)
	if cmd != nil || typed.SSH.Testing {
		t.Error("r ran the test from inside the alias field")
	}
	if typed.SSH.TestAlias != "r" {
		t.Errorf("the alias field holds %q, want the letter typed into it", typed.SSH.TestAlias)
	}

	form := sshTestModel(t, ScreenSSHForm)
	form.SSH.FormMode = "create"
	form.SSH.Form = sshHostForm{}
	form.Cursor = 0
	next, cmd = form.handleSSHFormKeys("w")
	typedForm := next.(Model)
	if cmd != nil || typedForm.SSH.Plan != nil {
		t.Error("w planned a write from inside the alias field")
	}
	if typedForm.SSH.Form.Alias != "w" {
		t.Errorf("the alias field holds %q, want the letter typed into it", typedForm.SSH.Form.Alias)
	}

	keygen := sshTestModel(t, ScreenSSHKeys)
	keygen.SSH.KeysMode = "generate"
	keygen.SSH.Keygen = sshKeygenForm{Path: "/home/testuser/.ssh/id_ed25519", Type: "ed25519"}
	keygen.Cursor = 2 // the comment field
	next, cmd = keygen.handleSSHKeysKeys("g")
	typedKeygen := next.(Model)
	if cmd != nil || typedKeygen.SSH.Working {
		t.Error("g generated a key from inside the comment field")
	}
	if typedKeygen.SSH.Keygen.Comment != "g" {
		t.Errorf("the comment field holds %q, want the letter typed into it", typedKeygen.SSH.Keygen.Comment)
	}
}

// TestSSHFieldStatesMoveWithTheArrowsOnly covers the other half of the field
// rule: k and j are letters on a screen whose fields own the keys, so the arrows
// are what move between them -- and the cursor still reaches the rows below the
// fields, where the list moves as every list does.
func TestSSHFieldStatesMoveWithTheArrowsOnly(t *testing.T) {
	form := sshTestModel(t, ScreenSSHForm)
	form.SSH.FormMode = "create"
	form.SSH.Form = sshHostForm{Alias: "staging"}
	form.Cursor = 0

	want := "staging"
	for _, letter := range []string{"j", "k"} {
		want += letter
		next, _ := form.handleSSHFormKeys(letter)
		landed := next.(Model)
		if landed.Cursor != 0 {
			t.Errorf("%q moved the cursor to row %d inside a field", letter, landed.Cursor)
		}
		if landed.SSH.Form.Alias != want {
			t.Errorf("%q was not typed into the field: %q, want %q", letter, landed.SSH.Form.Alias, want)
		}
		form = landed
	}

	next, _ := form.handleSSHFormKeys("down")
	moved := next.(Model)
	if moved.Cursor != 1 {
		t.Errorf("down reached row %d, want the next field", moved.Cursor)
	}
	// Down from the last field reaches the row that writes, past the separator.
	for moved.Cursor < sshFormFieldCount {
		next, _ = moved.handleSSHFormKeys("down")
		moved = next.(Model)
	}
	if row := moved.GetCurrentOptions()[moved.Cursor]; row != sshSaveRow {
		t.Errorf("down from the last field reached %q, want the row that writes", row)
	}
	// And the list below the fields still moves on k and j, the way every list
	// does: only the fields give those two letters to the text.
	for moved.Cursor < len(moved.GetCurrentOptions())-1 {
		next, _ = moved.handleSSHFormKeys("j")
		moved = next.(Model)
	}
	atEnd := moved.Cursor
	next, _ = moved.handleSSHFormKeys("j")
	if landed := next.(Model); landed.Cursor != atEnd {
		t.Errorf("j moved past the last row of the form")
	}
	next, _ = moved.handleSSHFormKeys("k")
	if landed := next.(Model); landed.Cursor != atEnd-1 {
		t.Errorf("k did not move up the rows below the fields: it reached %d, want %d", landed.Cursor, atEnd-1)
	}
}

// TestSSHKeygenSwitchIsTheOnlySpaceOnTheScreen covers the second rule: Space
// toggles the one switch, and it is offered nowhere else. Enter is not a toggle,
// and a screen with no switch does not offer Space at all.
func TestSSHKeygenSwitchIsTheOnlySpaceOnTheScreen(t *testing.T) {
	m := sshTestModel(t, ScreenSSHKeys)
	m.SSH.KeysMode = "generate"
	m.SSH.Keygen = sshKeygenForm{Path: "/home/testuser/.ssh/id_ed25519", Type: "ed25519"}
	m.Cursor = indexOfRow(t, m, sshReplaceRowOff)

	next, _ := m.handleSSHKeysKeys("enter")
	if landed := next.(Model); landed.SSH.Keygen.Replace {
		t.Error("Enter toggled the replace switch")
	}
	if !sshFooterNames(m, "Space") {
		t.Error("the footer does not name the switch key")
	}

	next, _ = m.handleSSHKeysKeys(" ")
	toggled := next.(Model)
	if !toggled.SSH.Keygen.Replace {
		t.Error("Space did not toggle the replace switch")
	}
	if !containsRow(toggled.GetCurrentOptions(), sshReplaceRowOn) {
		t.Errorf("the switch does not show its new state: %v", toggled.GetCurrentOptions())
	}

	// The screens with no switch do not offer Space, and the key does nothing on
	// their rows.
	for _, state := range []sshGrammarState{
		{screen: ScreenSSHManager},
		{screen: ScreenSSHHost, prepare: func(m *Model) { m.SSH.HostIndex = 0 }},
		{screen: ScreenSSHTest},
	} {
		m := sshGrammarModel(t, state)
		if sshFooterNames(m, "Space") {
			t.Errorf("the %v screen offers Space with no switch to toggle", state.screen)
		}
	}
}

// TestSSHKeyActionsActOnTheSelectedKey covers the rule behind the key screen's
// action rows: they are about the key the cursor stood on, so walking down onto
// one of them must not change the selection -- nor make the row itself vanish,
// which is what a selection read straight from the cursor would do.
func TestSSHKeyActionsActOnTheSelectedKey(t *testing.T) {
	m := sshTestModel(t, ScreenSSHKeys)

	next, _ := m.handleSSHKeysKeys("down") // the second key
	walked := next.(Model)
	if walked.SSH.KeyIndex != 1 {
		t.Fatalf("the selection is key %d after moving onto the second key, want 1", walked.SSH.KeyIndex)
	}
	if walked.SSH.AgentLoaded && walked.SSH.Agent.Reachable && containsRow(walked.GetCurrentOptions(), sshRemoveKeyRow) {
		t.Error("the second key is offered for removal although the agent does not hold it")
	}

	// Walk down to the add row and open it: the prompt must be about the key the
	// cursor stood on, and the row must still be there to be reached.
	addIndex := indexOfRow(t, walked, utilitiesScreenRow(sshAddKeyRow))
	for walked.Cursor < addIndex {
		next, _ = walked.handleSSHKeysKeys("down")
		walked = next.(Model)
	}
	if walked.Cursor != addIndex {
		t.Fatalf("the cursor reached row %d, want the add row %d", walked.Cursor, addIndex)
	}
	next, _ = walked.handleSSHKeysKeys("enter")
	opened := next.(Model)
	if opened.SSH.KeysMode != "add" {
		t.Fatalf("Enter on the add row left the screen in mode %q", opened.SSH.KeysMode)
	}
	if name := opened.sshSelectedKeyName(); name != "id_rsa" {
		t.Errorf("the prompt is about %q, want the key the cursor stood on", name)
	}
}
