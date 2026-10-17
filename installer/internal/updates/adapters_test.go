package updates

import (
	"context"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// The parsers
// ---------------------------------------------------------------------------
//
// Every adapter's answer is parsed by a pure function that takes the manager's own
// text. That is the whole reason these guards exist: a manager's output is the one
// thing this package cannot control, and a parser that guesses turns a warning into
// a package name.

// TestParseAptSimulationReadsOnlyWhatTheTransactionSays pins the apt parse: an
// upgrade is an `Inst` line with the version being replaced, a new install is not
// an upgrade, configuration lines are not entries, and the kept-back list is
// reported as what it is.
func TestParseAptSimulationReadsOnlyWhatTheTransactionSays(t *testing.T) {
	output := `Reading package lists...
Building dependency tree...
The following packages will be upgraded:
  curl git
The following packages have been kept back:
  libc6 linux-image-amd64
2 upgraded, 1 newly installed, 2 kept back, 0 to remove.
Inst curl [8.7.0] (8.7.1 Debian:13/stable [amd64])
Inst git [1:2.43.0-1] (1:2.44.0-1 Debian:13/stable [amd64])
Inst newtool (1.0 Debian:13/stable [amd64])
Conf curl (8.7.1 Debian:13/stable [amd64])
Conf git (1:2.44.0-1 Debian:13/stable [amd64])
`

	items, keptBack := ParseAptSimulation(output)
	if len(items) != 2 {
		t.Fatalf("an upgrade with a version in brackets is the only update here, got %+v", items)
	}
	if items[0].Name != "curl" || items[0].Current != "8.7.0" || items[0].Available != "8.7.1" {
		t.Fatalf("the first entry must carry apt's own versions, got %+v", items[0])
	}
	if items[1].Name != "git" || items[1].Current != "1:2.43.0-1" || items[1].Available != "1:2.44.0-1" {
		t.Fatalf("an epoch in a version must be kept as apt printed it, got %+v", items[1])
	}
	for _, item := range items {
		if item.Manager != ManagerApt || item.Scope != ScopeSystem || item.Kind != KindPackage {
			t.Fatalf("an apt entry is the machine's, of kind package, got %+v", item)
		}
	}
	if len(keptBack) != 2 || keptBack[0] != "libc6" || keptBack[1] != "linux-image-amd64" {
		t.Fatalf("apt's own kept-back list must be reported separately, got %+v", keptBack)
	}
}

// TestParseAptSimulationOfAnEmptyAnswer pins the answer with nothing in it: no
// entries and no kept-back list, which is what "up to date" looks like.
func TestParseAptSimulationOfAnEmptyAnswer(t *testing.T) {
	items, keptBack := ParseAptSimulation("Reading package lists...\n0 upgraded, 0 newly installed, 0 to remove and 0 not upgraded.\n")
	if len(items) != 0 || len(keptBack) != 0 {
		t.Fatalf("an empty simulation is an empty answer, got %+v and %+v", items, keptBack)
	}
}

// TestParsePacmanOutdatedReadsTheArrowAndTheIgnoreMarker pins the pacman parse,
// including the marker pacman itself appends to an ignored package.
func TestParsePacmanOutdatedReadsTheArrowAndTheIgnoreMarker(t *testing.T) {
	output := `curl 8.7.0-1 -> 8.7.1-1
git 2.43.0-1 -> 2.44.0-1 [ignored]
warning: ignoring package foo
`

	items := ParsePacmanOutdated(output, map[string]bool{"held-by-config": true})
	if len(items) != 2 {
		t.Fatalf("two rows carry an arrow and the warning does not, got %+v", items)
	}
	if items[0].Name != "curl" || items[0].Current != "8.7.0-1" || items[0].Available != "8.7.1-1" {
		t.Fatalf("the versions must be pacman's own, got %+v", items[0])
	}
	if items[0].Pinned {
		t.Fatal("curl is not ignored and must not read as pinned")
	}
	if !items[1].Pinned || items[1].PinReason == "" {
		t.Fatalf("pacman's own [ignored] marker must mark the entry, got %+v", items[1])
	}
	if items[1].Manager != ManagerPacman || items[1].Scope != ScopeSystem {
		t.Fatalf("a pacman entry is the machine's, got %+v", items[1])
	}
}

// TestParsePacmanOutdatedMarksTheConfiguredIgnoreList pins the other half of a
// hold: a package pacman is configured to ignore is marked, and the reason names
// the list.
func TestParsePacmanOutdatedMarksTheConfiguredIgnoreList(t *testing.T) {
	items := ParsePacmanOutdated("curl 8.7.0-1 -> 8.7.1-1\n", map[string]bool{"curl": true})
	if len(items) != 1 || !items[0].Pinned {
		t.Fatalf("a package pacman's IgnorePkg names must be marked, got %+v", items)
	}
	if !strings.Contains(items[0].PinReason, "IgnorePkg") {
		t.Fatalf("the reason must name the list, got %q", items[0].PinReason)
	}
}

// TestParseDnfCheckUpdateSkipsTheObsoletedSection pins the dnf parse: the
// obsoleted package's row names two packages and is not an update of either.
func TestParseDnfCheckUpdateSkipsTheObsoletedSection(t *testing.T) {
	output := `Last metadata expiration check: 0:12:00 ago.
curl.x86_64                        8.7.1-1.fc40             updates
git.x86_64                         2.44.0-1.fc40            updates

Obsoleting Packages
old-tool.x86_64                    1.0-1.fc40               @System
    new-tool.x86_64                2.0-1.fc40               updates
`

	items := ParseDnfCheckUpdate(output)
	if len(items) != 2 {
		t.Fatalf("only the two upgrade rows are updates, got %+v", items)
	}
	if items[0].Name != "curl" || items[0].Available != "8.7.1-1.fc40" {
		t.Fatalf("the name is the field before the dot and the version the second field, got %+v", items[0])
	}
	for _, item := range items {
		if item.Manager != ManagerDnf || item.Scope != ScopeSystem || item.Kind != KindPackage {
			t.Fatalf("a dnf entry is the machine's, got %+v", item)
		}
		if item.Current != "" {
			t.Fatalf("dnf's listing says nothing about the installed version, so it must stay empty, got %+v", item)
		}
	}
}

// TestParseRpmQueryJoinsInstalledVersions pins the second half of the dnf read:
// rpm's own answer about what is installed.
func TestParseRpmQueryJoinsInstalledVersions(t *testing.T) {
	versions := ParseRpmQuery("curl 8.7.0-1.fc40\ngit 2.43.0-1.fc40\npackage nosuch is not installed\n")
	if versions["curl"] != "8.7.0-1.fc40" || versions["git"] != "2.43.0-1.fc40" {
		t.Fatalf("rpm's rows must be read by name, got %+v", versions)
	}
	if _, ok := versions["package"]; ok {
		t.Fatalf("a refusal line is not a version, got %+v", versions)
	}
}

// TestParseSnapListingsSkipTheHeader pins snapd's two listings: the header is not a
// snap, and the refresh listing's rows are the updates.
func TestParseSnapListingsSkipTheHeader(t *testing.T) {
	installed := ParseSnapList("Name    Version   Rev    Tracking       Publisher   Notes\ncore20  20240416  2318   latest/stable  canonical✓  base\n")
	if installed["core20"] != "20240416" {
		t.Fatalf("the installed version must be read by name, got %+v", installed)
	}
	if _, ok := installed["Name"]; ok {
		t.Fatalf("the header is not a snap, got %+v", installed)
	}

	updates := ParseSnapRefreshList("Name    Version   Rev    Size    Publisher   Notes\ncore20  20240501  2400   63MB    canonical✓  held\nfirefox 126.0     4021   250MB   mozilla✓    -\n")
	if len(updates) != 2 {
		t.Fatalf("two snaps have a newer revision, got %+v", updates)
	}
	if updates[0].Name != "core20" || updates[0].Available != "20240501" {
		t.Fatalf("the refresh listing's second field is the available version, got %+v", updates[0])
	}
	if !updates[0].Pinned || !strings.Contains(updates[0].PinReason, "unhold") {
		t.Fatalf("a held snap must be marked with the way to release it, got %+v", updates[0])
	}
	if updates[1].Pinned {
		t.Fatalf("a dash in the notes is not a hold, got %+v", updates[1])
	}
}

// TestParseSnapHeldReadsTheHeldList pins the third snap listing, which exists only
// on newer snapd and therefore may be absent rather than empty.
func TestParseSnapHeldReadsTheHeldList(t *testing.T) {
	held := ParseSnapHeld("Name    Hold level  Expiry\ncore20  general     -\n")
	if !held["core20"] {
		t.Fatalf("the held snap must be read, got %+v", held)
	}
	if held["Name"] {
		t.Fatalf("the header is not a snap, got %+v", held)
	}
}

// TestParseFlatpakListingsJoinByApplicationAndBranch pins flatpak's two listings.
// The branch is part of the identity in flatpak's own model, so the join uses it.
func TestParseFlatpakListingsJoinByApplicationAndBranch(t *testing.T) {
	installations := ParseFlatpakList("org.gimp.GIMP\tstable\tsystem\norg.gnome.Calculator\tstable\tuser\n")
	if installations["org.gimp.GIMP\tstable"] != ScopeSystem {
		t.Fatalf("the system installation must be read as the machine's, got %+v", installations)
	}
	if installations["org.gnome.Calculator\tstable"] != ScopeUser {
		t.Fatalf("the user installation must be read as the user's, got %+v", installations)
	}

	updates := ParseFlatpakUpdates("org.gimp.GIMP\tstable\t2.10.40\n")
	if len(updates) != 1 || updates[0].App != "org.gimp.GIMP" || updates[0].Latest != "2.10.40" || updates[0].Branch != "stable" {
		t.Fatalf("the update row must carry application, branch and version, got %+v", updates)
	}
}

// TestParseRustupCheckTellsUpdatesFromUpToDateAndFromWhatItCannotRead pins
// rustup's three kinds of line: an update, an up-to-date toolchain, and a line
// this package does not understand, which is reported rather than assumed.
func TestParseRustupCheckTellsUpdatesFromUpToDateAndFromWhatItCannotRead(t *testing.T) {
	output := `stable-x86_64-unknown-linux-gnu - Up to date : 1.79.0
nightly-x86_64-unknown-linux-gnu - Update available : 1.81.0-nightly (2024-07-01) -> 1.81.0-nightly (2024-07-02)
something else entirely
`

	updates, unread := ParseRustupCheck(output)
	if len(updates) != 1 {
		t.Fatalf("one toolchain has an update, got %+v", updates)
	}
	if updates[0].Name != "nightly-x86_64-unknown-linux-gnu" ||
		updates[0].Current != "1.81.0-nightly (2024-07-01)" ||
		updates[0].Available != "1.81.0-nightly (2024-07-02)" {
		t.Fatalf("the toolchain and both versions must be read as rustup printed them, got %+v", updates[0])
	}
	if len(unread) != 1 || unread[0] != "something else entirely" {
		t.Fatalf("a line with neither shape must be reported as unread, got %+v", unread)
	}
}

// TestBrewEntriesKeepWhatHomebrewSaidAndSkipWhatItDidNot pins the JSON half of the
// Homebrew read: a pinned formula, a cask, a formula with two installed versions,
// and an entry with no version at all, which is skipped rather than shown as an
// update to nothing.
func TestBrewEntriesKeepWhatHomebrewSaidAndSkipWhatItDidNot(t *testing.T) {
	pinned, ok := brewItem(brewEntry{Name: "held", InstalledVersions: []string{"1.0"}, CurrentVersion: "1.1", Pinned: true}, KindFormula)
	if !ok || !pinned.Pinned || !strings.Contains(pinned.PinReason, "brew unpin held") {
		t.Fatalf("a pinned formula must carry the way to release it, got %+v", pinned)
	}
	multi, ok := brewItem(brewEntry{Name: "multi", InstalledVersions: []string{"1.0", "1.1"}, CurrentVersion: "1.2"}, KindFormula)
	if !ok || multi.Current != "1.1" {
		t.Fatalf("the newest installed version is the current one, got %+v", multi)
	}
	if !strings.Contains(multi.Note, "2 versions") {
		t.Fatalf("an entry with several installed versions must say so, got %q", multi.Note)
	}
	cask, ok := brewItem(brewEntry{Name: "app", InstalledVersions: []string{"0.1"}, Version: "0.2"}, KindCask)
	if !ok || cask.Available != "0.2" || cask.Kind != KindCask {
		t.Fatalf("a cask whose version field is spelled differently must still be read, got %+v", cask)
	}
	if _, ok := brewItem(brewEntry{Name: "empty"}, KindFormula); ok {
		t.Fatal("an entry with no version is not an update and must be skipped")
	}
}

// TestReadsThatCarryTheirOwnExitConvention pins the two managers whose exit codes
// are not a failure: dnf's 100 means it has something to report, and rustup's
// status is not the answer, so a parseable answer is taken whatever it is.
func TestReadsThatCarryTheirOwnExitConvention(t *testing.T) {
	dnf := &fakeRunner{answers: map[string]Output{
		QuoteArgv([]string{"dnf", "-q", "check-update"}): {
			ExitCode: 100,
			Err:      failed("").Err,
			Stdout:   "curl.x86_64 8.7.1-1.fc40 updates\n",
		},
		QuoteArgv([]string{"rpm", "-q", "--qf", "%{NAME} %{VERSION}-%{RELEASE}\n", "curl"}): ok("curl 8.7.0-1.fc40\n"),
		QuoteArgv([]string{"dnf", "-q", "versionlock", "list"}):                             failed("Unknown command: versionlock\n"),
	}}
	center := &Center{Adapters: DefaultAdapters(), Runner: dnf, Env: testEnv("dnf", "rpm")}
	result := center.Read(context.Background(), ManagerDnf)
	if result.Status != StatusUpdates {
		t.Fatalf("dnf's 100 is an answer and must be read as one, got %s (%s)", result.Status, result.Reason)
	}
	if len(result.Items) != 1 || result.Items[0].Current != "8.7.0-1.fc40" || result.Items[0].Available != "8.7.1-1.fc40" {
		t.Fatalf("the entry must carry both versions, got %+v", result.Items)
	}

	rustup := &fakeRunner{answers: map[string]Output{
		QuoteArgv([]string{"rustup", "check"}): {
			ExitCode: 1,
			Err:      failed("").Err,
			Stdout:   "stable-x86_64-unknown-linux-gnu - Update available : 1.79.0 -> 1.80.0\n",
		},
	}}
	center = &Center{Adapters: DefaultAdapters(), Runner: rustup, Env: testEnv("rustup")}
	result = center.Read(context.Background(), ManagerRustup)
	if result.Status != StatusUpdates || len(result.Items) != 1 {
		t.Fatalf("a parseable rustup answer is the answer whatever its status, got %s (%s) and %+v",
			result.Status, result.Reason, result.Items)
	}
	if result.Items[0].Kind != KindToolchain || result.Items[0].Scope != ScopeUser {
		t.Fatalf("a toolchain is the user's own entry, got %+v", result.Items[0])
	}
}

// TestAnAnswerWithACredentialNeverKeepsItInAReasonOrANote pins the redaction on
// the paths that carry a manager's words into the interface.
func TestAnAnswerWithACredentialNeverKeepsItInAReasonOrANote(t *testing.T) {
	runner := &fakeRunner{answers: map[string]Output{
		QuoteArgv([]string{"flatpak", "remote-ls", "--updates", "--columns=application,branch,version"}): failed(
			"error: Failed to fetch https://user:hunter2@example.com/repo.flatpakrepo: Could not resolve host: example.com\n"),
	}}
	center := &Center{Adapters: DefaultAdapters(), Runner: runner, Env: testEnv("flatpak")}
	result := center.Read(context.Background(), ManagerFlatpak)
	if strings.Contains(result.Reason, "hunter2") {
		t.Fatalf("a reason must not carry a credential: %q", result.Reason)
	}
	if result.Status != StatusOffline {
		t.Fatalf("flatpak's failure names a host that cannot be resolved, got %s", result.Status)
	}
}

// ---------------------------------------------------------------------------
// Probes and plans, one adapter at a time
// ---------------------------------------------------------------------------

// TestEachAdaptersProbeAnswersWithTheReason pins the probe of every adapter on the
// hosts this repository supports. A missing program is unavailable and a host the
// manager does not serve is unsupported; both carry a reason, and neither is an
// error.
func TestEachAdaptersProbeAnswersWithTheReason(t *testing.T) {
	tests := []struct {
		name       string
		adapter    Adapter
		env        Env
		wantAvail  bool
		wantStatus Status
	}{
		{"brew is present", BrewAdapter{}, testEnv("brew"), true, StatusUpdates},
		{"brew is missing", BrewAdapter{}, testEnv(), false, StatusUnavailable},
		{"brew on a BSD", BrewAdapter{}, Env{GOOS: "freebsd"}, false, StatusUnsupported},
		{"apt is present", AptAdapter{}, testEnv("apt-get"), true, StatusUpdates},
		{"apt is missing", AptAdapter{}, testEnv(), false, StatusUnavailable},
		{"apt on Termux", AptAdapter{}, Env{GOOS: "linux", Termux: true, LookPath: testEnv("apt-get").LookPath}, false, StatusUnsupported},
		{"apt on macOS", AptAdapter{}, Env{GOOS: "darwin", LookPath: testEnv("apt-get").LookPath}, false, StatusUnsupported},
		{"pacman is present", PacmanAdapter{}, testEnv("pacman"), true, StatusUpdates},
		{"pacman is missing", PacmanAdapter{}, testEnv(), false, StatusUnavailable},
		{"dnf is present", DnfAdapter{}, testEnv("dnf"), true, StatusUpdates},
		{"dnf is missing", DnfAdapter{}, testEnv(), false, StatusUnavailable},
		{"pkg on a Termux host", PkgAdapter{}, Env{GOOS: "linux", Termux: true, LookPath: testEnv("pkg", "apt-get").LookPath}, true, StatusUpdates},
		{"pkg on a Debian host", PkgAdapter{}, testEnv("pkg", "apt-get"), false, StatusUnsupported},
		{"pkg on Termux without its backend", PkgAdapter{}, Env{GOOS: "linux", Termux: true, LookPath: testEnv("pkg").LookPath}, false, StatusUnavailable},
		{"snap is present", SnapAdapter{}, testEnv("snap"), true, StatusUpdates},
		{"snap is missing", SnapAdapter{}, testEnv(), false, StatusUnavailable},
		{"flatpak is present", FlatpakAdapter{}, testEnv("flatpak"), true, StatusUpdates},
		{"flatpak is missing", FlatpakAdapter{}, testEnv(), false, StatusUnavailable},
		{"rustup is present", RustupAdapter{}, testEnv("rustup"), true, StatusUpdates},
		{"rustup is missing", RustupAdapter{}, testEnv(), false, StatusUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			availability := tt.adapter.Probe(tt.env)
			if availability.Available != tt.wantAvail {
				t.Fatalf("available = %v, want %v (%s)", availability.Available, tt.wantAvail, availability.Reason)
			}
			if !tt.wantAvail && availability.Reason == "" {
				t.Fatal("an unavailable or unsupported manager must carry its reason")
			}
			if tt.wantStatus != StatusUpdates && availability.Status != tt.wantStatus {
				t.Fatalf("status = %s, want %s", availability.Status, tt.wantStatus)
			}
		})
	}
}

