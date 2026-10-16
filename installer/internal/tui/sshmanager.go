package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/sshcfg"
	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// THE UTILITIES SECTION: THE SSH CONFIGURATION, ITS KEYS AND ITS AGENT
// ============================================================================
//
// This is the utility that reads and manages the user's OpenSSH client setup.
// It exists because the client's own answer is spread over several files -- the
// user's config, everything it Includes, the system-wide file and the command
// line -- and the parts a developer most needs to see (which key a host really
// uses, why a host is unreachable, whether the agent is even running) are
// exactly the parts no single file states.
//
// Four rules shape every screen in this file:
//
//  1. The effective configuration is the client's answer, not this tool's.
//     sshcfg asks the client (`ssh -G`) and falls back to its own reading only
//     when the client cannot be asked, saying so on the screen when it does.
//  2. Nothing outside the block this tool owns is rewritten. An entry that
//     cannot be edited safely is shown with the reason and no edit row exists
//     for it.
//  3. A private key is never read. Every key fact on these screens comes from
//     the public half, from the agent's own listing, or from ssh-keygen.
//  4. A connection is tested only when the user presses the row that tests it,
//     under a bound, with host-key verification left exactly as the client and
//     the user's configuration have it.

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

// sshHostForm is the editable half of one host entry: the fields the client
// understands as a host's identity, and the values the write layer turns into
// the entry's directives.
type sshHostForm struct {
	Alias      string
	HostName   string
	User       string
	Port       string
	Identity   string
	ProxyJump  string
	EditTarget string // the alias being changed; empty for a new entry
}

// sshKeygenForm is the key generation form. The passphrase is typed here, kept
// only in this struct while the screen is open, and handed to ssh-keygen over a
// pipe: it is never written to a file, never logged and never an argument.
type sshKeygenForm struct {
	Path       string
	Type       string
	Comment    string
	Passphrase string
	Confirm    string
	Replace    bool
}

// sshManagerState is everything the SSH manager screens draw. Like every other
// utility's state it is filled by commands and read by renders: no screen in
// this file touches a file, a process or the environment while it draws.
type sshManagerState struct {
	// Manager is resolved once, when the section is first opened, so the client
	// and its tools are probed once rather than on every screen.
	Manager *sshcfg.Manager

	// Notice is the last result, drawn under the list of whichever screen
	// produced it.
	Notice string

	// The hosts the client would use.
	Hosts          []sshcfg.Host
	ConfigFiles    []sshcfg.ConfigFile
	ConfigFailures []string
	MatchBlocks    int
	HostsLoaded    bool
	HostsErr       string

	// The effective configuration of the entry the detail screen shows.
	Detail      sshcfg.Effective
	DetailAlias string
	DetailErr   string

	// The public keys, and what the known_hosts file holds.
	Keys          []sshcfg.Key
	KnownHosts    []sshcfg.KnownHost
	KeysLoaded    bool
	KeysErr       string
	KnownHostsErr string

	// The agent.
	Agent        sshcfg.AgentState
	AgentLoaded  bool
	AgentConfirm bool // a first press on the removal row, waiting for the second

	// The diagnosis.
	Findings          []sshcfg.Finding
	DiagnosticsLoaded bool
	DiagnosticsErr    string

	// The connection test. TestCancel is the cancel function of the test in
	// flight; TestSeq makes a stale answer from an abandoned run identifiable,
	// and TestOrigin is the screen the way back leads to.
	Test       sshcfg.ConnectResult
	TestAlias  string
	Testing    bool
	Tested     bool
	TestCancel context.CancelFunc
	TestSeq    int
	TestOrigin Screen

	// HostIndex and KeyIndex are the entry and the key the detail and the actions
	// are about. They are separate from Cursor because a screen's cursor is the
	// row it is on: the host order in the list is not the action order on the
	// detail screen, and the key an action was started for is not drawn at all in
	// the two sub-states the key screen opens.
	HostIndex int
	KeyIndex  int

	// The write flow: the form, the plan the form turned into, and the plan's
	// confirmation.
	Form     sshHostForm
	FormMode string // "" (no form), "create", "edit", "duplicate"
	Plan     *sshcfg.WritePlan
	Writing  bool

	// The key screen's two sub-states: the generation form and the passphrase
	// prompt that adding a key to the agent needs.
	KeysMode      string // "", "generate", "add"
	Keygen        sshKeygenForm
	KeygenFocus   int
	AddPassphrase string
	Working       bool
}

// sshTypingActive reports whether the screen in front of the user has a field
// that owns the text keys. The global handlers ask this before they take space
// for the leader key or backspace for "back": on a form those two keys are text,
// and taking them would make the field unusable.
func (m Model) sshTypingActive() bool {
	switch m.Screen {
	case ScreenSSHForm:
		return m.SSH.Plan == nil
	case ScreenSSHTest:
		return !m.SSH.Testing
	case ScreenSSHKeys:
		return m.SSH.KeysMode == "generate" || m.SSH.KeysMode == "add"
	}
	return false
}

// sshHasForm reports whether the form screen is editing anything at all: it is
// what the row builders and the handler agree on, so a screen that is open
// without a form cannot draw rows that do nothing.
func (m Model) sshHasForm() bool {
	return m.Screen != ScreenSSHForm || m.SSH.FormMode != ""
}

// ---------------------------------------------------------------------------
// Rows
// ---------------------------------------------------------------------------

// The rows of the manager's screens. Each one is a constant the handler matches
// against, so the row that is drawn and the row that acts are one string.
const (
	sshAddHostRow  = "Add a host this tool manages"
	sshKeysRow     = "See the public keys"
	sshAgentRow    = "See the SSH agent"
	sshDiagnoseRow = "Diagnose the configuration"
	sshTestRow     = "Test a connection"
	sshBackRow     = "← Back"

	sshBackToHostsRow = "← Back to the hosts"
	sshEditRow        = "Edit this entry"
	sshDuplicateRow   = "Duplicate this entry"
	sshDeleteRow      = "Delete this entry"
	sshTestThisRow    = "Test this connection"
	sshCopyPubRow     = "Copy the public key"
	sshShowPubRow     = "Show the public key"

	sshSaveRow    = "Write the entry"
	sshDiscardRow = "Leave the file alone"
	sshCancelRow  = "Cancel"

	sshGenerateKeyRow = "Generate the key"
	// The replace switch is two rows rather than one, because the state is what the
	// user is deciding about: a row that says only "Replace an existing pair" leaves
	// the current answer invisible at the moment of choosing it.
	sshReplaceRowPrefix = "Replace an existing pair: "
	sshReplaceRowOn     = sshReplaceRowPrefix + "on"
	sshReplaceRowOff    = sshReplaceRowPrefix + "off"
	sshCopyKeyRow       = "Copy the selected public key"
	sshAddKeyRow        = "Add the selected key to the agent"
	sshRemoveKeyRow     = "Remove the selected key from the agent"
	sshAddToAgentRow    = "Add it to the agent"
	sshKnownHostsInfo   = "known hosts"

	sshRemoveEveryIdentityRow = "Remove every identity from the agent"
	sshRemoveIdentityPrefix   = "Remove the identity "

	sshRunTestRow    = "Run the test"
	sshCancelTestRow = "Cancel the test"
	sshTestTargetRow = "Host or alias"

	// The form's field rows. The value follows the label, so the row reads as
	// the field it edits rather than as a list of names.
	sshAliasField     = "Alias"
	sshHostNameField  = "Host name"
	sshUserField      = "User"
	sshPortField      = "Port"
	sshIdentityField  = "Identity file"
	sshProxyJumpField = "Jump host"
	sshPathField      = "File name"
	sshTypeField      = "Type"
	sshCommentField   = "Comment"
	sshPassField      = "Passphrase"
	sshPassAgainField = "Passphrase again"
)

// sshFieldRow renders one labelled field with its current value. An empty value
// says so in words rather than as trailing blank space, which is what makes an
// untouched field distinguishable from one holding spaces.
func sshFieldRow(label, value string) string {
	if value == "" {
		return label + ": (empty)"
	}
	return label + ": " + value
}

// sshHostRow is one host as the manager's list draws it: the alias the client
// matches on, then the declared identity beside it, and a marker when the entry
// is the tool's own or when it may not be edited. The marker is a word, never a
// colour: a 16-colour terminal loses nothing.
func sshHostRow(h sshcfg.Host) string {
	alias := h.Alias
	if alias == "" {
		alias = strings.Join(h.Patterns, " ")
	}
	if alias == "" {
		alias = "(no name)"
	}
	parts := []string{alias}
	if identity := sshDeclaredIdentity(h); identity != "" {
		parts = append(parts, identity)
	}
	if h.Managed {
		parts = append(parts, "[managed]")
	} else if h.ReadOnly != "" {
		parts = append(parts, "[read-only]")
	}
	return strings.Join(parts, "  ")
}

// sshDeclaredIdentity is the `user@hostname:port` the entry itself declares, as
// far as the entry declares it. It is deliberately the declared half and not the
// effective one: the list is about what the file says, and the detail screen is
// where the client's own answer is shown beside it.
func sshDeclaredIdentity(h sshcfg.Host) string {
	hostname := sshOptionValue(h, "hostname")
	user := sshOptionValue(h, "user")
	port := sshOptionValue(h, "port")
	if hostname == "" && user == "" && port == "" {
		return ""
	}
	name := hostname
	if name == "" {
		name = h.Alias
	}
	if user != "" {
		name = user + "@" + name
	}
	if port != "" {
		name += ":" + port
	}
	return name
}

// sshOptionValue is the value of one directive of a stanza, compared without
// regard to case because the client's own keywords are case-insensitive.
func sshOptionValue(h sshcfg.Host, key string) string {
	for _, opt := range h.Options {
		if strings.EqualFold(opt.Key, key) {
			return opt.Value
		}
	}
	return ""
}

// sshManagerRows is the section's own screen: one row per host the client would
// use, then the four jobs, then the way back.
func (m Model) sshManagerRows() []string {
	rows := make([]string, 0, len(m.SSH.Hosts)+7)
	for _, host := range m.SSH.Hosts {
		rows = append(rows, sshHostRow(host))
	}
	if len(rows) > 0 {
		rows = append(rows, m.menuSeparator())
	}
	rows = append(rows, sshAddHostRow, sshKeysRow, sshAgentRow, sshDiagnoseRow, sshTestRow)
	rows = append(rows, m.menuSeparator(), sshBackRow)
	return rows
}

// sshHostRows is one entry's detail screen. The actions are offered only when
// they can be carried out: an entry that may not be edited has no edit row, and
// an entry whose identity has no public half has nothing to copy.
func (m Model) sshHostRows() []string {
	var rows []string
	host, ok := m.sshSelectedHost()
	if ok {
		if host.ReadOnly == "" {
			rows = append(rows, sshEditRow)
		}
		rows = append(rows, sshDuplicateRow)
		if host.ReadOnly == "" {
			rows = append(rows, sshDeleteRow)
		}
	}
	rows = append(rows, sshTestThisRow)
	if m.sshDetailPublicKey() != "" {
		rows = append(rows, sshCopyPubRow)
	}
	rows = append(rows, m.menuSeparator(), sshBackToHostsRow)
	return rows
}

// sshFormRows is the add/edit form, or the plan's confirmation once a plan
// exists. The two states are one list because they are one screen: the user
// never leaves the form to confirm what it would write.
func (m Model) sshFormRows() []string {
	if m.SSH.Plan != nil {
		return []string{sshSaveRow, sshDiscardRow}
	}
	form := m.SSH.Form
	rows := []string{
		sshFieldRow(sshAliasField, form.Alias),
		sshFieldRow(sshHostNameField, form.HostName),
		sshFieldRow(sshUserField, form.User),
		sshFieldRow(sshPortField, form.Port),
		sshFieldRow(sshIdentityField, form.Identity),
		sshFieldRow(sshProxyJumpField, form.ProxyJump),
		m.menuSeparator(),
	}
	if m.sshFormReady() {
		rows = append(rows, sshSaveRow)
	}
	return append(rows, sshCancelRow)
}

