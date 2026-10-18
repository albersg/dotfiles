package tui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// A MACHINE OF THE TEST'S OWN
// ============================================================================
//
// Every doctor test runs against doctorFakeHost: the environment, the
// filesystem, the PATH, the commands and the two things a fix can change are all
// in memory. The rules the brief cares about are exactly the ones a real machine
// reaches only by accident -- a dangling symlink, a directory nobody can read, a
// unicode path with spaces, a command that never answers -- and a test that
// reached them by accident would depend on the developer's own machine and would
// read it while doing so.

// doctorFakeInfo is one entry of the fake filesystem.
type doctorFakeInfo struct {
	name    string
	mode    os.FileMode
	size    int64
	dir     bool
	modTime time.Time
}

func (i doctorFakeInfo) Name() string       { return i.name }
func (i doctorFakeInfo) Size() int64        { return i.size }
func (i doctorFakeInfo) Mode() os.FileMode  { return i.mode }
func (i doctorFakeInfo) ModTime() time.Time { return i.modTime }
func (i doctorFakeInfo) IsDir() bool        { return i.dir }
func (i doctorFakeInfo) Sys() any           { return nil }

// doctorFakeHost is the machine: what the environment says, what the filesystem
// holds, what each command answers, and what the doctor changed.
type doctorFakeHost struct {
	env      map[string]string
	files    map[string]doctorFakeInfo
	data     map[string]string
	targets  map[string]string
	commands map[string]doctorCommandResult
	goos     string
	homeErr  error

	// Recorded effects. A scan must not write anything at all, and a fix must
	// write exactly one thing: these three slices are what proves it.
	commandsRun []string
	mutations   []string
	reads       []string
}

func newDoctorFakeHost() *doctorFakeHost {
	return &doctorFakeHost{
		env: map[string]string{
			"HOME":  "/home/tester",
			"PATH":  "/usr/local/bin:/usr/bin:/bin",
			"SHELL": "/bin/zsh",
			"LANG":  "en_US.UTF-8",
			"TERM":  "xterm-256color",
		},
		files:    map[string]doctorFakeInfo{},
		data:     map[string]string{},
		targets:  map[string]string{},
		commands: map[string]doctorCommandResult{},
		goos:     "linux",
	}
}

func (h *doctorFakeHost) setenv(name, value string) *doctorFakeHost {
	h.env[name] = value
	return h
}

func (h *doctorFakeHost) unsetenv(name string) *doctorFakeHost {
	delete(h.env, name)
	return h
}

// addDir adds a directory, creating nothing above it: a path whose parent is
// missing is exactly the state the checks have to report.
func (h *doctorFakeHost) addDir(path string) *doctorFakeHost {
	return h.addFile(path, 0o755, true, "")
}