// TestEachAdaptersPlanIsItsOwnManagersCommand pins every manager's update command:
// the program the machine owns, the flags that keep it non-interactive, and the
// one kind it carries.
func TestEachAdaptersPlanIsItsOwnManagersCommand(t *testing.T) {
	tests := []struct {
		name         string
		env          Env
		item         Item
		wantContains []string
		wantElevated bool
	}{
		{
			name: "brew formula", env: testEnv("brew"),
			item:         Item{Manager: ManagerBrew, Name: "git", Scope: ScopeUser, Kind: KindFormula},
			wantContains: []string{"brew upgrade git"},
		},
		{
			name: "apt package", env: testEnv("apt-get"),
			item:         Item{Manager: ManagerApt, Name: "curl", Scope: ScopeSystem, Kind: KindPackage},
			wantContains: []string{"apt-get install", "-y", "--only-upgrade", "curl"}, wantElevated: true,
		},
		{
			name: "pacman package", env: testEnv("pacman"),
			item:         Item{Manager: ManagerPacman, Name: "curl", Scope: ScopeSystem, Kind: KindPackage},
			wantContains: []string{"pacman -S", "--noconfirm", "--needed", "curl"}, wantElevated: true,
		},
		{
			name: "dnf package", env: testEnv("dnf"),
			item:         Item{Manager: ManagerDnf, Name: "curl", Scope: ScopeSystem, Kind: KindPackage},
			wantContains: []string{"dnf -y upgrade curl"}, wantElevated: true,
		},
		{
			name: "pkg package", env: Env{GOOS: "linux", Termux: true, LookPath: testEnv("pkg").LookPath},
			item:         Item{Manager: ManagerPkg, Name: "bash", Scope: ScopeUser, Kind: KindPackage},
			wantContains: []string{"pkg install", "-y", "bash"},
		},
		{
			name: "snap", env: testEnv("snap"),
			item:         Item{Manager: ManagerSnap, Name: "firefox", Scope: ScopeSystem, Kind: KindSnap},
			wantContains: []string{"snap refresh firefox"}, wantElevated: true,
		},
		{
			name: "flatpak user installation", env: testEnv("flatpak"),
			item:         Item{Manager: ManagerFlatpak, Name: "org.gimp.GIMP", Scope: ScopeUser, Kind: KindFlatpak},
			wantContains: []string{"flatpak update", "--noninteractive", "-y", "--user", "org.gimp.GIMP"},
		},
		{
			name: "flatpak system installation", env: testEnv("flatpak"),
			item:         Item{Manager: ManagerFlatpak, Name: "org.gimp.GIMP", Scope: ScopeSystem, Kind: KindFlatpak},
			wantContains: []string{"flatpak update", "--system", "org.gimp.GIMP"}, wantElevated: true,
		},
		{
			name: "rustup toolchain", env: testEnv("rustup"),
			item:         Item{Manager: ManagerRustup, Name: "stable-x86_64-unknown-linux-gnu", Scope: ScopeUser, Kind: KindToolchain},
			wantContains: []string{"rustup update stable-x86_64-unknown-linux-gnu"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, ok := (&Center{Adapters: DefaultAdapters(), Env: tt.env}).Adapter(tt.item.Manager)
			if !ok {
				t.Fatalf("no adapter owns %s", tt.item.Manager)
			}
			plan, err := adapter.Plan([]Item{tt.item}, tt.env)
			if err != nil {
				t.Fatalf("the plan refused: %v", err)
			}
			display := plan.Display()
			for _, want := range tt.wantContains {
				if !strings.Contains(display, want) {
					t.Fatalf("the command must carry %q, got %q", want, display)
				}
			}
			if plan.Elevation() != tt.wantElevated {
				t.Fatalf("elevation = %v, want %v (%q)", plan.Elevation(), tt.wantElevated, display)
			}
			if !strings.Contains(display, tt.item.Name) {
				t.Fatalf("the command must name the selected entry, got %q", display)
			}
			if plan.Restart == "" || plan.Rollback == "" || len(plan.Notes) == 0 {
				t.Fatalf("every plan states what stays pending, what going back costs and what else the manager does: %+v", plan)
			}
			// The command is an argv, and the program it starts is the manager's.
			if plan.Command.Argv[0] == "sudo" {
				t.Fatal("elevation is a field of the command, never a part of its argv")
			}
		})
	}
}