// sshKeyRows is the key screen in its three states: the inventory with its
// actions, the generation form, and the passphrase prompt that adding a key to
// the agent needs.
func (m Model) sshKeyRows() []string {
	switch m.SSH.KeysMode {
	case "generate":
		form := m.SSH.Keygen
		rows := []string{
			sshFieldRow(sshPathField, form.Path),
			sshFieldRow(sshTypeField, form.Type),
			sshFieldRow(sshCommentField, form.Comment),
			sshFieldRow(sshPassField, sshMask(form.Passphrase)),
			sshFieldRow(sshPassAgainField, sshMask(form.Confirm)),
			m.menuSeparator(),
		}
		// The switch is offered whether or not the form is ready: it is one of the
		// answers, and hiding it until the passphrases match would leave the user
		// unable to say what to do about an existing pair.
		rows = append(rows, sshReplaceKeygenRow(form.Replace))
		if m.sshKeygenReady() {
			rows = append(rows, sshGenerateKeyRow)
		}
		return append(rows, sshCancelRow)
	case "add":
		return []string{
			sshFieldRow(sshPassField, sshMask(m.SSH.AddPassphrase)),
			m.menuSeparator(),
			sshAddToAgentRow,
			sshCancelRow,
		}
	}

	rows := make([]string, 0, len(m.SSH.Keys)+8)
	for _, key := range m.SSH.Keys {
		rows = append(rows, sshKeyRow(key))
	}
	if len(rows) > 0 {
		rows = append(rows, m.menuSeparator())
	}
	rows = append(rows, sshGenerateKeyRow)
	if _, ok := m.sshSelectedKey(); ok {
		rows = append(rows, sshCopyKeyRow)
		if m.SSH.AgentLoaded && m.SSH.Agent.Reachable {
			if m.sshSelectedKeyInAgent() {
				rows = append(rows, sshRemoveKeyRow)
			} else {
				rows = append(rows, sshAddKeyRow)
			}
		}
	}
	rows = append(rows, m.menuSeparator(), sshBackRow)
	return rows
}

// sshKeyRow is one key as the list draws it: the file, what it is, its
// fingerprint and whether the agent holds it. The private half's mode is named
// only when it is wrong, because a row that says "0600" on every key spends the
// width that the fingerprint needs.
func sshKeyRow(key sshcfg.Key) string {
	name := filepath.Base(key.Path)
	parts := []string{name}
	kind := sshKeyKind(key)
	if kind != "" {
		parts = append(parts, kind)
	}
	if key.Fingerprint != "" {
		parts = append(parts, key.Fingerprint)
	}
	if key.PublicKnown && !key.PermissionsOK && key.Mode != 0 {
		parts = append(parts, fmt.Sprintf("mode %04o", key.Mode.Perm()))
	}
	if key.PublicKnown && key.PubPath == "" && key.Path != "" {
		parts = append(parts, "no public half")
	}
	return strings.Join(parts, "  ")
}

// sshKeyKind names a key's type and size in the words a person reads.
func sshKeyKind(key sshcfg.Key) string {
	if key.Type == "" {
		return ""
	}
	name := strings.TrimPrefix(key.Type, "ssh-")
	name = strings.TrimSuffix(name, "-cert-v01@openssh.com")
	if key.Bits > 0 {
		return fmt.Sprintf("%s %d bits", name, key.Bits)
	}
	return name
}

// sshAgentRows is the agent screen: one row per identity the agent holds, then
// the one action that removes everything. Identities are removable one by one
// because that is the only removal ssh-add offers without a fingerprint lookup.
func (m Model) sshAgentRows() []string {
	rows := make([]string, 0, len(m.SSH.Agent.Identities)+2)
	for i, id := range m.SSH.Agent.Identities {
		rows = append(rows, sshRemoveIdentityPrefix+sshAgentIdentityLabel(i, id))
	}
	if len(rows) > 0 {
		rows = append(rows, m.menuSeparator())
		if m.SSH.Agent.CanRemoveAll {
			rows = append(rows, sshRemoveEveryIdentityRow)
			rows = append(rows, m.menuSeparator())
		}
	}
	return append(rows, sshBackRow)
}

// sshAgentIdentityLabel is one agent identity, in the shape `ssh-add -l` prints
// it plus the index the removal needs to point at the same entry.
func sshAgentIdentityLabel(index int, id sshcfg.AgentIdentity) string {
	parts := []string{fmt.Sprintf("%d.", index+1)}
	if id.Fingerprint != "" {
		parts = append(parts, id.Fingerprint)
	}
	if id.Comment != "" {
		parts = append(parts, id.Comment)
	}
	if id.Type != "" {
		parts = append(parts, "("+strings.TrimPrefix(id.Type, "ssh-")+")")
	}
	return strings.Join(parts, "  ")
}

// sshDiagnosticRows is the diagnosis: one row per finding with its severity as a
// word, then the way back. The selected finding's detail and fix are in the
// screen's own body.
func (m Model) sshDiagnosticRows() []string {
	rows := make([]string, 0, len(m.SSH.Findings)+1)
	for _, f := range m.SSH.Findings {
		rows = append(rows, sshFindingRow(f))
	}
	return append(rows, sshBackRow)
}

// sshFindingRow is one finding as a row: the severity as a word, then where it
// is, then what it is about.
func sshFindingRow(f sshcfg.Finding) string {
	where := f.File
	if where != "" {
		where = filepath.Base(where)
	}
	if f.Line > 0 {
		where = fmt.Sprintf("%s:%d", where, f.Line)
	}
	parts := []string{"[" + f.Severity.String() + "]"}
	if where != "" {
		parts = append(parts, where)
	}
	parts = append(parts, f.Title)
	return strings.Join(parts, "  ")
}

// sshTestRows is the test screen: the alias to test, the press that runs it, and
// the press that stops it while it runs.
func (m Model) sshTestRows() []string {
	rows := []string{sshFieldRow(sshTestTargetRow, m.SSH.TestAlias)}
	if m.SSH.Testing {
		return append(rows, m.menuSeparator(), sshCancelTestRow, sshBackRow)
	}
	rows = append(rows, m.menuSeparator())
	if strings.TrimSpace(m.SSH.TestAlias) != "" && m.sshCanTest() {
		rows = append(rows, sshRunTestRow)
	}
	return append(rows, sshBackRow)
}

// ---------------------------------------------------------------------------
// Bodies
// ---------------------------------------------------------------------------

// sshManagerDescription is what the section says about the machine it is looking
// at: how many hosts the client would use, where they come from, which of them
// this tool owns, and what the client and its tools are. Every fact is stated
// with its source, because a report that hides where a number came from is a
// report nobody can check.
func (m Model) sshManagerDescription() []string {
	if m.SSH.Manager == nil {
		return []string{"Reading the SSH client configuration: the client's own answer, the files it reads and the keys on this machine."}
	}
	mgr := m.SSH.Manager
	var paragraphs []string

	if m.SSH.HostsErr != "" {
		paragraphs = append(paragraphs, "The configuration could not be read: "+m.SSH.HostsErr)
	} else {
		managed := 0
		readOnly := 0
		for _, h := range m.SSH.Hosts {
			if h.Managed {
				managed++
			}
			if h.ReadOnly != "" {
				readOnly++
			}
		}
		files := sshReadFiles(m.SSH.ConfigFiles)
		paragraphs = append(paragraphs, fmt.Sprintf(
			"The client reads %s. %s; %d of them are managed by this tool, and %d more are shown read-only because editing them would mean rewriting somebody else's file.",
			files, sshHostCountLabel(len(m.SSH.Hosts)), managed, readOnly))
		if m.SSH.MatchBlocks > 0 {
			paragraphs = append(paragraphs, fmt.Sprintf(
				"%d Match %s are evaluated by the client with criteria this report cannot fully reproduce; the entries inside them are shown as they are declared.",
				m.SSH.MatchBlocks, sshPlural(m.SSH.MatchBlocks, "block", "blocks")))
		}
		for _, failure := range m.SSH.ConfigFailures {
			paragraphs = append(paragraphs, "Reading the configuration: "+failure)
		}
	}

	paragraphs = append(paragraphs, sshToolSummary(mgr))
	paragraphs = append(paragraphs, fmt.Sprintf(
		"This tool writes only inside its own block in %s, after a backup beside the file and a validation "+
			"with the client's own parser; every line outside that block is left byte-for-byte as it is.",
		sshManagerPath(mgr)))
	return paragraphs
}

// sshReadFiles names the files the client would read, in the order it reads
// them, and says when one of them does not exist.
func sshReadFiles(files []sshcfg.ConfigFile) string {
	if len(files) == 0 {
		return "no configuration file at all"
	}
	names := make([]string, 0, len(files))
	for _, file := range files {
		name := file.Path
		if file.Missing {
			name += " (missing)"
		}
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// sshToolSummary says what this host offers: the client, key generation and the
// agent. A missing tool is named as missing rather than left for the user to
// discover when a row refuses.
func sshToolSummary(mgr *sshcfg.Manager) string {
	var parts []string
	if mgr.Caps.HasSSH() {
		version := mgr.Caps.SSHVersion
		if version == "" {
			version = "version unavailable"
		}
		parts = append(parts, "ssh: "+version)
	} else {
		parts = append(parts, "ssh: not found on PATH")
	}
	if mgr.Caps.HasKeygen() {
		parts = append(parts, "ssh-keygen: found")
	} else {
		parts = append(parts, "ssh-keygen: not found, so keys cannot be generated or fingerprinted here")
	}
	if mgr.Caps.AgentSocket == "" {
		parts = append(parts, "SSH agent: no agent is advertised in this environment")
	} else if mgr.Caps.HasAgentControl() {
		parts = append(parts, "SSH agent: an agent is advertised and ssh-add is available")
	} else {
		parts = append(parts, "SSH agent: an agent is advertised but ssh-add was not found, so identities cannot be changed here")
	}
	return strings.Join(parts, "; ") + "."
}

// sshHostCountLabel names how many hosts were found, in the singular when there
// is one, because "1 hosts" reads as a count nobody looked at.
func sshHostCountLabel(n int) string {
	return fmt.Sprintf("%d %s", n, sshPlural(n, "host was found", "hosts were found"))
}

// sshPlural picks the singular or the plural word.
func sshPlural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// sshHostDescription is one entry's detail: the client's effective answer, where
// each value came from, what the entry declares, and every caveat -- the reason
// it cannot be edited, the criteria that were not evaluated, and the client's own
// refusal to answer.
func (m Model) sshHostDescription() []string {
	host, ok := m.sshSelectedHost()
	if !ok {
		return []string{"No entry is selected. Go back to the list and choose one."}
	}
	var paragraphs []string

	declared := fmt.Sprintf("Declared in %s:%d", sshShortPath(m.SSH.Manager, host.File), host.Line)
	if host.Managed {
		declared += ", inside the block this tool manages"
	}
	paragraphs = append(paragraphs, declared+".")

	if host.ReadOnly != "" {
		paragraphs = append(paragraphs, "Shown read-only: "+host.ReadOnly)
	}

	eff := m.SSH.Detail
	if m.SSH.DetailErr != "" {
		paragraphs = append(paragraphs, "The client could not be asked about this entry: "+m.SSH.DetailErr)
	} else {
		paragraphs = append(paragraphs, sshEffectiveRows(eff)...)
	}
	for _, warning := range eff.Warnings {
		paragraphs = append(paragraphs, "Note: "+warning)
	}

	if len(host.Options) > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf(
			"The entry itself declares %d %s: %s.", len(host.Options),
			sshPlural(len(host.Options), "option", "options"), sshOptionList(host.Options, 6)))
	}
	if pub := m.sshDetailPublicKey(); pub != "" {
		paragraphs = append(paragraphs, "Public key ("+sshFirstIdentity(m.SSH.Manager, eff)+"):", pub)
	}
	return paragraphs
}

// sshEffectiveRows renders the effective values with their source. A value the
// client answered is named as the client's answer; one this package resolved is
// named with its file and line, which is the part the client does not report.
func sshEffectiveRows(eff sshcfg.Effective) []string {
	rows := []string{}
	add := func(label, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		rows = append(rows, fmt.Sprintf("%s: %s%s", label, value, sshSourceSuffix(eff, label)))
	}
	add("Host name", eff.HostName)
	add("User", eff.User)
	add("Port", eff.Port)
	if len(eff.IdentityFiles) > 0 {
		add("Identity file", strings.Join(eff.IdentityFiles, ", "))
	}
	if eff.ProxyJump != "" {
		add("Jump host", eff.ProxyJump)
	}
	if len(rows) == 0 {
		rows = append(rows, "The client's answer named no host name, user, port or identity for this entry.")
	}
	return rows
}