func (h *doctorFakeHost) addFile(path string, mode os.FileMode, dir bool, content string) *doctorFakeHost {
	h.files[path] = doctorFakeInfo{
		name:    filepath.Base(path),
		mode:    mode,
		size:    int64(len(content)),
		dir:     dir,
		modTime: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	if content != "" {
		h.data[path] = content
	}
	return h
}

func (h *doctorFakeHost) addText(path string, mode os.FileMode, content string) *doctorFakeHost {
	return h.addFile(path, mode, false, content)
}

func (h *doctorFakeHost) addLink(path, target string) *doctorFakeHost {
	h.files[path] = doctorFakeInfo{name: filepath.Base(path), mode: os.ModeSymlink | 0o777}
	h.targets[path] = target
	return h
}

func (h *doctorFakeHost) addCommand(line, stdout string) *doctorFakeHost {
	h.commands[line] = doctorCommandResult{Stdout: stdout}
	return h
}

func (h *doctorFakeHost) addCommandResult(line string, result doctorCommandResult) *doctorFakeHost {
	h.commands[line] = result
	return h
}

func (h *doctorFakeHost) Getenv(name string) string { return h.env[name] }

func (h *doctorFakeHost) Environ() []string {
	env := make([]string, 0, len(h.env))
	for name, value := range h.env {
		env = append(env, name+"="+value)
	}
	sort.Strings(env)
	return env
}

func (h *doctorFakeHost) LookPath(name string) (string, error) {
	// A name with a separator in it is looked up as a path, the way
	// exec.LookPath does: $SHELL is usually an absolute path.
	if strings.ContainsRune(name, filepath.Separator) {
		if info, ok := h.files[filepath.Clean(name)]; ok && !info.dir && info.mode&0o111 != 0 {
			return filepath.Clean(name), nil
		}
		return "", &exec.Error{Name: name, Err: fs.ErrNotExist}
	}
	for _, entry := range filepath.SplitList(h.env["PATH"]) {
		candidate := filepath.Join(entry, name)
		info, ok := h.files[candidate]
		if !ok || info.dir || info.mode&0o111 == 0 {
			continue
		}
		return candidate, nil
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

// resolve follows a symbolic link one step at a time, bounded, the way the
// kernel does: a link to a link is followed, and a loop stops.
func (h *doctorFakeHost) resolve(path string) (string, bool) {
	for hops := 0; hops < 8; hops++ {
		target, ok := h.targets[path]
		if !ok {
			return path, true
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = filepath.Clean(target)
	}
	return path, false
}

func (h *doctorFakeHost) lookup(path string, follow bool) (os.FileInfo, error) {
	h.reads = append(h.reads, path)
	if info, ok := h.files[path]; ok {
		if info.mode&os.ModeSymlink == 0 {
			return info, nil
		}
		if !follow {
			return info, nil
		}
		resolved, ok := h.resolve(path)
		if !ok {
			return nil, &fs.PathError{Op: "stat", Path: path, Err: errors.New("too many levels of symbolic links")}
		}
		if info, ok := h.files[resolved]; ok {
			return info, nil
		}
		return nil, &fs.PathError{Op: "stat", Path: resolved, Err: fs.ErrNotExist}
	}
	return nil, &fs.PathError{Op: "stat", Path: path, Err: fs.ErrNotExist}
}

func (h *doctorFakeHost) Lstat(path string) (os.FileInfo, error) {
	return h.lookup(filepath.Clean(path), false)
}

func (h *doctorFakeHost) Stat(path string) (os.FileInfo, error) {
	return h.lookup(filepath.Clean(path), true)
}

func (h *doctorFakeHost) ReadDir(path string) ([]os.DirEntry, error) {
	path = filepath.Clean(path)
	info, err := h.lookup(path, true)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &fs.PathError{Op: "readdir", Path: path, Err: errors.New("not a directory")}
	}
	var entries []os.DirEntry
	for candidate, candidateInfo := range h.files {
		if filepath.Dir(candidate) != path {
			continue
		}
		entries = append(entries, fs.FileInfoToDirEntry(candidateInfo))
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

func (h *doctorFakeHost) ReadFile(path string) ([]byte, error) {
	h.reads = append(h.reads, path)
	content, ok := h.data[filepath.Clean(path)]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}
	return []byte(content), nil
}

func (h *doctorFakeHost) Readlink(path string) (string, error) {
	h.reads = append(h.reads, path)
	target, ok := h.targets[filepath.Clean(path)]
	if !ok {
		return "", &fs.PathError{Op: "readlink", Path: path, Err: errors.New("invalid argument")}
	}
	return target, nil
}

func (h *doctorFakeHost) UserHomeDir() (string, error) {
	if h.homeErr != nil {
		return "", h.homeErr
	}
	if home := h.env["HOME"]; home != "" {
		return home, nil
	}
	return "", errors.New("no home directory")
}

func (h *doctorFakeHost) GOOS() string { return h.goos }

func (h *doctorFakeHost) Run(_ context.Context, name string, args ...string) doctorCommandResult {
	line := name
	if len(args) > 0 {
		line += " " + strings.Join(args, " ")
	}
	h.commandsRun = append(h.commandsRun, line)
	if result, ok := h.commands[line]; ok {
		return result
	}
	return doctorCommandResult{Code: -1, Err: "the command was not found"}
}

func (h *doctorFakeHost) MkdirAll(path string, perm os.FileMode) error {
	h.mutations = append(h.mutations, "mkdir "+path)
	path = filepath.Clean(path)
	var missing []string
	for candidate := path; candidate != "/" && candidate != "."; candidate = filepath.Dir(candidate) {
		if _, ok := h.files[candidate]; ok {
			break
		}
		missing = append(missing, candidate)
	}
	for index := len(missing) - 1; index >= 0; index-- {
		h.addFile(missing[index], perm, true, "")
	}
	return nil
}

func (h *doctorFakeHost) Chmod(path string, mode os.FileMode) error {
	h.mutations = append(h.mutations, fmt.Sprintf("chmod %s %04o", path, mode.Perm()))
	path = filepath.Clean(path)
	info, ok := h.files[path]
	if !ok {
		return &fs.PathError{Op: "chmod", Path: path, Err: fs.ErrNotExist}
	}
	info.mode = (info.mode &^ os.ModePerm) | mode.Perm()
	h.files[path] = info
	return nil
}

func (h *doctorFakeHost) WriteFile(path string, data []byte, perm os.FileMode) error {
	h.mutations = append(h.mutations, "write "+path)
	h.addText(filepath.Clean(path), perm, string(data))
	return nil
}

// ============================================================================
// Helpers
// ============================================================================

// doctorTestInfo is the platform detection the checks read. Most tests use the
// Linux entry; the platform-specific ones replace it.
func doctorTestLinuxInfo() *system.SystemInfo {
	return &system.SystemInfo{OS: system.OSLinux, OSName: "Linux", Arch: "x86_64", HomeDir: "/home/tester", UserShell: "/bin/zsh"}
}

func doctorTestScan(host doctorHost, info *system.SystemInfo) *doctorScan {
	return newDoctorScan(host, info, time.Date(2026, time.January, 4, 12, 0, 0, 0, time.UTC))
}

// doctorCheckForTest finds a check by ID, failing by name when the catalogue
// does not hold it: a renamed check must break its own test loudly.
func doctorCheckForTest(t *testing.T, id string) doctorCheck {
	t.Helper()
	check, ok := doctorCheckByID(id)
	if !ok {
		t.Fatalf("the catalogue no longer holds the check %q", id)
	}
	return check
}

// doctorCheckFindings runs one check and returns its findings.
func doctorCheckFindings(t *testing.T, host doctorHost, info *system.SystemInfo, id string) []doctorFinding {
	t.Helper()
	result := runDoctorCheck(doctorTestScan(host, info), doctorCheckForTest(t, id))
	return result.Findings
}

// doctorWorst is the state a set of findings has to be reported as. It starts
// from the first finding rather than from ok, so a set that holds only a
// not-applicable answer does not read as ok.
func doctorWorst(findings []doctorFinding) doctorStatus {
	if len(findings) == 0 {
		return doctorUnknown
	}
	status := findings[0].Status
	for _, finding := range findings {
		status = worseDoctorStatus(status, finding.Status)
	}
	return status
}

// doctorFindingWith is the first finding whose subject or evidence mentions the
// text, failing by name when there is none.
func doctorFindingWith(t *testing.T, findings []doctorFinding, text string) doctorFinding {
	t.Helper()
	for _, finding := range findings {
		if strings.Contains(finding.Subject, text) || strings.Contains(finding.Evidence, text) {
			return finding
		}
	}
	t.Fatalf("no finding mentions %q; the findings are:\n%s", text, doctorFindingSummary(findings))
	return doctorFinding{}
}

// doctorNoFindingWith fails when any finding mentions the text.
func doctorNoFindingWith(t *testing.T, findings []doctorFinding, text string) {
	t.Helper()
	for _, finding := range findings {
		if strings.Contains(finding.Subject, text) || strings.Contains(finding.Evidence, text) {
			t.Errorf("a finding mentions %q, which it must not:\n%s", text, doctorFindingSummary(findings))
			return
		}
	}
}

func doctorFindingSummary(findings []doctorFinding) string {
	var lines []string
	for _, finding := range findings {
		lines = append(lines, fmt.Sprintf("  [%s] %s — %s", finding.Status, finding.Subject, finding.Evidence))
	}
	return strings.Join(lines, "\n")
}

// doctorRunEveryCheck runs the whole catalogue over one machine and returns the
// results, for the guards that have to look at all of them at once.
func doctorRunEveryCheck(t *testing.T, host doctorHost, info *system.SystemInfo) []doctorCheckResult {
	t.Helper()
	scan := doctorTestScan(host, info)
	var results []doctorCheckResult
	for _, check := range doctorChecks() {
		results = append(results, runDoctorCheck(scan, check))
	}
	return results
}

// doctorRichHost is a machine with something wrong in every category: a PATH
// with a duplicate and a missing directory, a dangling link among the dotfiles,
// a world-writable key, secret-looking variables and a command that never
// answers. It is what the guards run the whole catalogue against.
func doctorRichHost() *doctorFakeHost {
	h := newDoctorFakeHost()
	h.setenv("PATH", "/usr/local/bin:/usr/bin:/usr/bin:/nowhere/bin:~/bin")
	h.setenv("SSH_AUTH_SOCK", "/tmp/agent.sock")
	h.setenv("GITHUB_TOKEN", "ghp_do_not_print_me_0123456789")
	h.setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI-do-not-print")
	h.setenv("DOTFILES_ANIM", "maybe")
	h.setenv("LC_ALL", "C")

	h.addDir("/home/tester")
	h.addDir("/home/tester/.ssh")
	h.files["/home/tester/.ssh"] = doctorFakeInfo{name: ".ssh", mode: 0o755, dir: true}
	h.addText("/home/tester/.ssh/id_ed25519", 0o644, "-----BEGIN OPENSSH PRIVATE KEY-----\nAAAA\n-----END OPENSSH PRIVATE KEY-----\n")
	h.addText("/home/tester/.ssh/config", 0o600, "Host *\n  AddKeysToAgent yes\n")
	h.addText("/home/tester/.zshrc", 0o644, "source ~/.does-not-exist\nalias ll='eza -l'\n")
	h.addText("/home/tester/.config/fish/config.fish", 0o644, "alias ll 'eza -l'\n")
	h.addLink("/home/tester/.p10k.zsh", "/home/tester/gone/p10k.zsh")
	h.addText("/home/tester/.git-credentials", 0o644, "https://user:token@example.com\n")
	h.addDir("/home/tester/.local")
	h.addDir("/home/tester/.config")
	h.addDir("/home/tester/.config/nvim")
	h.addText("/home/tester/.config/nvim/init.lua", 0o666, "-- writable by anyone\n")

	h.addCommand("/usr/bin/git --version", "git version 2.44.0")
	h.addCommand("/usr/bin/git config --global --get user.name", "Tester\n")
	h.addCommand("/usr/bin/git config --global --get user.email", "tester@example.com\n")
	h.addCommandResult("/usr/bin/git config --global --get credential.helper", doctorCommandResult{Code: 1, Err: "exited with status 1"})
	h.addCommand("/usr/bin/git rev-parse --show-toplevel", "/home/tester/project\n")
	h.addCommand("/usr/bin/git -C /home/tester/project config --local --get user.name", "Tester At Work\n")
	h.addCommand("/usr/bin/git -C /home/tester/project config --local --get user.email", "tester@work.example\n")
	h.addCommandResult("/bin/zsh -n /home/tester/.zshrc", doctorCommandResult{Code: 1, Err: "exited with status 1", Stderr: "/home/tester/.zshrc:3: parse error near `\\n'\n"})
	h.addCommandResult("/bin/zsh -l -c printf %s \"$PATH\"", doctorCommandResult{TimedOut: true, Err: "the command did not finish inside its bound", Code: -1})
	h.addCommand("/usr/bin/ssh-add -l", "256 SHA256:aaa tester@example.com (ED25519)\n")
	return h
}

// ============================================================================
// The catalogue, the states and the report
// ============================================================================

// TestDoctorCheckCatalogueIsWellFormed is the catalogue's own guard: unique IDs,
// a category the section offers, a title, a run function, and at least one check
// per category. A check added without one of these fails here rather than on the
// screen of whoever opens the doctor next.
func TestDoctorCheckCatalogueIsWellFormed(t *testing.T) {
	checks := doctorChecks()
	if len(checks) == 0 {
		t.Fatal("the catalogue is empty, so this guard proves nothing")
	}

	seen := map[string]bool{}
	perCategory := map[doctorCategory]int{}
	for _, check := range checks {
		if check.ID == "" {
			t.Errorf("a check has no ID: %+v", check)
			continue
		}
		if seen[check.ID] {
			t.Errorf("the ID %q is used by two checks", check.ID)
		}
		seen[check.ID] = true
		if check.Title == "" {
			t.Errorf("the check %q has no title, so the progress line cannot name it", check.ID)
		}
		if check.Run == nil {
			t.Errorf("the check %q has no run function", check.ID)
		}
		perCategory[check.Category]++
	}

	for _, category := range doctorCategories() {
		if perCategory[category] == 0 {
			t.Errorf("the category %q has no check, so the section offers a row that runs nothing", category)
		}
		total := 0
		for _, check := range doctorChecks() {
			if check.Category == category {
				total++
			}
		}
		if total != perCategory[category] {
			t.Errorf("the category %q has %d checks in the section's order and %d in the catalogue", category, total, perCategory[category])
		}
	}
	t.Logf("%d checks over %d categories", len(checks), len(doctorCategories()))
}

// TestEveryFindingCarriesEvidenceImpactAndAdvice runs the whole catalogue over a
// machine with something wrong everywhere and over a bare one, and requires every
// finding to state what was read, what it means and what to do. A finding with no
// evidence is an opinion, and this is the guard that fails it by name.
func TestEveryFindingCarriesEvidenceImpactAndAdvice(t *testing.T) {
	machines := []struct {
		name string
		host doctorHost
	}{
		{"a machine with something wrong in every category", doctorRichHost()},
		{"a bare machine", newDoctorFakeHost().unsetenv("HOME").unsetenv("PATH").unsetenv("SHELL")},
	}

	for _, machine := range machines {
		machine := machine
		t.Run(machine.name, func(t *testing.T) {
			for _, result := range doctorRunEveryCheck(t, machine.host, doctorTestLinuxInfo()) {
				if len(result.Findings) == 0 {
					t.Errorf("the check %q produced no finding, and a check that produced nothing did not establish anything", result.ID)
					continue
				}
				for _, finding := range result.Findings {
					where := fmt.Sprintf("%s / %s", result.ID, finding.Subject)
					if strings.TrimSpace(finding.Subject) == "" {
						t.Errorf("%s: the finding has no subject", result.ID)
					}
					if strings.TrimSpace(finding.Evidence) == "" {
						t.Errorf("%s: the finding has no evidence", where)
					}
					if strings.TrimSpace(finding.Impact) == "" {
						t.Errorf("%s: the finding has no impact", where)
					}
					if strings.TrimSpace(finding.Advice) == "" {
						t.Errorf("%s: the finding has no advice, so a reader has nothing to do with it", where)
					}
					if finding.Fix != nil && finding.Status != doctorWarning && finding.Status != doctorError {
						t.Errorf("%s: a fix is offered for a finding in state %s, and only a warning or an error may be repaired", where, finding.Status)
					}
				}
			}
		})
	}
}

// TestUnknownIsItsOwnStateNotAnInventedOk is the five-state guard. A check that
// did not run reports unknown with the reason, and unknown is the zero value, so
// a result nobody filled in cannot read as ok.
func TestUnknownIsItsOwnStateNotAnInventedOk(t *testing.T) {
	check := doctorCheckForTest(t, "path.entries")
	result := notRunResult(check, "the run was cancelled before this check ran")

	if result.Status != doctorUnknown {
		t.Errorf("a check that did not run reports %q, want %q", result.Status, doctorUnknown)
	}
	if len(result.Findings) != 1 || result.Findings[0].Status != doctorUnknown {
		t.Fatalf("a check that did not run must carry one unknown finding, got %s", doctorFindingSummary(result.Findings))
	}
	if !strings.Contains(result.Findings[0].Evidence, "cancelled") {
		t.Errorf("the unknown finding does not carry the reason it did not run: %q", result.Findings[0].Evidence)
	}

	// The zero value of the status is unknown, and the aggregation order puts it
	// above ok: an answer nobody established may never be folded into a clean
	// result.
	var zero doctorStatus
	if zero != doctorUnknown {
		t.Errorf("the zero status is %q, want unknown", zero)
	}
	if got := worseDoctorStatus(doctorOK, doctorUnknown); got != doctorUnknown {
		t.Errorf("ok aggregated with unknown is %q, want unknown", got)
	}
	if got := worseDoctorStatus(doctorUnknown, doctorError); got != doctorError {
		t.Errorf("unknown aggregated with an error is %q, want the error", got)
	}

	// The five states are five different words, which is what makes the screen
	// readable without colour.
	labels := map[string]doctorStatus{}
	for _, status := range []doctorStatus{doctorOK, doctorWarning, doctorError, doctorNotApplicable, doctorUnknown} {
		label := status.label()
		if previous, ok := labels[label]; ok {
			t.Errorf("%q and %q share the label %q, so the screen cannot tell them apart", previous, status, label)
		}
		labels[label] = status
		if status.mark() == "" {
			t.Errorf("the state %q has no mark", status)
		}
	}
}

// TestEveryDeterministicFixIsPreviewedAppliedAndVerified is the fix catalogue's
// own guard, and the brief's cycle in one test: for each fix the doctor can
// offer, the preview states the effect, the apply makes one change, and the
// verification reads the result back.
func TestEveryDeterministicFixIsPreviewedAppliedAndVerified(t *testing.T) {
	cases := []struct {
		name    string
		host    *doctorFakeHost
		fix     doctorFix
		prepare func(*doctorFakeHost)
		want    string
	}{
		{
			name: "a directory the environment names but that is not there",
			host: func() *doctorFakeHost {
				h := newDoctorFakeHost()
				h.addDir("/home/tester")
				h.addDir("/home/tester/.local")
				return h
			}(),
			fix:  doctorFix{Kind: doctorFixMakeDir, Path: "/home/tester/.local/state"},
			want: "mkdir /home/tester/.local/state",
		},
		{
			name: "the mode of the ssh directory",
			host: func() *doctorFakeHost {
				h := newDoctorFakeHost()
				h.addFile("/home/tester/.ssh", 0o755, true, "")
				return h
			}(),
			fix:  doctorFix{Kind: doctorFixChmodDir, Path: "/home/tester/.ssh", Mode: 0o700},
			want: "chmod /home/tester/.ssh 0700",
		},
		{
			name: "the mode of a private key",
			host: func() *doctorFakeHost {
				h := newDoctorFakeHost()
				h.addDir("/home/tester/.ssh")
				h.addText("/home/tester/.ssh/id_ed25519", 0o644, "-----BEGIN OPENSSH PRIVATE KEY-----\n")
				return h
			}(),
			fix:  doctorFix{Kind: doctorFixChmodFile, Path: "/home/tester/.ssh/id_ed25519", Mode: 0o600},
			want: "chmod /home/tester/.ssh/id_ed25519 0600",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			t.Setenv("DOTFILES_DRY_RUN", "0")
			host := tc.host
			scan := doctorTestScan(host, doctorTestLinuxInfo())

			preview := tc.fix.preview(scan)
			if preview.Refusal != "" {
				t.Fatalf("the fix refuses a state it should repair: %s", preview.Refusal)
			}
			if len(preview.Effect) < 2 {
				t.Errorf("the preview does not state the effect as a before and an after: %v", preview.Effect)
			}
			if preview.Risk == "" || preview.Permission == "" || preview.Backup == "" {
				t.Errorf("the preview is missing the risk, the permission or the backup: %+v", preview)
			}

			outcome := tc.fix.apply(scan)
			if !outcome.Applied {
				t.Fatalf("the fix was not applied: %s", outcome.Notice)
			}
			// The fix record is written beside the installer's own state, so it is
			// read out of the mutations rather than allowed to hide the change the
			// user confirmed.
			changes := doctorChangesToTheMachine(host)
			if len(changes) != 1 || changes[0] != tc.want {
				t.Fatalf("the machine was changed by %v, want exactly %q", changes, tc.want)
			}
			if !strings.Contains(outcome.Notice, "Verified") {
				t.Errorf("the notice does not report the verification: %q", outcome.Notice)
			}
			if outcome.Record == nil {
				t.Fatal("no record was kept, so the change cannot be put back")
			}
			if outcome.Record.Before == "" || outcome.Record.After == "" {
				t.Errorf("the record does not state what it replaced and what it left: %+v", outcome.Record)
			}
			if outcome.Record.Revert == "" {
				t.Errorf("the record names no way back: %+v", outcome.Record)
			}

			// The record is on disk, in the installer's own state directory, and
			// the second apply of the same fix is refused because the state moved.
			data, ok := host.data[doctorFixHistoryPath()]
			if !ok {
				t.Fatalf("the fix record was not written; the machine recorded %v", host.mutations)
			}
			if !strings.Contains(data, tc.fix.Path) {
				t.Errorf("the record does not name the path it changed:\n%s", data)
			}
			if again := tc.fix.apply(scan); again.Applied {
				t.Errorf("the same fix was applied twice: %s", again.Notice)
			}
			if changes := doctorChangesToTheMachine(host); len(changes) != 1 {
				t.Errorf("the second press changed the machine again: %v", changes)
			}
		})
	}
}

// TestFixRefusesWhatItCannotProve is the other half of the fix rule: a state that
// moved, a path that is a link, a mode that is already right, and a directory
// nobody may write are all refusals with a reason -- never a change made anyway.
func TestFixRefusesWhatItCannotProve(t *testing.T) {
	cases := []struct {
		name string
		host *doctorFakeHost
		fix  doctorFix
	}{
		{
			name: "the directory already exists",
			host: func() *doctorFakeHost {
				h := newDoctorFakeHost()
				h.addDir("/home/tester/.local/state")
				return h
			}(),
			fix: doctorFix{Kind: doctorFixMakeDir, Path: "/home/tester/.local/state"},
		},
		{
			name: "a file is in the way of the directory",
			host: func() *doctorFakeHost {
				h := newDoctorFakeHost()
				h.addText("/home/tester/.local", 0o644, "not a directory\n")
				return h
			}(),
			fix: doctorFix{Kind: doctorFixMakeDir, Path: "/home/tester/.local"},
		},
		{
			name: "the directory above cannot be written",
			host: func() *doctorFakeHost {
				h := newDoctorFakeHost()
				h.addFile("/home/tester/.local", 0o555, true, "")
				return h
			}(),
			fix: doctorFix{Kind: doctorFixMakeDir, Path: "/home/tester/.local/state"},
		},
		{
			name: "the mode is already the one the fix sets",
			host: func() *doctorFakeHost {
				h := newDoctorFakeHost()
				h.addFile("/home/tester/.ssh", 0o700, true, "")
				return h
			}(),
			fix: doctorFix{Kind: doctorFixChmodDir, Path: "/home/tester/.ssh", Mode: 0o700},
		},
		{
			name: "the path is a symbolic link",
			host: func() *doctorFakeHost {
				h := newDoctorFakeHost()
				h.addDir("/home/tester/real")
				h.addLink("/home/tester/.ssh", "/home/tester/real")
				return h
			}(),
			fix: doctorFix{Kind: doctorFixChmodDir, Path: "/home/tester/.ssh", Mode: 0o700},
		},
		{
			name: "the path is gone",
			host: newDoctorFakeHost(),
			fix:  doctorFix{Kind: doctorFixChmodFile, Path: "/home/tester/.ssh/id_ed25519", Mode: 0o600},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			t.Setenv("DOTFILES_DRY_RUN", "0")
			scan := doctorTestScan(tc.host, doctorTestLinuxInfo())

			preview := tc.fix.preview(scan)
			if preview.Refusal == "" {
				t.Fatalf("the fix offered to change a machine it cannot prove: %+v", preview)
			}
			outcome := tc.fix.apply(scan)
			if outcome.Applied {
				t.Errorf("the fix applied a change it had refused: %s", outcome.Notice)
			}
			if len(tc.host.mutations) != 0 {
				t.Errorf("a refused fix changed the machine: %v", tc.host.mutations)
			}
			if !strings.Contains(outcome.Notice, "Nothing was changed") {
				t.Errorf("the refusal is not stated as one: %q", outcome.Notice)
			}
		})
	}
}

// TestADryRunChangesNothing is the brief's own rule, checked on the two writes
// the doctor can make: a scan writes no report and a fix changes nothing.
func TestADryRunChangesNothing(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "1")
	host := doctorRichHost()
	scan := doctorTestScan(host, doctorTestLinuxInfo())

	for _, result := range doctorChecks() {
		runDoctorCheck(scan, doctorCheckForTest(t, result.ID))
	}
	if len(host.mutations) != 0 {
		t.Fatalf("a scan changed the machine: %v", host.mutations)
	}

	fix := doctorFix{Kind: doctorFixMakeDir, Path: "/home/tester/.local/state"}
	outcome := fix.apply(scan)
	if outcome.Applied || len(host.mutations) != 0 {
		t.Errorf("a dry run changed the machine: %v (%s)", host.mutations, outcome.Notice)
	}
	if !strings.Contains(outcome.Notice, "dry run") {
		t.Errorf("the dry run does not say it changed nothing: %q", outcome.Notice)
	}

	report := doctorReport{Time: time.Now(), Checked: 1}
	if _, err := report.write(filepath.Join(t.TempDir(), "nested", "report.txt")); err != nil {
		t.Errorf("a dry run reported a write failure: %v", err)
	}
}

// TestTheReportNeverCarriesASecret is the redaction guard. The machine's
// environment holds two secrets, a credentials file and an agent socket; the
// rendered report must carry none of the values, and must still say that the
// variables whose names announce a secret are set.
func TestTheReportNeverCarriesASecret(t *testing.T) {
	host := doctorRichHost()
	host.setenv("MY_PASSWORD", "hunter2-not-for-the-report")
	host.setenv("PROXY_TOKEN", "tok_do_not_print")
	scan := doctorTestScan(host, doctorTestLinuxInfo())

	report := doctorReport{Time: time.Date(2026, time.January, 4, 12, 0, 0, 0, time.UTC), Build: "dotfiles test", Platform: scan.platformLabel(), Checked: len(doctorChecks())}
	for _, check := range doctorChecks() {
		report.Results = append(report.Results, runDoctorCheck(scan, check))
	}
	text := report.render()

	for _, secret := range []string{
		"ghp_do_not_print_me_0123456789",
		"wJalrXUtnFEMI-do-not-print",
		"hunter2-not-for-the-report",
		"tok_do_not_print",
		"user:token@example.com",
	} {
		if strings.Contains(text, secret) {
			t.Errorf("the report carries the secret %q:\n%s", secret, doctorContextAround(text, secret))
		}
	}
	// A secret variable may be named -- a reader has to know something is set --
	// but never beside its value.
	for _, line := range strings.Split(text, "\n") {
		for _, name := range []string{"GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY", "MY_PASSWORD", "PROXY_TOKEN"} {
			if strings.Contains(line, name) && !strings.Contains(line, "<redacted>") {
				t.Errorf("the report names %s with a value beside it: %q", name, line)
			}
		}
	}
	if !strings.Contains(text, "GITHUB_TOKEN") {
		t.Logf("the environment check reports the variables it documents rather than the whole environment, which is why the report does not name GITHUB_TOKEN at all")
	}

	// The report says it is a read, and the states it found are in it by name.
	if !strings.Contains(text, "Read-only") {
		t.Errorf("the report does not state that the scan changed nothing")
	}
	if !strings.Contains(text, "error") && !strings.Contains(text, "warning") {
		t.Errorf("the report of a machine with something wrong everywhere states no problem:\n%s", text)
	}
}

// doctorChangesToTheMachine is what a fix changed outside the installer's own
// state directory: the two things a fix may touch are the path it names and its
// record of having done so.
func doctorChangesToTheMachine(host *doctorFakeHost) []string {
	var changes []string
	for _, mutation := range host.mutations {
		if strings.Contains(mutation, doctorFixesFileName) || strings.Contains(mutation, "/dotfiles") {
			continue
		}
		changes = append(changes, mutation)
	}
	return changes
}

// TestTheRendererRedactsEveryFieldItWrites is the end-to-end half of the
// redactor: whatever a check read by accident reaches the report through a
// finding, and the renderer is the last place that can stop it.
func TestTheRendererRedactsEveryFieldItWrites(t *testing.T) {
	report := doctorReport{Checked: 1, Results: []doctorCheckResult{{
		ID: "git.credentials", Title: "The credential helper", Category: doctorCategoryGit, Status: doctorError,
		Findings: []doctorFinding{{
			Subject:  "https://tester:sup3rs3cret@example.com/repo.git",
			Status:   doctorError,
			Evidence: "the helper is !f() { echo sk-live-9f8a7b6c9f8a7b6c; }; f for https://u:p4ssw0rd@host/x",
			Impact:   "a token such as ghp_aaaaaaaaaaaaaaaaaaaa could be read",
			Advice:   "set the helper to store (no secret here)",
		}},
	}}}
	text := report.render()
	for _, secret := range []string{"sup3rs3cret", "p4ssw0rd", "sk-live-9f8a7b6c9f8a7b6c", "ghp_aaaaaaaaaaaaaaaaaaaa"} {
		if strings.Contains(text, secret) {
			t.Errorf("the renderer wrote the secret %q:\n%s", secret, doctorContextAround(text, secret))
		}
	}
}

// doctorContextAround is a small helper for a redaction failure: it prints the
// line that carried a value rather than the whole report.
func doctorContextAround(text, needle string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return "(the value is not on one line)"
}

// TestRedactionCoversTheShapesAValueCanTake pins the redactor itself, including
// the one shape that is not an environment variable: a credential inside a URL
// and a private key body.
func TestRedactionCoversTheShapesAValueCanTake(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		gone  string
	}{
		{
			name:  "a password in a URL",
			input: "remote: https://tester:sup3rs3cret@github.com/acme/repo.git",
			want:  "https://tester:<redacted>@github.com",
			gone:  "sup3rs3cret",
		},
		{
			name:  "an assignment whose name announces a secret",
			input: "MY_API_TOKEN=abc123",
			want:  "MY_API_TOKEN=<redacted>",
			gone:  "abc123",
		},
		{
			name:  "a private key body",
			input: "-----BEGIN OPENSSH PRIVATE KEY-----\nAAAAsecretAAAA\n-----END OPENSSH PRIVATE KEY-----",
			want:  "<redacted private key>",
			gone:  "AAAAsecretAAAA",
		},
		{
			name:  "a plain path is left alone",
			input: "PATH entry 1 of 9: /usr/local/bin",
			want:  "/usr/local/bin",
			gone:  "",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := doctorRedact(tc.input)
			if !strings.Contains(got, tc.want) {
				t.Errorf("the redacted text does not hold %q:\n%s", tc.want, got)
			}
			if tc.gone != "" && strings.Contains(got, tc.gone) {
				t.Errorf("the redacted text still holds %q:\n%s", tc.gone, got)
			}
		})
	}

	if !doctorSecretName("GITHUB_TOKEN") || !doctorSecretName("MY_PASSWORD") || !doctorSecretName("AWS_SECRET_ACCESS_KEY") {
		t.Error("a variable whose name announces a secret is not recognised")
	}
	if doctorSecretName("PATH") || doctorSecretName("EDITOR") || doctorSecretName("LANG") {
		t.Error("a variable that is not a secret is treated as one, which would hide a value a finding needs")
	}
}