// TestAPinnedEntryIsRefusedByEveryAdapterThatCanHoldOne pins the hold rule at the
// adapter boundary: whatever built the command, it refuses to move a held entry.
func TestAPinnedEntryIsRefusedByEveryAdapterThatCanHoldOne(t *testing.T) {
	tests := []struct {
		name string
		env  Env
		item Item
	}{
		{"brew", testEnv("brew"), Item{Manager: ManagerBrew, Name: "git", Scope: ScopeUser, Kind: KindFormula, Pinned: true, PinReason: "pinned with `brew pin`"}},
		{"apt", testEnv("apt-get"), Item{Manager: ManagerApt, Name: "curl", Scope: ScopeSystem, Kind: KindPackage, Pinned: true, PinReason: "held with `apt-mark hold`"}},
		{"pacman", testEnv("pacman"), Item{Manager: ManagerPacman, Name: "curl", Scope: ScopeSystem, Kind: KindPackage, Pinned: true, PinReason: "listed in pacman's IgnorePkg"}},
		{"dnf", testEnv("dnf"), Item{Manager: ManagerDnf, Name: "curl", Scope: ScopeSystem, Kind: KindPackage, Pinned: true, PinReason: "locked with dnf's versionlock plugin"}},
		{"pkg", Env{GOOS: "linux", Termux: true, LookPath: testEnv("pkg").LookPath}, Item{Manager: ManagerPkg, Name: "bash", Scope: ScopeUser, Kind: KindPackage, Pinned: true, PinReason: "held with `apt-mark hold`"}},
		{"snap", testEnv("snap"), Item{Manager: ManagerSnap, Name: "firefox", Scope: ScopeSystem, Kind: KindSnap, Pinned: true, PinReason: "held by snapd"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, _ := (&Center{Adapters: DefaultAdapters(), Env: tt.env}).Adapter(tt.item.Manager)
			plan, err := adapter.Plan([]Item{tt.item}, tt.env)
			if err == nil {
				t.Fatalf("a pinned entry must be refused, got %q", plan.Display())
			}
			if !strings.Contains(err.Error(), tt.item.Name) {
				t.Fatalf("the refusal must name the entry, got %q", err)
			}
		})
	}
}

// TestAPlanRefusesADifferentEntryOfTheSameManager pins the other refusal at the
// adapter boundary: one command carries one kind and one installation, so a
// mixed group is refused rather than silently split.
func TestAPlanRefusesADifferentEntryOfTheSameManager(t *testing.T) {
	env := testEnv("brew")
	plan, err := BrewAdapter{}.Plan([]Item{
		{Manager: ManagerBrew, Name: "git", Kind: KindFormula},
		{Manager: ManagerBrew, Name: "alacritty", Kind: KindCask},
	}, env)
	if err == nil {
		t.Fatalf("a formula and a cask need two commands, got %q", plan.Display())
	}

	flatpakEnv := testEnv("flatpak")
	if _, err := (FlatpakAdapter{}).Plan([]Item{
		{Manager: ManagerFlatpak, Name: "org.example.User", Scope: ScopeUser, Kind: KindFlatpak},
		{Manager: ManagerFlatpak, Name: "org.example.System", Scope: ScopeSystem, Kind: KindFlatpak},
	}, flatpakEnv); err == nil {
		t.Fatal("two installations of one manager need two commands")
	}
}