// sshSourceSuffix is where a value came from, in words: the client's own answer,
// or the file and line that declared it.
func sshSourceSuffix(eff sshcfg.Effective, label string) string {
	key := strings.ToLower(strings.ReplaceAll(label, " ", ""))
	source, ok := eff.Sources[key]
	if !ok {
		if eff.FromClient {
			return "  (the client's answer)"
		}
		return ""
	}
	if source.File == "" {
		return ""
	}
	return fmt.Sprintf("  (%s:%d)", sshShortPath(nil, source.File), source.Line)
}

// sshOptionList names the options of a stanza, cut to a bound with a count of
// what was left out: a wall of unknown options would spend the whole frame.
func sshOptionList(options []sshcfg.Option, limit int) string {
	names := make([]string, 0, len(options))
	for _, opt := range options {
		names = append(names, opt.Key)
	}
	if len(names) > limit {
		hidden := len(names) - limit
		names = append(names[:limit], fmt.Sprintf("and %d more", hidden))
	}
	return strings.Join(names, ", ")
}

// sshShortPath shortens a path for a row: the home directory becomes ~, which is
// what the user would type.
func sshShortPath(mgr *sshcfg.Manager, path string) string {
	if mgr != nil && mgr.Paths.Home != "" && strings.HasPrefix(path, mgr.Paths.Home) {
		return "~" + strings.TrimPrefix(path, mgr.Paths.Home)
	}
	return path
}

// sshFirstIdentity is the identity path of the effective configuration, or the
// declared one when the client was not asked.
func sshFirstIdentity(mgr *sshcfg.Manager, eff sshcfg.Effective) string {
	if len(eff.IdentityFiles) == 0 {
		return "no identity is configured for this entry"
	}
	return sshShortPath(mgr, eff.IdentityFiles[0])
}

// sshFormDescription is what the form says about itself: which file the entry
// will go to, and every reason the entry could not be written as it stands. The
// reasons are the write layer's own validation, so the screen and the writer
// cannot disagree about what is acceptable.
func (m Model) sshFormDescription() []string {
	if m.SSH.Plan != nil {
		return sshPlanDescription(m.SSH.Plan)
	}
	var paragraphs []string
	switch m.SSH.FormMode {
	case "create":
		paragraphs = append(paragraphs, fmt.Sprintf(
			"A new entry is written inside the block this tool manages in %s. Everything outside that block is left byte-for-byte as it is.",
			sshManagerPath(m.SSH.Manager)))
	case "edit":
		paragraphs = append(paragraphs, fmt.Sprintf(
			"Editing %s in place: only the lines whose value changes are rewritten, and the entry's other options and comments stay where they are.",
			m.SSH.Form.EditTarget))
	case "duplicate":
		paragraphs = append(paragraphs, fmt.Sprintf(
			"Duplicating %s: the copy is written as a new entry this tool manages, and the original is left alone.",
			m.SSH.Form.EditTarget))
	}
	for _, reason := range m.sshFormRefusals() {
		paragraphs = append(paragraphs, reason)
	}
	return paragraphs
}

// sshPlanDescription is the confirmation: what the plan would change, where, and
// the backup it would take first. It is drawn from the plan itself, so the
// preview and the write cannot describe different changes.
func sshPlanDescription(plan *sshcfg.WritePlan) []string {
	paragraphs := []string{plan.Summary}
	paragraphs = append(paragraphs, fmt.Sprintf("File: %s", plan.Path))
	if plan.Backup != "" {
		paragraphs = append(paragraphs, fmt.Sprintf("A backup is written first, beside the file: %s", plan.Backup))
	} else {
		paragraphs = append(paragraphs, "There is nothing to back up: this file does not exist yet.")
	}
	paragraphs = append(paragraphs, "The change, in lines:")
	paragraphs = append(paragraphs, plan.Preview...)
	return paragraphs
}

// sshKeyDescription is what the key screen says: the directory the keys live in,
// the selected key's own facts and warnings, the public half in full, and what
// the known_hosts file holds. The public half is shown rather than copied
// silently: the screen is the copy that works on a terminal with no clipboard.
func (m Model) sshKeyDescription() []string {
	switch m.SSH.KeysMode {
	case "generate":
		return []string{
			"That a key is generated with ssh-keygen using the algorithms this client supports. The passphrase is typed here, handed to ssh-keygen over a pipe, and never written down: it is not stored, logged or put in an argument list.",
			"The private half is written 0600 and its public half 0644, and an existing pair is replaced only when the switch below reads on.",
		}
	case "add":
		return []string{
			fmt.Sprintf("Adding %s to the agent. Type the key's passphrase, or leave it empty when the key has none, and press the row that adds it.", m.sshSelectedKeyName()),
			"The passphrase is handed to ssh-add over a pipe and is never stored, logged or put in an argument list.",
		}
	}

	var paragraphs []string
	if m.SSH.KeysErr != "" {
		paragraphs = append(paragraphs, "The keys could not be listed: "+m.SSH.KeysErr)
	} else {
		paragraphs = append(paragraphs, fmt.Sprintf(
			"%d %s and %d %s in %s. A private key is never read by this screen: every fact comes from the public half, from the agent's own listing, or from ssh-keygen.",
			len(m.SSH.Keys), sshPlural(len(m.SSH.Keys), "key pair", "key pairs"),
			len(m.SSH.KnownHosts), sshPlural(len(m.SSH.KnownHosts), "known host", "known hosts"),
			sshManagerDir(m.SSH.Manager)))
	}

	if key, ok := m.sshSelectedKey(); ok {
		if key.Fingerprint != "" {
			paragraphs = append(paragraphs, fmt.Sprintf("%s: %s, %s.", filepath.Base(key.Path), key.Fingerprint, sshKeyKind(key)))
		}
		if key.Encrypted == "yes" {
			paragraphs = append(paragraphs, "The private half is encrypted with a passphrase; the agent needs it before the key can be used.")
		}
		for _, warning := range key.Warnings {
			paragraphs = append(paragraphs, "Note: "+warning)
		}
		if key.Public != "" {
			paragraphs = append(paragraphs, "Public key:", key.Public)
		}
	}

	if m.SSH.KnownHostsErr != "" {
		paragraphs = append(paragraphs, "The known hosts could not be read: "+m.SSH.KnownHostsErr)
	} else if len(m.SSH.KnownHosts) > 0 {
		paragraphs = append(paragraphs, "Known hosts: "+sshKnownHostSummary(m.SSH.KnownHosts))
	}
	return paragraphs
}

// sshKnownHostSummary names what the known_hosts file holds: the spelled-out
// hosts, and how many entries are hashed and therefore unreadable by design.
func sshKnownHostSummary(known []sshcfg.KnownHost) string {
	var named []string
	hashed := 0
	for _, entry := range known {
		if entry.Hashed || len(entry.Hosts) == 0 {
			hashed++
			continue
		}
		label := strings.Join(entry.Hosts, ",")
		if len(entry.Markers) > 0 {
			label = strings.Join(entry.Markers, ",") + " " + label
		}
		named = append(named, label)
	}
	sort.Strings(named)
	if len(named) > 8 {
		hidden := len(named) - 8
		named = append(named[:8], fmt.Sprintf("and %d more", hidden))
	}
	summary := strings.Join(named, ", ")
	if hashed > 0 {
		if summary != "" {
			summary += "; "
		}
		summary += fmt.Sprintf("%d hashed %s the client can only match, never print", hashed, sshPlural(hashed, "entry", "entries"))
	}
	if summary == "" {
		summary = "only hashed entries"
	}
	return summary
}

// sshAgentDescription is what the agent screen says: which agent it is, how the
// listing went, and what this particular agent is known not to support. The
// differences are stated because agents are not interchangeable and promising
// they were would be the bug.
func (m Model) sshAgentDescription() []string {
	agent := m.SSH.Agent
	var paragraphs []string

	if !m.SSH.AgentLoaded {
		return []string{"Asking the agent for the identities it holds."}
	}
	if !agent.Advertised {
		return []string{"No agent is advertised here: SSH_AUTH_SOCK is unset, so no process in this environment can reach one. A key can still be used straight from a file."}
	}
	paragraphs = append(paragraphs, fmt.Sprintf("Agent: %s at %s.", agent.Kind, agent.Socket))
	if !agent.Reachable {
		reason := agent.Reason
		if reason == "" {
			reason = "the listing was refused without a message"
		}
		paragraphs = append(paragraphs, "The agent could not be asked for its identities: "+reason)
	} else if len(agent.Identities) == 0 {
		paragraphs = append(paragraphs, "The agent is running and holds no identities: nothing would authenticate through it until a key is added.")
	} else {
		paragraphs = append(paragraphs, fmt.Sprintf("It holds %d %s, listed below with their fingerprints.",
			len(agent.Identities), sshPlural(len(agent.Identities), "identity", "identities")))
	}
	for _, note := range agent.Notes {
		paragraphs = append(paragraphs, note)
	}
	if !agent.CanRemoveAll && agent.Reachable {
		paragraphs = append(paragraphs, "Removing identities needs ssh-add, which was not found on PATH.")
	}
	return paragraphs
}

// sshDiagnosticDescription is the diagnosis screen: how many findings of each
// severity, and the selected finding's own detail and fix.
func (m Model) sshDiagnosticDescription() []string {
	if !m.SSH.DiagnosticsLoaded {
		return []string{"Reading the configuration files, the permissions on ~/.ssh and the client's own parse of its configuration."}
	}
	if m.SSH.DiagnosticsErr != "" {
		return []string{"The diagnosis could not run: " + m.SSH.DiagnosticsErr}
	}
	problems, warnings, infos := sshFindingCounts(m.SSH.Findings)
	var paragraphs []string
	if len(m.SSH.Findings) == 0 {
		paragraphs = append(paragraphs, "Nothing to report: the permissions, the identity paths and the configuration's syntax all check out.")
	} else {
		paragraphs = append(paragraphs, fmt.Sprintf("%d %s to look at: %d %s, %d %s and %d %s.",
			len(m.SSH.Findings), sshPlural(len(m.SSH.Findings), "finding", "findings"),
			problems, sshPlural(problems, "problem", "problems"),
			warnings, sshPlural(warnings, "warning", "warnings"),
			infos, sshPlural(infos, "note", "notes")))
	}
	if f, ok := m.sshSelectedFinding(); ok {
		paragraphs = append(paragraphs, f.Title+". "+f.Detail)
		if f.Fix != "" {
			paragraphs = append(paragraphs, "Fix: "+f.Fix)
		} else {
			paragraphs = append(paragraphs, "This one is a decision rather than a command: this tool reports it and changes nothing.")
		}
		if f.File != "" {
			location := f.File
			if f.Line > 0 {
				location = fmt.Sprintf("%s:%d", f.File, f.Line)
			}
			paragraphs = append(paragraphs, "Found at "+location+".")
		}
	}
	return paragraphs
}

// sshFindingCounts counts the report by severity, which is the first thing a
// reader wants to know about a list of findings.
func sshFindingCounts(findings []sshcfg.Finding) (problems, warnings, infos int) {
	for _, f := range findings {
		switch f.Severity {
		case sshcfg.SeverityProblem:
			problems++
		case sshcfg.SeverityWarning:
			warnings++
		default:
			infos++
		}
	}
	return problems, warnings, infos
}