// TestCountsAndSummaryAgreeWithTheFindings pins the arithmetic the screen and the
// report both read, so the notice and the report cannot disagree about what a
// scan found.
func TestCountsAndSummaryAgreeWithTheFindings(t *testing.T) {
	report := doctorReport{Results: []doctorCheckResult{
		{Findings: []doctorFinding{{Status: doctorError}, {Status: doctorOK}}},
		{Findings: []doctorFinding{{Status: doctorWarning}}},
		{Findings: []doctorFinding{{Status: doctorUnknown}, {Status: doctorNotApplicable}}},
	}}
	counts := report.counts()
	if counts.Error != 1 || counts.Warning != 1 || counts.OK != 1 || counts.Unknown != 1 || counts.NotApplicable != 1 {
		t.Fatalf("the counts do not match the findings: %+v", counts)
	}
	if counts.total() != 5 {
		t.Errorf("the total is %d, want 5", counts.total())
	}
	if counts.worst() != doctorError {
		t.Errorf("the run's own state is %q, want the worst finding", counts.worst())
	}
	summary := counts.summary()
	for _, want := range []string{"1 errors", "1 warnings", "1 unknown", "1 not applicable", "1 ok"} {
		if !strings.Contains(summary, want) {
			t.Errorf("the summary %q does not name %q", summary, want)
		}
	}

	empty := doctorCounts{}
	if empty.worst() != doctorUnknown {
		t.Errorf("a report with no finding at all reports %q, want unknown: an empty scan is not a clean one", empty.worst())
	}
	if got := empty.summary(); got != "no findings" {
		t.Errorf("an empty report summarizes as %q", got)
	}
}