// sshTestDescription is the test screen: the exact command that would run, the
// bound it runs under, and then the result with its own explanation. A test is
// offered only where the client exists, and the screen says so when it does not.
func (m Model) sshTestDescription() []string {
	mgr := m.SSH.Manager
	var paragraphs []string

	if mgr == nil || !mgr.Caps.HasSSH() {
		return []string{"No connection can be tested here: the ssh client was not found on PATH, and this tool tests connections by asking the client to open one."}
	}
	alias := strings.TrimSpace(m.SSH.TestAlias)
	if alias == "" {
		paragraphs = append(paragraphs, "Type a host or an alias from the list, then press the row that runs the test.")
	} else {
		paragraphs = append(paragraphs, fmt.Sprintf(
			"The test asks ssh to open a session to %s and run `true` there, under a %s bound, with the configuration exactly as the client would use it.",
			alias, sshTestTimeout))
	}
	// The rule is stated before the press and in both states of the field: it is the
	// promise that makes the row safe to press, and it would be wrong to state it
	// only while the field happened to be empty.
	paragraphs = append(paragraphs, "Nothing is sent anywhere until that press: no connection is opened while you read this.")
	paragraphs = append(paragraphs, "Host keys are verified: an unknown or changed key fails the test, nothing is added to known_hosts, and no password is ever prompted for.")

	if m.SSH.Testing {
		paragraphs = append(paragraphs, fmt.Sprintf("Running the test against %s…", m.SSH.Test.Alias))
		return paragraphs
	}
	if !m.SSH.Tested {
		return paragraphs
	}

	result := m.SSH.Test
	paragraphs = append(paragraphs, fmt.Sprintf("%s in %s.", sshTestOutcomeWord(result.Outcome), result.Duration.Round(time.Millisecond)))
	paragraphs = append(paragraphs, result.Explanation)
	if len(result.Args) > 0 {
		paragraphs = append(paragraphs, "Command: "+strings.Join(result.Args, " "))
	}
	for i, line := range sshBoundedLines(result.Stderr, sshTestOutputRows) {
		if i == 0 {
			paragraphs = append(paragraphs, "The client said:")
		}
		paragraphs = append(paragraphs, line)
	}
	for i, line := range sshBoundedLines(result.Stdout, sshTestOutputRows) {
		if i == 0 {
			paragraphs = append(paragraphs, "The host answered:")
		}
		paragraphs = append(paragraphs, line)
	}
	return paragraphs
}

// sshTestOutcomeWord is the plain word for an outcome, so the reader does not
// have to decode a machine value.
func sshTestOutcomeWord(outcome sshcfg.ConnectOutcome) string {
	switch outcome {
	case sshcfg.OutcomeOK:
		return "The connection opened"
	case sshcfg.OutcomeTimeout:
		return "The connection timed out"
	case sshcfg.OutcomeCancelled:
		return "The test was cancelled"
	case sshcfg.OutcomeHostKeyUnknown:
		return "The host is unknown"
	case sshcfg.OutcomeHostKeyChanged:
		return "The host key changed"
	case sshcfg.OutcomeAuth:
		return "The host refused the authentication"
	case sshcfg.OutcomeDNS:
		return "The host name did not resolve"
	case sshcfg.OutcomeRefused:
		return "The host refused the connection"
	case sshcfg.OutcomeUnreachable:
		return "The host could not be reached"
	case sshcfg.OutcomeNoClient:
		return "No client was available"
	default:
		return "The test failed"
	}
}

// sshBoundedLines splits a tool's output into at most rows lines, marking the
// cut. A wall of client output is evidence, but it is not the screen.
func sshBoundedLines(text string, rows int) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > rows {
		hidden := len(lines) - rows + 1
		lines = lines[:rows-1]
		lines = append(lines, fmt.Sprintf("… and %d more", hidden))
	}
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, "\r")
	}
	return lines
}

// ---------------------------------------------------------------------------
// Renderers
// ---------------------------------------------------------------------------

// The SSH manager's screens are the same shape as the other utilities screens:
// a title, a description, a list and a notice, sized by the shared budget
// arithmetic in renderThemeScreen. They use it rather than their own renderer so
// the frame's rows, the companion and the notice slot behave identically here.

func (m Model) renderSSHManager() string {
	return m.renderThemeScreen(m.sshManagerDescription(), m.SSH.Notice,
		[]installerHint{hintUp, hintDown, hintSelect, hintBack})
}

func (m Model) renderSSHHost() string {
	return m.renderThemeScreen(m.sshHostDescription(), m.SSH.Notice,
		[]installerHint{hintUp, hintDown, hintSelect, hintBack})
}

func (m Model) renderSSHForm() string {
	hints := []installerHint{hintUp, hintDown, hintSelect, hintCancel}
	if m.SSH.Plan == nil {
		hints = []installerHint{hintMove, hintType, hintSelect, hintCancel}
	}
	return m.renderThemeScreen(m.sshFormDescription(), m.SSH.Notice, hints)
}

func (m Model) renderSSHKeys() string {
	hints := []installerHint{hintUp, hintDown, hintSelect, hintBack}
	if m.sshTypingActive() {
		hints = []installerHint{hintMove, hintType, hintSelect, hintCancel}
	}
	return m.renderThemeScreen(m.sshKeyDescription(), m.SSH.Notice, hints)
}

func (m Model) renderSSHAgent() string {
	return m.renderThemeScreen(m.sshAgentDescription(), m.SSH.Notice,
		[]installerHint{hintUp, hintDown, hintSelect, hintBack})
}

func (m Model) renderSSHDiagnostics() string {
	return m.renderThemeScreen(m.sshDiagnosticDescription(), m.SSH.Notice,
		[]installerHint{hintUp, hintDown, hintBack})
}

func (m Model) renderSSHTest() string {
	hints := []installerHint{hintType, hintSelect, hintBack}
	if m.SSH.Testing {
		hints = []installerHint{hintCancel, hintBack}
	}
	return m.renderThemeScreen(m.sshTestDescription(), m.SSH.Notice, hints)
}

// ---------------------------------------------------------------------------
// Messages and commands
// ---------------------------------------------------------------------------

// sshManagerLoadedMsg carries the resolved manager and the configuration read
// with it.
type sshManagerLoadedMsg struct {
	manager *sshcfg.Manager
	config  sshcfg.Config
	err     error
}

// sshEffectiveMsg carries the client's answer for one alias.
type sshEffectiveMsg struct {
	alias     string
	effective sshcfg.Effective
}

// sshKeysLoadedMsg carries the key inventory and the known hosts.
type sshKeysLoadedMsg struct {
	keys       []sshcfg.Key
	knownHosts []sshcfg.KnownHost
	err        error
	knownErr   string
}

// sshAgentLoadedMsg carries the agent's state.
type sshAgentLoadedMsg struct {
	state sshcfg.AgentState
}

// sshDiagnosticsMsg carries the diagnosis.
type sshDiagnosticsMsg struct {
	findings []sshcfg.Finding
	err      error
}

// sshTestedMsg carries one connection test's result. Seq identifies the run the
// answer belongs to, so an abandoned test's late answer is dropped rather than
// drawn over a newer one.
type sshTestedMsg struct {
	seq    int
	result sshcfg.ConnectResult
}

// sshWrittenMsg carries the outcome of a write.
type sshWrittenMsg struct {
	result sshcfg.WriteResult
	err    error
	what   string
}

// sshGeneratedMsg carries the outcome of a key generation.
type sshGeneratedMsg struct {
	result sshcfg.GenerateResult
	err    error
}

// sshAgentActionMsg carries the outcome of an agent action.
type sshAgentActionMsg struct {
	action sshcfg.AgentAction
	what   string
}

// sshCopiedMsg is the clipboard's answer: which mechanism was used, or why none
// was available.
type sshCopiedMsg struct {
	mechanism string
	err       error
}

// sshLoadCmd resolves the client, its tools and the configuration, off the
// update loop. The probe runs ssh -V and ssh -Q and parses the files, so it is a
// command rather than a render.
func sshLoadCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), sshManagerLoadTimeout)
		defer cancel()
		manager := sshcfg.Open(ctx, nil)
		return sshManagerLoadedMsg{manager: manager, config: manager.LoadConfig(ctx)}
	}
}

// sshConfigCmd re-reads the configuration with the client that is already
// resolved. It is what a write answers with: the list on the screen has to show
// what is now in the file, not what was there before the write.
func sshConfigCmd(manager *sshcfg.Manager) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), sshManagerActionTimeout)
		defer cancel()
		return sshManagerLoadedMsg{manager: manager, config: manager.LoadConfig(ctx)}
	}
}

// isSSHManagerScreen reports whether a screen belongs to the SSH manager. The
// key dispatcher uses it to route the manager's screens before the global
// meanings of space and backspace, and the escape handler uses it to reach each
// screen's own way back.
func isSSHManagerScreen(s Screen) bool {
	switch s {
	case ScreenSSHManager, ScreenSSHHost, ScreenSSHForm, ScreenSSHKeys,
		ScreenSSHAgent, ScreenSSHDiagnostics, ScreenSSHTest:
		return true
	}
	return false
}

// handleKeyPressSSH is the SSH manager's own key dispatch. It runs before the
// global handling of space, backspace and Escape, because three of these screens
// have a text field: the space belongs to the field the cursor is on, backspace
// deletes a character from it, and Escape means cancel-then-leave rather than
// leave. Ctrl+C is handled by the caller before this point, so the way out of
// the program is unchanged on every screen here.
func (m Model) handleKeyPressSSH(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch m.Screen {
	case ScreenSSHManager:
		return m.handleSSHManagerKeys(key)
	case ScreenSSHHost:
		return m.handleSSHHostKeys(key)
	case ScreenSSHForm:
		return m.handleSSHFormKeys(key)
	case ScreenSSHKeys:
		return m.handleSSHKeysKeys(key)
	case ScreenSSHAgent:
		return m.handleSSHAgentKeys(key)
	case ScreenSSHDiagnostics:
		return m.handleSSHDiagnosticsKeys(key)
	case ScreenSSHTest:
		return m.handleSSHTestKeys(key)
	}
	return m, nil
}

// handleSSHMessage applies one of the manager's asynchronous answers to the
// model and returns the command the answer makes necessary: a write re-reads the
// configuration, a generated key re-reads the inventory, an agent action
// re-reads both the agent and the inventory.
func (m Model) handleSSHMessage(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case sshManagerLoadedMsg:
		if msg.manager != nil {
			m.SSH.Manager = msg.manager
		}
		m.SSH.Hosts = msg.config.Hosts
		m.SSH.ConfigFiles = msg.config.Files
		m.SSH.ConfigFailures = msg.config.Failures
		m.SSH.MatchBlocks = msg.config.MatchBlocks
		m.SSH.HostsLoaded = true
		if msg.err != nil {
			m.SSH.HostsErr = msg.err.Error()
		} else {
			m.SSH.HostsErr = ""
		}
		if m.SSH.HostIndex >= len(m.SSH.Hosts) {
			m.SSH.HostIndex = 0
		}
		return m, nil, true

	case sshEffectiveMsg:
		if msg.alias != m.SSH.DetailAlias {
			return m, nil, true
		}
		m.SSH.Detail = msg.effective
		m.SSH.DetailErr = ""
		return m, nil, true

	case sshKeysLoadedMsg:
		m.SSH.Keys = msg.keys
		m.SSH.KnownHosts = msg.knownHosts
		m.SSH.KeysLoaded = true
		if msg.err != nil {
			m.SSH.KeysErr = msg.err.Error()
		} else {
			m.SSH.KeysErr = ""
		}
		m.SSH.KnownHostsErr = msg.knownErr
		if m.Cursor >= len(m.SSH.Keys) && m.SSH.KeysMode == "" {
			// The cursor was on a key that is gone; the action rows moved up under
			// it, so the cursor is clamped to the first action rather than left
			// pointing at a row that no longer exists.
			m.Cursor = len(m.SSH.Keys)
			if len(m.SSH.Keys) > 0 {
				m.Cursor++
			}
		}
		return m, nil, true

	case sshAgentLoadedMsg:
		m.SSH.Agent = msg.state
		m.SSH.AgentLoaded = true
		m.SSH.AgentConfirm = false
		return m, nil, true

	case sshDiagnosticsMsg:
		m.SSH.Findings = msg.findings
		m.SSH.DiagnosticsLoaded = true
		if msg.err != nil {
			m.SSH.DiagnosticsErr = msg.err.Error()
		} else {
			m.SSH.DiagnosticsErr = ""
		}
		return m, nil, true

	case sshTestedMsg:
		// A stale answer -- one from a run the user abandoned or cancelled -- is
		// dropped rather than drawn over a newer test.
		if msg.seq != m.SSH.TestSeq {
			return m, nil, true
		}
		m.SSH.Test = msg.result
		m.SSH.Testing = false
		m.SSH.Tested = true
		m.SSH.TestCancel = nil
		return m, nil, true

	case sshWrittenMsg:
		m.SSH.Writing = false
		if msg.err != nil {
			m.SSH.Notice = "Nothing was written: " + msg.err.Error()
			return m, nil, true
		}
		m.SSH.Plan = nil
		m.SSH.FormMode = ""
		m.SSH.Form = sshHostForm{}
		m.Screen = ScreenSSHManager
		m.Cursor = 0
		notice := "The change was written"
		if msg.result.Backup != "" {
			notice += ", after a backup at " + msg.result.Backup
		}
		if msg.result.Validated {
			notice += ", and validated by the client"
		} else if msg.result.Validation != "" {
			notice += ". Not validated: " + msg.result.Validation
		}
		m.SSH.Notice = notice + "."
		if m.SSH.Manager != nil {
			return m, sshConfigCmd(m.SSH.Manager), true
		}
		return m, nil, true

	case sshGeneratedMsg:
		m.SSH.Working = false
		if msg.err != nil {
			m.SSH.Notice = "No key was written: " + msg.err.Error()
			return m, nil, true
		}
		m.SSH.KeysMode = ""
		m.Cursor = 0
		m.SSH.Keygen = sshKeygenForm{}
		notice := "The key was written to " + msg.result.Key.Path + " with its public half at " + msg.result.Key.PubPath
		if msg.result.PassphraseConfirmed {
			notice += ", and the passphrase was confirmed to open it"
		} else if msg.result.Key.Encrypted == "" {
			notice += "; no passphrase was set"
		}
		m.SSH.Notice = notice + "."
		if m.SSH.Manager != nil {
			return m, sshKeysCmd(m.SSH.Manager), true
		}
		return m, nil, true

	case sshCopiedMsg:
		if msg.err != nil {
			m.SSH.Notice = "The public key is on the screen to select and copy: " + msg.err.Error() + ". A private key is never written to a clipboard."
			return m, nil, true
		}
		m.SSH.Notice = "The public key was copied with " + msg.mechanism + ". A private key is never copied."
		return m, nil, true

	case sshAgentActionMsg:
		if msg.action.OK {
			m.SSH.Notice = msg.action.Explanation
		} else {
			m.SSH.Notice = "Nothing changed: " + msg.action.Explanation
		}
		if m.SSH.Manager != nil {
			return m, tea.Batch(sshAgentCmd(m.SSH.Manager), sshKeysCmd(m.SSH.Manager)), true
		}
		return m, nil, true
	}
	return m, nil, false
}

// sshEffectiveCmd asks the client for one alias's effective configuration.
func sshEffectiveCmd(manager *sshcfg.Manager, alias string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), sshManagerActionTimeout)
		defer cancel()
		return sshEffectiveMsg{alias: alias, effective: manager.Resolve(ctx, alias)}
	}
}

// sshKeysCmd reads the key inventory and the known hosts.
func sshKeysCmd(manager *sshcfg.Manager) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), sshManagerActionTimeout)
		defer cancel()
		keys, err := manager.ListKeys(ctx)
		known, knownErr := manager.ListKnownHosts(ctx)
		msg := sshKeysLoadedMsg{keys: keys, knownHosts: known}
		if err != nil {
			msg.err = err
		}
		if knownErr != nil {
			msg.knownErr = knownErr.Error()
		}
		return msg
	}
}

// sshAgentCmd asks the agent for its state and identities.
func sshAgentCmd(manager *sshcfg.Manager) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), sshManagerActionTimeout)
		defer cancel()
		return sshAgentLoadedMsg{state: manager.AgentState(ctx)}
	}
}

// sshDiagnosticsCmd runs the diagnosis.
func sshDiagnosticsCmd(manager *sshcfg.Manager) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), sshManagerDiagnoseTimeout)
		defer cancel()
		return sshDiagnosticsMsg{findings: manager.Diagnose(ctx)}
	}
}

// sshTestCmd runs one connection test under the caller's context, which is what
// the Cancel row cancels.
func sshTestCmd(manager *sshcfg.Manager, ctx context.Context, seq int, alias string) tea.Cmd {
	return func() tea.Msg {
		return sshTestedMsg{seq: seq, result: manager.TestConnection(ctx, sshcfg.ConnectRequest{
			Alias:   alias,
			Timeout: sshTestTimeout,
		})}
	}
}

// sshApplyCmd writes a plan. It is the only command in this file that changes a
// file the user has, and it runs only after the confirmation row was pressed.
func sshApplyCmd(manager *sshcfg.Manager, plan sshcfg.WritePlan, what string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), sshManagerActionTimeout)
		defer cancel()
		result, err := manager.Apply(ctx, plan)
		return sshWrittenMsg{result: result, err: err, what: what}
	}
}

// sshGenerateKeyCmd creates a key pair. The passphrase is part of the request
// and travels from here straight into ssh-keygen's pipe.
func sshGenerateKeyCmd(manager *sshcfg.Manager, req sshcfg.KeygenRequest) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), sshKeygenTimeout)
		defer cancel()
		result, err := manager.GenerateKey(ctx, req)
		return sshGeneratedMsg{result: result, err: err}
	}
}

// sshAgentActionCmd runs one agent action.
func sshAgentActionCmd(run func() sshcfg.AgentAction, what string) tea.Cmd {
	return func() tea.Msg {
		return sshAgentActionMsg{action: run(), what: what}
	}
}

// sshCopyCmd puts a public key on the clipboard through the first mechanism this
// host has. The key is written to the tool's standard input, never to an
// argument list, and when no mechanism exists the screen says so instead of
// pretending the copy happened.
func sshCopyCmd(text string) tea.Cmd {
	return func() tea.Msg {
		mechanism, command, err := sshClipboardTool()
		if err != nil {
			return sshCopiedMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), sshClipboardTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, command[0], command[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return sshCopiedMsg{mechanism: mechanism, err: fmt.Errorf("%s refused the key: %w", mechanism, err)}
		}
		return sshCopiedMsg{mechanism: mechanism}
	}
}

// sshClipboardTool is the first clipboard mechanism this host has, in the order
// that matches how a developer's terminal usually reaches a clipboard: the
// platform's own tool when there is one, and a refusal with a reason when there
// is none. The public key is the only thing this ever carries.
func sshClipboardTool() (string, []string, error) {
	candidates := [][]string{
		{"pbcopy"},
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--input", "--clipboard"},
		{"clip.exe"},
	}
	for _, candidate := range candidates {
		if _, err := lookPathFunc(candidate[0]); err == nil {
			return candidate[0], candidate, nil
		}
	}
	return "", nil, fmt.Errorf("no clipboard tool was found (pbcopy, wl-copy, xclip, xsel or clip.exe)")
}

// lookPathFunc is exec.LookPath, and it is a variable so a test can state which
// clipboard tools a host has.
var lookPathFunc = exec.LookPath

// ---------------------------------------------------------------------------
// Bounds
// ---------------------------------------------------------------------------

const (
	// sshManagerLoadTimeout bounds resolving the client and reading the
	// configuration. The probe runs the client twice and parses the files, and a
	// filesystem that is not answering must not hold the screen.
	sshManagerLoadTimeout = 20 * time.Second
	// sshManagerActionTimeout bounds the actions that answer a press: the
	// client's effective answer, the key inventory, the agent's state, one write.
	sshManagerActionTimeout = 20 * time.Second
	// sshManagerDiagnoseTimeout bounds the diagnosis, which parses every included
	// file and asks the client to parse the configuration once.
	sshManagerDiagnoseTimeout = 20 * time.Second
	// sshKeygenTimeout bounds key generation. ssh-keygen's own work is fast; the
	// bound is there for a host whose entropy source is slow, and it is long
	// enough for the KDF rounds the recommended algorithm uses.
	sshKeygenTimeout = 60 * time.Second
	// sshTestTimeout bounds one connection test. The client is given the same
	// number of seconds as its own ConnectTimeout, so the test cannot outlive the
	// bound the screen names.
	sshTestTimeout = 10 * time.Second
	// sshClipboardTimeout bounds the clipboard tool, which must never hold the
	// interface behind a process that is waiting on a display that is not there.
	sshClipboardTimeout = 5 * time.Second
	// sshTestOutputRows is how many lines of the client's own output a test result
	// may spend. The rest is a count: evidence belongs on the screen, a wall of it
	// does not.
	sshTestOutputRows = 6
)

// ---------------------------------------------------------------------------
// Reading the state
// ---------------------------------------------------------------------------

// sshSelectedHost is the entry the detail screen is about.
func (m Model) sshSelectedHost() (sshcfg.Host, bool) {
	if m.SSH.HostIndex < 0 || m.SSH.HostIndex >= len(m.SSH.Hosts) {
		return sshcfg.Host{}, false
	}
	return m.SSH.Hosts[m.SSH.HostIndex], true
}

// sshSelectedKey is the key an action targets: the row under the cursor in the
// list, and the key the action was started for in the two sub-states, because
// neither sub-state draws the list the cursor was on.
func (m Model) sshSelectedKey() (sshcfg.Key, bool) {
	index := m.Cursor
	if m.SSH.KeysMode != "" {
		index = m.SSH.KeyIndex
	}
	if index < 0 || index >= len(m.SSH.Keys) {
		return sshcfg.Key{}, false
	}
	return m.SSH.Keys[index], true
}

// sshSelectedKeyName names the key an action targets, for a sentence.
func (m Model) sshSelectedKeyName() string {
	key, ok := m.sshSelectedKey()
	if !ok {
		return "the selected key"
	}
	return filepath.Base(key.Path)
}

// sshSelectedKeyInAgent reports whether the selected key is one the agent
// already holds. The match is the fingerprint, which is the only identity both
// sides state: the agent prints fingerprints and the public half has one.
func (m Model) sshSelectedKeyInAgent() bool {
	key, ok := m.sshSelectedKey()
	if !ok || key.Fingerprint == "" {
		return false
	}
	for _, id := range m.SSH.Agent.Identities {
		if id.Fingerprint == key.Fingerprint {
			return true
		}
	}
	return false
}

// sshSelectedFinding is the finding the diagnosis body is about: the row under
// the cursor, because that list is read-only and the cursor is the selection.
func (m Model) sshSelectedFinding() (sshcfg.Finding, bool) {
	if m.Cursor < 0 || m.Cursor >= len(m.SSH.Findings) {
		return sshcfg.Finding{}, false
	}
	return m.SSH.Findings[m.Cursor], true
}

// sshDetailPublicKey is the public half of the detail screen's identity, or
// empty when there is none to show. It reads the key inventory the manager is
// already holding: the detail screen is opened after the list that filled it.
func (m Model) sshDetailPublicKey() string {
	identity := ""
	if len(m.SSH.Detail.IdentityFiles) > 0 {
		identity = m.SSH.Detail.IdentityFiles[0]
	}
	if identity == "" {
		if host, ok := m.sshSelectedHost(); ok {
			identity = sshOptionValue(host, "identityfile")
		}
	}
	if identity == "" {
		return ""
	}
	home := m.sshHome()
	for _, key := range m.SSH.Keys {
		if key.Public == "" {
			continue
		}
		if sshSamePath(key.PubPath, identity, home) || sshSamePath(key.Path, identity, home) {
			return key.Public
		}
	}
	return ""
}

// sshHome is the home directory the resolved manager works with, or the
// process's own when nothing has been resolved yet.
func (m Model) sshHome() string {
	if m.SSH.Manager != nil {
		return m.SSH.Manager.Paths.Home
	}
	return sshcfg.DefaultPaths().Home
}

// sshHomeForm is a path reduced to its form relative to one home, so the same key
// reached as ~/.ssh/id_ed25519 and as /home/alice/.ssh/id_ed25519 has one form.
// It is used for comparison only: nothing here rebuilds an absolute path, which is
// what keeps this file free of a $HOME join whose destination the installer's own
// path guard would have to classify -- this screen writes nothing itself.
func sshHomeForm(path, home string) string {
	if path == "~" {
		return "."
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Clean(rest)
	}
	if home != "" {
		if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
			return filepath.Clean(rest)
		}
	}
	return filepath.Clean(path)
}

// sshSamePath compares two paths as the client would resolve them: a leading ~/
// means the home the paths were resolved for, and everything else is compared as
// written. Two empty paths are not the same file.
func sshSamePath(a, b, home string) bool {
	if a == "" || b == "" {
		return false
	}
	return sshHomeForm(a, home) == sshHomeForm(b, home)
}

// sshManagerPath is the user configuration file the manager writes to.
func sshManagerPath(mgr *sshcfg.Manager) string {
	if mgr == nil {
		return sshDefaultConfigPath
	}
	return sshHomePath(mgr.Paths.Home, mgr.Paths.UserConfig)
}

// sshDefaultConfigPath is the file the manager writes when no paths were resolved
// yet: the client's own default, written the way a person writes it.
const sshDefaultConfigPath = "~/.ssh/config"

// sshHomePath renders one of the client's paths relative to the home it was
// resolved for, so the screen names ~/.ssh/config instead of printing whichever
// directory this machine happens to use. A path outside that home is printed as
// it is: it is not this tool's place to shorten somebody else's path.
func sshHomePath(home, path string) string {
	if home == "" || path == "" {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return path
	}
	return "~/" + filepath.ToSlash(rel)
}

// sshManagerDir is the directory the keys live in.
func sshManagerDir(mgr *sshcfg.Manager) string {
	if mgr == nil {
		return "~/.ssh"
	}
	return sshHomePath(mgr.Paths.Home, mgr.Paths.SSHDir)
}

// sshReplaceKeygenRow is the replace switch as a row, carrying its state.
func sshReplaceKeygenRow(replace bool) string {
	if replace {
		return sshReplaceRowOn
	}
	return sshReplaceRowOff
}

// sshCanTest reports whether this machine has a client to run a connection test
// with. A row that cannot run is not offered at all: the screen says why in its
// body instead of offering a press that would fail.
func (m Model) sshCanTest() bool {
	return m.SSH.Manager != nil && m.SSH.Manager.Caps.HasSSH()
}

// sshMask is a secret as the screen draws it: one bullet per character, never
// the characters. An empty secret is named as empty so the field is
// distinguishable from one holding a single space.
func sshMask(secret string) string {
	if secret == "" {
		return ""
	}
	return strings.Repeat("•", len([]rune(secret)))
}

// ---------------------------------------------------------------------------
// The form's own validation
// ---------------------------------------------------------------------------

// sshFormRefusals are the reasons the form cannot be written as it stands, in
// the order the fields appear. They are validation rather than rewriting: the
// form never changes what the user typed, it says what would be refused.
func (m Model) sshFormRefusals() []string {
	form := m.SSH.Form
	var refusals []string

	alias := strings.TrimSpace(form.Alias)
	switch {
	case alias == "":
		refusals = append(refusals, "An alias is required: it is the name the client matches on.")
	case strings.ContainsAny(alias, "*?!"):
		refusals = append(refusals, "The alias must be a single literal name: a wildcard, a pattern list or a negation cannot be an entry this tool manages.")
	case strings.ContainsAny(alias, " \t"):
		refusals = append(refusals, "The alias cannot contain a space.")
	default:
		for i, host := range m.SSH.Hosts {
			if i == m.SSH.HostIndex && m.SSH.FormMode == "edit" {
				continue
			}
			if host.Alias == alias {
				refusals = append(refusals, fmt.Sprintf("Another entry already uses the alias %s in %s; the client would stop at the first one, so a second entry with that name would never apply.", alias, sshShortPath(m.SSH.Manager, host.File)))
				break
			}
		}
	}

	if port := strings.TrimSpace(form.Port); port != "" {
		if !sshIsPort(port) {
			refusals = append(refusals, "The port must be a number between 1 and 65535.")
		}
	}
	return refusals
}

// sshIsPort reports whether a value is a port number.
func sshIsPort(value string) bool {
	n := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
		if n > 65535 {
			return false
		}
	}
	return n > 0
}

// sshFormReady reports whether the form's own validation passes, which is what
// puts the save row on the screen.
func (m Model) sshFormReady() bool { return len(m.sshFormRefusals()) == 0 }

// sshKeygenReady reports whether the generation form can be submitted: a path
// and two identical passphrases.
func (m Model) sshKeygenReady() bool {
	form := m.SSH.Keygen
	return strings.TrimSpace(form.Path) != "" && form.Passphrase == form.Confirm
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

// handleSSHManagerKeys drives the manager's own screen. The cursor walks the
// host rows and then the jobs; a host row opens that host's detail screen.
func (m Model) handleSSHManagerKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	if m.moveSSHRow(key, options) {
		return m, nil
	}

	switch key {
	case "esc":
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.SSH.Notice = ""
		return m, nil
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		if m.Cursor < len(m.SSH.Hosts) {
			m.SSH.HostIndex = m.Cursor
			m.SSH.DetailAlias = m.SSH.Hosts[m.Cursor].Alias
			m.SSH.Detail = sshcfg.Effective{Alias: m.SSH.DetailAlias}
			m.SSH.DetailErr = ""
			m.Screen = ScreenSSHHost
			m.Cursor = 0
			m.SSH.Notice = ""
			if m.SSH.Manager == nil {
				return m, nil
			}
			return m, sshEffectiveCmd(m.SSH.Manager, m.SSH.DetailAlias)
		}
		switch options[m.Cursor] {
		case sshAddHostRow:
			return m.startSSHForm("create", sshHostForm{}), nil
		case sshKeysRow:
			m.Screen = ScreenSSHKeys
			m.Cursor = 0
			m.SSH.KeysMode = ""
			m.SSH.Notice = ""
			if m.SSH.Manager == nil {
				return m, nil
			}
			return m, sshKeysCmd(m.SSH.Manager)
		case sshAgentRow:
			m.Screen = ScreenSSHAgent
			m.Cursor = 0
			m.SSH.Notice = ""
			m.SSH.AgentConfirm = false
			if m.SSH.Manager == nil {
				return m, nil
			}
			return m, sshAgentCmd(m.SSH.Manager)
		case sshDiagnoseRow:
			m.Screen = ScreenSSHDiagnostics
			m.Cursor = 0
			m.SSH.Notice = ""
			if m.SSH.Manager == nil {
				return m, nil
			}
			return m, sshDiagnosticsCmd(m.SSH.Manager)
		case sshTestRow:
			return m.openSSHTest(ScreenSSHManager), nil
		case sshBackRow:
			m.Screen = ScreenUtilities
			m.Cursor = 0
			m.SSH.Notice = ""
		}
	}
	return m, nil
}

// moveSSHRow moves the cursor over one of the manager's menus, skipping the
// separators the same way every other menu in the interface does. It reports
// whether the key was a move, so a handler can leave the rest alone.
func (m *Model) moveSSHRow(key string, options []string) bool {
	switch key {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if strings.HasPrefix(options[m.Cursor], menuSeparatorPrefix) && m.Cursor > 0 {
				m.Cursor--
			}
		}
		return true
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if strings.HasPrefix(options[m.Cursor], menuSeparatorPrefix) && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
		return true
	}
	return false
}

// handleSSHHostKeys drives one entry's detail screen: the actions that change it
// are offered only where they can run, and the test row is always there because
// asking the client is always safe.
func (m Model) handleSSHHostKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()
	if m.moveSSHRow(key, options) {
		return m, nil
	}

	switch key {
	case "esc":
		m.Screen = ScreenSSHManager
		m.Cursor = m.SSH.HostIndex
		m.SSH.Notice = ""
		return m, nil
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		switch options[m.Cursor] {
		case sshEditRow:
			return m.startSSHForm("edit", sshFormFromHost(m.sshSelectedHostOrZero(), m.SSH.Detail)), nil
		case sshDuplicateRow:
			return m.startSSHForm("duplicate", sshFormFromHost(m.sshSelectedHostOrZero(), m.SSH.Detail)), nil
		case sshDeleteRow:
			return m.startSSHDelete(), nil
		case sshTestThisRow:
			m.SSH.TestAlias = m.SSH.DetailAlias
			return m.openSSHTest(ScreenSSHHost), nil
		case sshCopyPubRow:
			if pub := m.sshDetailPublicKey(); pub != "" {
				m.SSH.Notice = "Copying the public key…"
				return m, sshCopyCmd(pub)
			}
		case sshBackToHostsRow:
			m.Screen = ScreenSSHManager
			m.Cursor = m.SSH.HostIndex
			m.SSH.Notice = ""
		}
	}
	return m, nil
}

// sshSelectedHostOrZero is the selected entry as a value, so the form builders
// take one shape whether or not a selection exists.
func (m Model) sshSelectedHostOrZero() sshcfg.Host {
	host, _ := m.sshSelectedHost()
	return host
}

// sshFormFromHost fills the form from an entry: the declared values first, then
// the client's effective answer for anything the entry does not declare. That
// order is what makes editing an entry that inherits `User` from a wildcard
// block show the user that would really be used.
func sshFormFromHost(host sshcfg.Host, effective sshcfg.Effective) sshHostForm {
	form := sshHostForm{
		Alias:    host.Alias,
		HostName: sshOptionValue(host, "hostname"),
		User:     sshOptionValue(host, "user"),
		Port:     sshOptionValue(host, "port"),
		Identity: sshOptionValue(host, "identityfile"),
	}
	if form.HostName == "" {
		form.HostName = effective.HostName
	}
	if form.User == "" {
		form.User = effective.User
	}
	if form.Port == "" {
		form.Port = effective.Port
	}
	if form.Identity == "" && len(effective.IdentityFiles) > 0 {
		form.Identity = effective.IdentityFiles[0]
	}
	return form
}

// startSSHForm opens the form for one of its three jobs. A duplicate starts from
// the same values with a name the client can tell apart, because an entry whose
// alias the original already owns would never apply.
func (m Model) startSSHForm(mode string, form sshHostForm) Model {
	if mode == "duplicate" {
		form.Alias = sshDuplicateAlias(form.Alias)
		form.EditTarget = form.Alias
	} else if mode == "edit" {
		form.EditTarget = form.Alias
	}
	m.SSH.Form = form
	m.SSH.FormMode = mode
	m.SSH.Plan = nil
	m.SSH.Writing = false
	m.SSH.Notice = ""
	m.Screen = ScreenSSHForm
	m.Cursor = 0
	return m
}

// sshDuplicateAlias is the name a duplicate starts with. It suggests rather than
// decides: the form shows it and the user may change it, and the validation
// refuses it when another entry already owns it.
func sshDuplicateAlias(alias string) string {
	if alias == "" {
		return "new-host"
	}
	return alias + "-copy"
}

// handleSSHFormKeys drives the form and the confirmation that follows it. Text
// keys reach exactly one of the six fields -- the one under the cursor -- and
// every other key moves, saves or leaves.
func (m Model) handleSSHFormKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	if m.SSH.Plan != nil {
		switch key {
		case "up", "k", "down", "j":
			m.moveSSHRow(key, options)
			return m, nil
		case "esc":
			// The plan was a preview of a write, so leaving it says the write did not
			// happen: a screen that goes back silently leaves the user unsure whether
			// the file was touched.
			if m.SSH.Plan != nil {
				m.SSH.Notice = "Nothing was written: the change was discarded before it reached the file."
			}
			// The notice set above is the point of this branch, so it is not cleared
			// here: the plan is gone and the screen says what that means.
			m.SSH.Plan = nil
			return m, nil
		case "enter", " ":
			if m.Cursor < 0 || m.Cursor >= len(options) {
				return m, nil
			}
			switch options[m.Cursor] {
			case sshSaveRow:
				if m.SSH.Manager == nil || m.SSH.Writing {
					return m, nil
				}
				m.SSH.Writing = true
				m.SSH.Notice = "Writing the entry, after a backup and a validation…"
				return m, sshApplyCmd(m.SSH.Manager, *m.SSH.Plan, m.sshPlanWhat())
			case sshDiscardRow:
				m.SSH.Plan = nil
				m.SSH.Notice = "Nothing was written."
			}
		}
		return m, nil
	}

	if m.moveSSHRow(key, options) {
		return m, nil
	}

	switch key {
	case "esc":
		return m.leaveSSHForm(), nil
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		switch options[m.Cursor] {
		case sshSaveRow:
			return m.planSSHForm(), nil
		case sshCancelRow:
			return m.leaveSSHForm(), nil
		}
	default:
		if m.Cursor < sshFormFieldCount {
			m.typeSSHFormField(key)
		}
	}
	return m, nil
}

// sshPlanWhat names what the plan does, for the result line.
func (m Model) sshPlanWhat() string {
	switch m.SSH.FormMode {
	case "create":
		return sshCreatedNotice
	case "edit":
		return sshUpdatedNotice
	case "duplicate":
		return sshDuplicatedNotice
	}
	return sshWrittenNotice
}

// leaveSSHForm closes the form and returns to the screen it was opened from.
func (m Model) leaveSSHForm() Model {
	m.SSH.Plan = nil
	m.SSH.FormMode = ""
	m.SSH.Form = sshHostForm{}
	m.Screen = ScreenSSHManager
	m.Cursor = m.SSH.HostIndex
	return m
}

// typeSSHFormField appends text to the field under the cursor, or deletes the
// last character on backspace. Keys that name an action never reach here: the
// caller only sends keys that are not one of the form's own.
func (m *Model) typeSSHFormField(key string) {
	if key == "backspace" {
		m.SSH.Form = sshFormWithoutLastRune(m.SSH.Form, m.Cursor)
		return
	}
	if !sshTypingKey(key) {
		return
	}
	m.SSH.Form = sshFormWithText(m.SSH.Form, m.Cursor, key)
}

// sshTypingKey reports whether a key is text rather than a command. A single
// rune is text; every named key -- up, enter, tab, esc -- is not.
func sshTypingKey(key string) bool {
	runes := []rune(key)
	if len(runes) != 1 {
		return false
	}
	r := runes[0]
	return r >= ' ' && r != 0x7f
}

// sshFormFieldCount is how many of the form's rows are fields rather than
// actions. The rows after them are the separator and the two actions.
const sshFormFieldCount = 6

// sshFormWithText appends text to the field at index.
func sshFormWithText(form sshHostForm, index int, text string) sshHostForm {
	switch index {
	case 0:
		form.Alias += text
	case 1:
		form.HostName += text
	case 2:
		form.User += text
	case 3:
		form.Port += text
	case 4:
		form.Identity += text
	case 5:
		form.ProxyJump += text
	}
	return form
}

// sshFormWithoutLastRune removes the last character of the field at index.
func sshFormWithoutLastRune(form sshHostForm, index int) sshHostForm {
	trim := func(value string) string {
		runes := []rune(value)
		if len(runes) == 0 {
			return value
		}
		return string(runes[:len(runes)-1])
	}
	switch index {
	case 0:
		form.Alias = trim(form.Alias)
	case 1:
		form.HostName = trim(form.HostName)
	case 2:
		form.User = trim(form.User)
	case 3:
		form.Port = trim(form.Port)
	case 4:
		form.Identity = trim(form.Identity)
	case 5:
		form.ProxyJump = trim(form.ProxyJump)
	}
	return form
}

// planSSHForm turns the form into a plan: nothing is written yet, and the plan's
// own preview is what the confirmation shows. The plan is built by the write
// layer, so the form cannot describe a change the writer would not make.
func (m Model) planSSHForm() Model {
	if m.SSH.Manager == nil || !m.sshFormReady() {
		return m
	}
	spec := sshHostSpec(m.SSH.Form)
	what := m.SSH.FormMode

	var plan sshcfg.WritePlan
	var err error
	switch what {
	case "create":
		plan, err = m.SSH.Manager.PlanCreate(context.Background(), spec)
	case "edit":
		plan, err = m.SSH.Manager.PlanUpdate(context.Background(), m.SSH.Form.EditTarget, spec)
	case "duplicate":
		plan, err = m.SSH.Manager.PlanDuplicate(context.Background(), m.SSH.Form.EditTarget, spec.Alias)
	default:
		err = fmt.Errorf("no entry is being written")
	}
	if err != nil {
		m.SSH.Notice = err.Error()
		return m
	}
	m.SSH.Plan = &plan
	m.SSH.Notice = "Showing the change: nothing has been written yet."
	m.Cursor = 0
	return m
}

// startSSHDelete plans the removal of the selected entry. Removing an entry is
// a write like any other: it is previewed, confirmed, backed up and validated.
func (m Model) startSSHDelete() Model {
	if m.SSH.Manager == nil {
		return m
	}
	plan, err := m.SSH.Manager.PlanDelete(context.Background(), m.SSH.DetailAlias)
	if err != nil {
		m.SSH.Notice = err.Error()
		return m
	}
	m.SSH.Plan = &plan
	m.Screen = ScreenSSHForm
	m.Cursor = 0
	m.SSH.Notice = "Showing the removal: nothing has been written yet."
	m.SSH.FormMode = "delete"
	return m
}

// sshHostSpec is the form as the write layer takes it.
func sshHostSpec(form sshHostForm) sshcfg.HostSpec {
	return sshcfg.HostSpec{
		Alias:        strings.TrimSpace(form.Alias),
		HostName:     strings.TrimSpace(form.HostName),
		User:         strings.TrimSpace(form.User),
		Port:         strings.TrimSpace(form.Port),
		IdentityFile: strings.TrimSpace(form.Identity),
		ProxyJump:    strings.TrimSpace(form.ProxyJump),
	}
}

// The sentences a completed write leaves on the screen. They name the file's
// outcome, never its content: a host entry is configuration, not a secret, but
// the rule that a result line names what happened is the same one the key
// screens follow with a passphrase.
const (
	sshCreatedNotice    = "The entry was written, after a backup and a validation."
	sshUpdatedNotice    = "The entry was updated, after a backup and a validation."
	sshDuplicatedNotice = "The copy was written as its own entry, after a backup and a validation."
	sshDeletedNotice    = "The entry was removed, after a backup and a validation."
	sshWrittenNotice    = "The file was written, after a backup and a validation."
)

// ---------------------------------------------------------------------------
// The key screen
// ---------------------------------------------------------------------------

// sshKeygenTypes is the key types the generation form offers, in the order
// OpenSSH's own recommendations put them: ed25519 first, then the same algorithm
// on a hardware token, then ECDSA, and RSA last because it is the compatibility
// choice rather than the default one. The list is filtered against what this
// client reports it can generate, so a row is never offered for a type the tool
// would refuse.
func sshKeygenTypes(caps sshcfg.Caps) []string {
	preferred := []string{
		"ed25519",
		"sk-ssh-ed25519@openssh.com",
		"ecdsa",
		"sk-ecdsa-sha2-nistp256@openssh.com",
		"rsa",
	}
	if len(caps.KeygenSupports) == 0 {
		return preferred
	}
	var offered []string
	for _, want := range preferred {
		for _, supported := range caps.KeygenSupports {
			if sshKeygenTypeMatches(want, supported) {
				offered = append(offered, want)
				break
			}
		}
	}
	if len(offered) == 0 {
		// The client listed its key types but none of the names this form knows:
		// the form says nothing rather than offering a type that would be refused.
		return nil
	}
	return offered
}

// sshKeygenTypeMatches compares a form type with a wire type the client listed.
// The client lists wire names (`ssh-ed25519`, `ecdsa-sha2-nistp256`), while the
// form names what ssh-keygen's -t flag takes (`ed25519`, `ecdsa`).
func sshKeygenTypeMatches(want, supported string) bool {
	switch want {
	case "ed25519":
		return supported == "ssh-ed25519"
	case "ecdsa":
		return strings.HasPrefix(supported, "ecdsa-sha2-")
	case "rsa":
		return supported == "ssh-rsa"
	default:
		return supported == want
	}
}

// sshKeygenFieldCount is how many of the key form's rows are text fields. The
// rows after them are the replace switch, the separator and the two actions.
const sshKeygenFieldCount = 5

// handleSSHKeysKeys drives the key screen in its three states. The replace row
// is a switch rather than a text field, which is why the count of fields and the
// index of the switch are named constants instead of a bare number.
func (m Model) handleSSHKeysKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	switch m.SSH.KeysMode {
	case "generate":
		return m.handleSSHKeygenKeys(key, options)
	case "add":
		return m.handleSSHAddKeys(key, options)
	}

	if m.moveSSHRow(key, options) {
		return m, nil
	}

	switch key {
	case "esc":
		m.Screen = ScreenSSHManager
		m.Cursor = 0
		m.SSH.Notice = ""
		return m, nil
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		if m.Cursor < len(m.SSH.Keys) {
			return m, nil
		}
		switch options[m.Cursor] {
		case sshGenerateKeyRow:
			m.SSH.KeysMode = "generate"
			m.SSH.Keygen = sshKeygenForm{
				Path:    sshManagerDir(m.SSH.Manager) + "/id_ed25519",
				Comment: sshDefaultKeyComment(),
			}
			if types := sshKeygenTypes(m.sshCaps()); len(types) > 0 {
				m.SSH.Keygen.Type = types[0]
			}
			m.Cursor = 0
			m.SSH.Notice = ""
		case sshCopyKeyRow:
			if key, ok := m.sshSelectedKey(); ok && key.Public != "" {
				m.SSH.Notice = "Copying the public key…"
				return m, sshCopyCmd(key.Public)
			}
		case sshAddKeyRow:
			if key, ok := m.sshSelectedKey(); ok {
				m.SSH.KeyIndex = m.Cursor
				m.SSH.KeysMode = "add"
				m.SSH.AddPassphrase = ""
				m.Cursor = 0
				m.SSH.Notice = ""
				_ = key
			}
		case sshRemoveKeyRow:
			return m.removeSelectedKeyFromAgent()
		case sshBackRow:
			m.Screen = ScreenSSHManager
			m.Cursor = 0
			m.SSH.Notice = ""
		}
	}
	return m, nil
}

// sshCaps is the resolved client capabilities, empty when nothing was resolved
// yet so a form can be opened without a manager and still draw its rows.
func (m Model) sshCaps() sshcfg.Caps {
	if m.SSH.Manager == nil {
		return sshcfg.Caps{}
	}
	return m.SSH.Manager.Caps
}

// sshDefaultKeyComment is the comment a new key starts with: the user and the
// host it was made on, which is what makes a key identifiable on the server's
// own list later. A host with neither writes no comment rather than a guess.
func sshDefaultKeyComment() string {
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}
	host, _ := os.Hostname()
	switch {
	case user != "" && host != "":
		return user + "@" + host
	case user != "":
		return user
	default:
		return host
	}
}

// handleSSHKeygenKeys drives the generation form. The type row cycles through
// the types this client supports with the left and right arrows, which is what
// the footer names; every other field takes text.
func (m Model) handleSSHKeygenKeys(key string, options []string) (tea.Model, tea.Cmd) {
	if m.moveSSHRow(key, options) {
		return m, nil
	}

	if key == "left" || key == "right" {
		if m.Cursor == 1 {
			m.SSH.Keygen.Type = sshCycleType(m.SSH.Keygen.Type, m.sshCaps(), key == "right")
			return m, nil
		}
	}

	switch key {
	case "esc":
		m.SSH.KeysMode = ""
		m.Cursor = 0
		m.SSH.Notice = ""
		return m, nil
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		switch options[m.Cursor] {
		case sshReplaceRowOn, sshReplaceRowOff:
			m.SSH.Keygen.Replace = !m.SSH.Keygen.Replace
		case sshGenerateKeyRow:
			return m, m.generateSSHKey()
		case sshCancelRow:
			m.SSH.KeysMode = ""
			m.Cursor = 0
			m.SSH.Notice = ""
		}
		return m, nil
	}

	if m.Cursor < sshKeygenFieldCount {
		m.typeSSHKeygenField(key)
	}
	return m, nil
}

// sshCycleType moves the chosen key type one step through the offered list.
func sshCycleType(current string, caps sshcfg.Caps, forward bool) string {
	types := sshKeygenTypes(caps)
	if len(types) == 0 {
		return current
	}
	index := 0
	for i, t := range types {
		if t == current {
			index = i
			break
		}
	}
	if forward {
		index = (index + 1) % len(types)
	} else {
		index = (index - 1 + len(types)) % len(types)
	}
	return types[index]
}

// typeSSHKeygenField appends text to the focused field of the generation form.
// The passphrase fields take text like any other and are never echoed: the row
// draws the mask instead of the value.
func (m *Model) typeSSHKeygenField(key string) {
	trim := func(value string) string {
		runes := []rune(value)
		if len(runes) == 0 {
			return value
		}
		return string(runes[:len(runes)-1])
	}
	appendText := func(value string) string {
		if !sshTypingKey(key) {
			return value
		}
		return value + key
	}
	form := m.SSH.Keygen
	switch m.Cursor {
	case 0:
		if key == "backspace" {
			form.Path = trim(form.Path)
		} else {
			form.Path = appendText(form.Path)
		}
	case 1:
		// The type is chosen from the list the client supports, so text does not
		// reach it: a typed type would be one the client might refuse.
		return
	case 2:
		if key == "backspace" {
			form.Comment = trim(form.Comment)
		} else {
			form.Comment = appendText(form.Comment)
		}
	case 3:
		if key == "backspace" {
			form.Passphrase = trim(form.Passphrase)
		} else {
			form.Passphrase = appendText(form.Passphrase)
		}
	case 4:
		if key == "backspace" {
			form.Confirm = trim(form.Confirm)
		} else {
			form.Confirm = appendText(form.Confirm)
		}
	}
	m.SSH.Keygen = form
}

// generateSSHKey submits the generation form. The confirmation of a passphrase
// mismatch is the form's own validation, so nothing is generated until the two
// fields agree, and an existing pair is refused unless the replace row was
// turned on.
func (m Model) generateSSHKey() tea.Cmd {
	if m.SSH.Manager == nil || m.SSH.Working {
		return nil
	}
	form := m.SSH.Keygen
	if !m.sshKeygenReady() {
		m.SSH.Notice = "The two passphrase fields must match, and the file name cannot be empty."
		return nil
	}
	if strings.TrimSpace(form.Type) == "" {
		m.SSH.Notice = "No key type is available here: this client reported the types it can generate and none of the recommended ones was among them."
		return nil
	}
	m.SSH.Working = true
	m.SSH.Notice = "Generating the key with ssh-keygen…"
	return sshGenerateKeyCmd(m.SSH.Manager, sshcfg.KeygenRequest{
		Path:         strings.TrimSpace(form.Path),
		Type:         form.Type,
		Comment:      form.Comment,
		Passphrase:   form.Passphrase,
		AllowReplace: form.Replace,
	})
}

// handleSSHAddKeys drives the passphrase prompt that adding a key to the agent
// needs. The passphrase is required for an encrypted key and must be omitted for
// one that is not, so an empty field is a value rather than a missing answer.
func (m Model) handleSSHAddKeys(key string, options []string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.SSH.KeysMode = ""
		m.SSH.AddPassphrase = ""
		m.Cursor = 0
		m.SSH.Notice = ""
		return m, nil
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		switch options[m.Cursor] {
		case sshAddToAgentRow:
			return m, m.addSelectedKeyToAgent()
		case sshCancelRow:
			m.SSH.KeysMode = ""
			m.SSH.AddPassphrase = ""
			m.Cursor = 0
			m.SSH.Notice = ""
		}
		return m, nil
	case "up", "k", "down", "j":
		m.moveSSHRow(key, options)
		return m, nil
	case "backspace":
		runes := []rune(m.SSH.AddPassphrase)
		if len(runes) > 0 {
			m.SSH.AddPassphrase = string(runes[:len(runes)-1])
		}
		return m, nil
	default:
		if sshTypingKey(key) {
			m.SSH.AddPassphrase += key
		}
	}
	return m, nil
}

// addSelectedKeyToAgent hands the selected key to ssh-add with the passphrase
// the user typed. The passphrase is part of the request and never part of the
// command's arguments.
func (m Model) addSelectedKeyToAgent() tea.Cmd {
	key, ok := m.sshSelectedKey()
	if !ok || m.SSH.Manager == nil {
		return nil
	}
	manager := m.SSH.Manager
	path := key.Path
	passphrase := m.SSH.AddPassphrase
	m.SSH.KeysMode = ""
	m.SSH.AddPassphrase = ""
	m.SSH.Notice = "Adding " + filepath.Base(path) + " to the agent…"
	return sshAgentActionCmd(func() sshcfg.AgentAction {
		ctx, cancel := context.WithTimeout(context.Background(), sshManagerActionTimeout)
		defer cancel()
		return manager.AddToAgent(ctx, path, passphrase, 0)
	}, "add")
}

// removeSelectedKeyFromAgent removes the selected key's identity from the agent.
// ssh-add removes by public key file, so a key whose public half is missing is
// refused here with that reason rather than by a tool that cannot do it either.
func (m Model) removeSelectedKeyFromAgent() (tea.Model, tea.Cmd) {
	key, ok := m.sshSelectedKey()
	if !ok || m.SSH.Manager == nil {
		return m, nil
	}
	if key.PubPath == "" {
		m.SSH.Notice = "The public half of " + filepath.Base(key.Path) + " was not found, and ssh-add removes an identity by its public key file. Nothing was removed."
		return m, nil
	}
	manager := m.SSH.Manager
	pubPath := key.PubPath
	m.SSH.Notice = "Removing " + filepath.Base(key.Path) + " from the agent…"
	return m, sshAgentActionCmd(func() sshcfg.AgentAction {
		ctx, cancel := context.WithTimeout(context.Background(), sshManagerActionTimeout)
		defer cancel()
		return manager.RemoveFromAgent(ctx, pubPath)
	}, "remove")
}

// ---------------------------------------------------------------------------
// The agent screen
// ---------------------------------------------------------------------------

// handleSSHAgentKeys drives the agent screen. Removing every identity is the one
// destructive action here, and it takes two presses: the first arms it and says
// so in the screen's own words, the second runs it.
func (m Model) handleSSHAgentKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()
	if m.moveSSHRow(key, options) {
		m.SSH.AgentConfirm = false
		return m, nil
	}

	switch key {
	case "esc":
		m.Screen = ScreenSSHManager
		m.Cursor = 0
		m.SSH.Notice = ""
		m.SSH.AgentConfirm = false
		return m, nil
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		if m.Cursor < len(m.SSH.Agent.Identities) {
			return m.removeAgentIdentity(m.Cursor)
		}
		switch options[m.Cursor] {
		case sshRemoveEveryIdentityRow:
			if !m.SSH.AgentConfirm {
				m.SSH.AgentConfirm = true
				m.SSH.Notice = "Press the same row again to remove every identity from the agent: they are forgotten by the agent, not deleted from disk."
				return m, nil
			}
			m.SSH.AgentConfirm = false
			manager := m.SSH.Manager
			if manager == nil {
				return m, nil
			}
			m.SSH.Notice = "Removing every identity from the agent…"
			return m, sshAgentActionCmd(func() sshcfg.AgentAction {
				ctx, cancel := context.WithTimeout(context.Background(), sshManagerActionTimeout)
				defer cancel()
				return manager.RemoveAllFromAgent(ctx)
			}, "remove-all")
		case sshBackRow:
			m.Screen = ScreenSSHManager
			m.Cursor = 0
			m.SSH.Notice = ""
			m.SSH.AgentConfirm = false
		}
	}
	return m, nil
}

// removeAgentIdentity removes one identity. The agent prints fingerprints and
// ssh-add removes by public key file, so the identity is matched to a key on
// disk by its fingerprint; an identity the agent holds with no matching file is
// refused with that reason rather than by a command that cannot name it.
func (m Model) removeAgentIdentity(index int) (tea.Model, tea.Cmd) {
	if m.SSH.Manager == nil || index < 0 || index >= len(m.SSH.Agent.Identities) {
		return m, nil
	}
	identity := m.SSH.Agent.Identities[index]
	for _, key := range m.SSH.Keys {
		if key.Fingerprint != "" && key.Fingerprint == identity.Fingerprint && key.PubPath != "" {
			manager := m.SSH.Manager
			pubPath := key.PubPath
			m.SSH.Notice = "Removing the identity from the agent…"
			return m, sshAgentActionCmd(func() sshcfg.AgentAction {
				ctx, cancel := context.WithTimeout(context.Background(), sshManagerActionTimeout)
				defer cancel()
				return manager.RemoveFromAgent(ctx, pubPath)
			}, "remove")
		}
	}
	m.SSH.Notice = "No public key file on this machine matches that identity's fingerprint, and ssh-add removes an identity by its public key file. Nothing was removed."
	return m, nil
}

// ---------------------------------------------------------------------------
// The diagnosis screen
// ---------------------------------------------------------------------------

// handleSSHDiagnosticsKeys walks the report. Nothing here changes anything: the
// screen exists to be read, and its only key is the way back.
func (m Model) handleSSHDiagnosticsKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()
	if m.moveSSHRow(key, options) {
		return m, nil
	}
	switch key {
	case "esc":
		m.Screen = ScreenSSHManager
		m.Cursor = 0
		m.SSH.Notice = ""
		return m, nil
	case "enter", " ":
		if m.Cursor >= 0 && m.Cursor < len(options) && options[m.Cursor] == sshBackRow {
			m.Screen = ScreenSSHManager
			m.Cursor = 0
			m.SSH.Notice = ""
		}
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// The connection test
// ---------------------------------------------------------------------------

// openSSHTest opens the test screen with the origin Esc returns to.
func (m Model) openSSHTest(origin Screen) Model {
	m.Screen = ScreenSSHTest
	m.Cursor = 0
	m.SSH.TestOrigin = origin
	m.SSH.Notice = ""
	m.SSH.Tested = false
	m.SSH.Testing = false
	return m
}

// handleSSHTestKeys drives the test screen: the alias field takes text, the run
// row starts the test, and the cancel row stops the one in flight.
func (m Model) handleSSHTestKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()

	if m.moveSSHRow(key, options) {
		return m, nil
	}

	switch key {
	case "esc":
		return m.leaveSSHTest(), nil
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		switch options[m.Cursor] {
		case sshRunTestRow:
			return m.startSSHTest()
		case sshCancelTestRow:
			m.cancelSSHTest("The test was cancelled; nothing on the host was changed.")
			return m, nil
		case sshBackRow:
			return m.leaveSSHTest(), nil
		}
		return m, nil
	case "backspace":
		if !m.SSH.Testing {
			runes := []rune(m.SSH.TestAlias)
			if len(runes) > 0 {
				m.SSH.TestAlias = string(runes[:len(runes)-1])
			}
		}
		return m, nil
	default:
		if !m.SSH.Testing && sshTypingKey(key) {
			m.SSH.TestAlias += key
		}
	}
	return m, nil
}

// leaveSSHTest returns to the screen the test was opened from. A test in flight
// is cancelled first: an answer nobody is waiting for any more must not land on
// a screen the user has left.
func (m Model) leaveSSHTest() Model {
	if m.SSH.Testing && m.SSH.TestCancel != nil {
		m.SSH.TestCancel()
		m.SSH.TestCancel = nil
		m.SSH.Testing = false
	}
	if m.SSH.TestOrigin == ScreenSSHHost {
		m.Screen = ScreenSSHHost
	} else {
		m.Screen = ScreenSSHManager
	}
	m.Cursor = 0
	m.SSH.Notice = ""
	return m
}

// startSSHTest runs the test under a context the screen can cancel.
func (m Model) startSSHTest() (tea.Model, tea.Cmd) {
	alias := strings.TrimSpace(m.SSH.TestAlias)
	if m.SSH.Manager == nil || alias == "" || m.SSH.Testing {
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.SSH.TestCancel = cancel
	m.SSH.Testing = true
	m.SSH.Tested = false
	m.SSH.TestSeq++
	m.SSH.Notice = ""
	return m, sshTestCmd(m.SSH.Manager, ctx, m.SSH.TestSeq, alias)
}

// cancelSSHTest stops a test in flight and says so on the screen.
func (m *Model) cancelSSHTest(notice string) {
	if m.SSH.TestCancel != nil {
		m.SSH.TestCancel()
		m.SSH.TestCancel = nil
	}
	m.SSH.Testing = false
	m.SSH.Notice = notice
}
