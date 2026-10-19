package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// THE UTILITIES SECTION: THE DEVELOPMENT-ENVIRONMENT DOCTOR
// ============================================================================
//
// The doctor is the utility that diagnoses this machine rather than describing
// one tool of it: the PATH a shell really resolves, the configuration of every
// shell that is installed, the tools this installer manages, the dotfiles it
// owns, the basics of git, and the environment faults that break a run.
//
// Three rules shape everything here.
//
// **Every answer is one of five states.** ok, a warning, an error, not
// applicable, or unknown -- and unknown is the zero value, because a check that
// did not run must never read as correct. A finding carries the evidence it was
// read from, what it means for the user, and what to do about it; the screen
// refuses to draw a finding that has no evidence.
//
// **The scan is a read.** Nothing here writes, edits or deletes. The only two
// writes the doctor can make live in doctor_fix.go, and each is previewed,
// confirmed, re-checked, applied minimally and verified, with the state it
// replaced recorded first.
//
// **A check that cannot look says so.** A missing binary, a denied directory, a
// command that outlives its bound, a platform that has no such capability: each
// ends as unknown or not-applicable with the reason, never as an invented
// verdict and never as an ok.
//
// Everything the checks read goes through doctorHost, so a test can hand the
// doctor a machine of its own -- unicode paths, dangling symlinks, denied
// reads, a command that hangs -- instead of reading the machine it runs on.

// doctorStatus is how one finding ended.
//
// The order is the aggregation order: the worst finding of a check is the
// check's status, and a check that partly could not run is not an ok. unknown
// sits above not-applicable and ok on purpose -- "I could not look" must never
// be reported as "there is nothing wrong" -- and below the two states that say
// something is wrong.
type doctorStatus int

const (
	// doctorUnknown is the zero value: the check did not run, or could not
	// finish. It is deliberately the zero value so a result nobody filled in
	// reads as unknown rather than as ok.
	doctorUnknown doctorStatus = iota
	doctorOK
	doctorNotApplicable
	doctorWarning
	doctorError
)

// String is the machine-readable name of the state, and the one the report
// writes. The screen's own word for it is label, so a report stays greppable
// while the interface keeps its voice.
func (s doctorStatus) String() string {
	switch s {
	case doctorOK:
		return "ok"
	case doctorNotApplicable:
		return "n/a"
	case doctorWarning:
		return "warning"
	case doctorError:
		return "error"
	default:
		return "unknown"
	}
}

// label is what the screen prints beside the state, so the state survives a
// terminal with no colour: the mark and the word are both shown.
func (s doctorStatus) label() string {
	switch s {
	case doctorOK:
		return "OK"
	case doctorNotApplicable:
		return "Not applicable"
	case doctorWarning:
		return "Warning"
	case doctorError:
		return "Error"
	default:
		return "Unknown"
	}
}

// mark is the one-character marker the list uses. It is a plain glyph drawn by
// every supported terminal, in the same family the installer already uses for
// its step states (view.go): no emoji, because a terminal without an emoji font
// draws a box.
func (s doctorStatus) mark() string {
	switch s {
	case doctorOK:
		return "✓"
	case doctorNotApplicable:
		return "⊘"
	case doctorWarning:
		return "!"
	case doctorError:
		return "✗"
	default:
		return "?"
	}
}

// rank is the order the states aggregate in, and it is deliberately not the
// constant order: unknown is the zero value so that a result nobody filled in
// cannot read as ok, and it still has to outrank ok when one check produced
// both. A finding that says "this was not established" may never be folded into
// a clean answer.
func (s doctorStatus) rank() int {
	switch s {
	case doctorNotApplicable:
		return 0
	case doctorOK:
		return 1
	case doctorWarning:
		return 3
	case doctorError:
		return 4
	default:
		return 2 // unknown
	}
}

// worse returns the state that has to be reported when one check produced both:
// the state with the higher rank wins, whichever way round they arrive.
func worseDoctorStatus(a, b doctorStatus) doctorStatus {
	if b.rank() > a.rank() {
		return b
	}
	return a
}

// doctorCategory is the group a check belongs to. It is what the section's rows
// offer ("Run every check", one row per category) and what the report groups by,
// so one list serves both.
type doctorCategory int

const (
	doctorCategoryPath doctorCategory = iota
	doctorCategoryShell
	doctorCategoryTools
	doctorCategoryDotfiles
	doctorCategoryGit
	doctorCategoryEnvironment
)

// doctorCategories is every category in the order the section lists them. The
// slice is the only list: doctorCategoryValues derives from it and the guard
// checks that every check's category is in it.
func doctorCategories() []doctorCategory {
	return []doctorCategory{
		doctorCategoryPath,
		doctorCategoryShell,
		doctorCategoryTools,
		doctorCategoryDotfiles,
		doctorCategoryGit,
		doctorCategoryEnvironment,
	}
}

// String names the category in the repository's language, which is English, and
// is used by the report.
func (c doctorCategory) String() string {
	switch c {
	case doctorCategoryPath:
		return "PATH and executables"
	case doctorCategoryShell:
		return "Shells and their configuration"
	case doctorCategoryTools:
		return "Tools this installer manages"
	case doctorCategoryDotfiles:
		return "Managed dotfiles"
	case doctorCategoryGit:
		return "Git and SSH"
	case doctorCategoryEnvironment:
		return "Environment and platform"
	default:
		return "Other"
	}
}

// row is the category's one-word name for the section's rows, where a full
// sentence would push the list off a short frame.
func (c doctorCategory) row() string {
	switch c {
	case doctorCategoryPath:
		return "PATH"
	case doctorCategoryShell:
		return "Shells"
	case doctorCategoryTools:
		return "Tools"
	case doctorCategoryDotfiles:
		return "Dotfiles"
	case doctorCategoryGit:
		return "Git"
	case doctorCategoryEnvironment:
		return "Environment"
	default:
		return "Other"
	}
}

// doctorFinding is one answer: what it is about, how it ended, the evidence it
// was read from, what that means and what to do.
//
// Evidence is not optional. A finding with no evidence is an opinion, and the
// guard doctor_findings_test.go fails a check that produces one, so the rule is
// held by the suite rather than by review.
type doctorFinding struct {
	// Check is the ID of the check that produced this finding.
	Check string
	// Subject is what the finding is about: a path, a variable, a tool, a file.
	// It is what the screen shows beside the state.
	Subject string
	// Status is one of the five states.
	Status doctorStatus
	// Evidence is what was read, quoted: a mode, a path, a line of a file, the
	// exit status of a command. It is never a secret and never a guess.
	Evidence string
	// Impact is what happens on this machine because of the finding.
	Impact string
	// Advice is what a person can do by hand. It is what every finding has.
	Advice string
	// Fix is the deterministic repair this finding offers, when one exists. Nil
	// means manual advice only, which is the honest answer for everything
	// ambiguous.
	Fix *doctorFix
}

// doctorCheck is one unit of diagnosis.
type doctorCheck struct {
	// ID names the check. It is stable, because the report and the fix records
	// name it.
	ID string
	// Category is the group the section offers it under.
	Category doctorCategory
	// Title is what the check says it looks at, in the section's voice.
	Title string
	// Required marks a check whose failure breaks a run of this installer or the
	// everyday use of the tools it installed. The screen sorts by the state, not
	// by this flag; the flag is what the impact line is allowed to claim.
	Required bool
	// Run is the check. It returns at least one finding.
	Run func(*doctorScan) []doctorFinding
}

// doctorChecks is every check the doctor runs, in the order it runs them: the
// categories in the section's order, and inside a category the checks that have
// to run first (the ones the others read) before the ones that report.
//
// It is one list on purpose. The rows the section offers, the progress the run
// reports, the report's groups and the guard that every check produces evidence
// all read it, so a check cannot exist in one place and be missing from another.
func doctorChecks() []doctorCheck {
	var checks []doctorCheck
	for _, category := range doctorCategories() {
		for _, check := range doctorCheckTable(category) {
			checks = append(checks, check)
		}
	}
	return checks
}

// doctorCheckTable is the per-category half of doctorChecks, so the catalogue
// reads in the order it runs and one category's checks stay together.
func doctorCheckTable(category doctorCategory) []doctorCheck {
	switch category {
	case doctorCategoryPath:
		return doctorPathChecks()
	case doctorCategoryShell:
		return doctorShellChecks()
	case doctorCategoryTools:
		return doctorToolChecks()
	case doctorCategoryDotfiles:
		return doctorDotfilesChecks()
	case doctorCategoryGit:
		return doctorGitChecks()
	case doctorCategoryEnvironment:
		return doctorEnvironmentChecks()
	default:
		return nil
	}
}

// doctorCheckResult is one check's outcome: the findings it produced, the worst
// state among them, and -- when it could not run at all -- the one finding that
// says so, so the screen never has an empty result to draw.
type doctorCheckResult struct {
	ID       string
	Title    string
	Category doctorCategory
	Required bool
	// Status is the worst finding's state, and unknown when the check did not
	// run.
	Status doctorStatus
	// Findings is never empty once Run has been called.
	Findings []doctorFinding
}

// doctorProgress is the slice of the catalogue a run is scheduled over, so an
// aborted run's results can say which checks never got their turn.
type doctorProgress struct {
	Checks   []doctorCheck
	Results  []doctorCheckResult
	Next     int
	Canceled bool
	// Current is the title of the check being run, empty between checks.
	Current string
}

// runDoctorCheck runs one check and turns a panic, a timeout or a nil result
// into an honest unknown. The doctor runs arbitrary checks over a machine it
// does not control, so a check that fails must cost one row and never the
// interface: a panic here is caught and reported as what it is.
func runDoctorCheck(scan *doctorScan, check doctorCheck) (result doctorCheckResult) {
	result = doctorCheckResult{
		ID:       check.ID,
		Title:    check.Title,
		Category: check.Category,
		Required: check.Required,
	}

	defer func() {
		if r := recover(); r != nil {
			result.Status = doctorUnknown
			result.Findings = []doctorFinding{{
				Check:    check.ID,
				Subject:  check.Title,
				Status:   doctorUnknown,
				Evidence: fmt.Sprintf("the check panicked: %v", r),
				Impact:   "This check did not finish, so nothing about it is known.",
				Advice:   "Run the doctor again; if this repeats, it is a defect in the doctor itself.",
			}}
		}
	}()

	findings := check.Run(scan)
	if len(findings) == 0 {
		// A check that produced nothing did not establish anything, and the one
		// answer it may not give is "ok".
		findings = []doctorFinding{{
			Check:    check.ID,
			Subject:  check.Title,
			Status:   doctorUnknown,
			Evidence: "the check produced no finding",
			Impact:   "Nothing about this is known.",
			Advice:   "Run the doctor again.",
		}}
	}
	for i := range findings {
		if findings[i].Check == "" {
			findings[i].Check = check.ID
		}
	}

	status := doctorOK
	for _, finding := range findings {
		status = worseDoctorStatus(status, finding.Status)
	}
	result.Status = status
	result.Findings = findings
	return result
}

// notRunResult is what a check that never got its turn reports. It exists so an
// aborted run keeps the five-state rule: the checks that did not run are unknown
// with the reason, not rows that quietly vanished.
func notRunResult(check doctorCheck, reason string) doctorCheckResult {
	return doctorCheckResult{
		ID:       check.ID,
		Title:    check.Title,
		Category: check.Category,
		Required: check.Required,
		Status:   doctorUnknown,
		Findings: []doctorFinding{{
			Check:    check.ID,
			Subject:  check.Title,
			Status:   doctorUnknown,
			Evidence: reason,
			Impact:   "This check did not run, so nothing about it is known.",
			Advice:   "Run the doctor again, or run this category on its own.",
		}},
	}
}

// ============================================================================
// The host seam
// ============================================================================

// doctorHost is every effect a check may have on this machine: the environment,
// the filesystem, the PATH lookup and the one way a command is run. It exists so
// the whole scan is testable without reading the machine the tests run on --
// including the failure paths a real machine only reaches by accident (a denied
// directory, a dangling symlink, a command that never answers).
type doctorHost interface {
	Getenv(name string) string
	Environ() []string
	LookPath(name string) (string, error)
	Lstat(path string) (os.FileInfo, error)
	Stat(path string) (os.FileInfo, error)
	ReadDir(path string) ([]os.DirEntry, error)
	ReadFile(path string) ([]byte, error)
	Readlink(path string) (string, error)
	UserHomeDir() (string, error)
	// GOOS is the platform the doctor itself runs on, which decides the path
	// semantics and the checks that can exist at all.
	GOOS() string
	// Run executes one command and returns what it printed. The caller bounds it
	// by the context it passes, so a command that hangs is a bounded failure.
	Run(ctx context.Context, name string, args ...string) doctorCommandResult
	// MkdirAll, Chmod and WriteFile are the only three ways the doctor can change
	// this machine. They exist on the host seam for the same reason the read
	// methods do: a test must be able to watch exactly what a fix would do -- and
	// prove that a dry run does nothing -- without touching a real home directory.
	MkdirAll(path string, perm os.FileMode) error
	Chmod(path string, mode os.FileMode) error
	WriteFile(path string, data []byte, perm os.FileMode) error
}

// doctorCommandResult is one bounded command: what it printed, how it ended and
// whether the bound is what ended it.
type doctorCommandResult struct {
	Stdout string
	Stderr string
	// Err is empty when the command exited zero. Otherwise it says what happened
	// -- the exit status, the missing binary, the bound -- in words.
	Err string
	// Code is the exit status, and -1 when there was no status to read.
	Code int
	// TimedOut reports that the bound ended the command rather than the command
	// ending itself.
	TimedOut bool
}

// doctorCommandWaitDelay bounds the I/O wait that survives the context deadline,
// for the reason shellAuditWaitDelay does: cancelling the context kills the
// command, but os/exec then waits for the goroutines copying its pipes, and a
// child that inherited the stdout pipe would keep that wait unbounded.
const doctorCommandWaitDelay = 1 * time.Second

// osDoctorHost is the real machine.
type osDoctorHost struct{}

func (osDoctorHost) Getenv(name string) string { return os.Getenv(name) }

func (osDoctorHost) Environ() []string { return os.Environ() }

func (osDoctorHost) LookPath(name string) (string, error) { return exec.LookPath(name) }

func (osDoctorHost) Lstat(path string) (os.FileInfo, error) { return os.Lstat(path) }

func (osDoctorHost) Stat(path string) (os.FileInfo, error) { return os.Stat(path) }

func (osDoctorHost) ReadDir(path string) ([]os.DirEntry, error) { return os.ReadDir(path) }

func (osDoctorHost) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func (osDoctorHost) Readlink(path string) (string, error) { return os.Readlink(path) }

func (osDoctorHost) UserHomeDir() (string, error) { return os.UserHomeDir() }

func (osDoctorHost) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }

func (osDoctorHost) Chmod(path string, mode os.FileMode) error { return os.Chmod(path, mode) }

func (osDoctorHost) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (osDoctorHost) GOOS() string { return runtime.GOOS }

// Run runs one command with stdout and stderr captured and no stdin: a check
// reads output, it never waits for a person. The context is what bounds it.
func (osDoctorHost) Run(ctx context.Context, name string, args ...string) doctorCommandResult {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = nil
	cmd.WaitDelay = doctorCommandWaitDelay

	err := cmd.Run()
	result := doctorCommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Code:   -1,
		TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded) ||
			errors.Is(err, exec.ErrWaitDelay),
	}
	var exit *exec.ExitError
	switch {
	case err == nil:
		result.Code = 0
	case errors.As(err, &exit):
		result.Code = exit.ExitCode()
		result.Err = fmt.Sprintf("exited with status %d", result.Code)
	case errors.Is(err, exec.ErrNotFound):
		result.Err = "the command was not found"
	default:
		result.Err = err.Error()
	}
	if result.TimedOut && result.Err == "" {
		result.Err = "the command did not finish inside its bound"
	}
	return result
}

// ============================================================================
// The scan
// ============================================================================

// Timeouts. Every command a check runs is bounded, and the bounds are named here
// so the worst case of the whole scan is a sum of named numbers rather than a
// property nobody measured. A shell's own startup is the slow one (the audit
// already spends ten seconds on a single start), and a version probe is a
// process that prints one line.
const (
	doctorVersionTimeout    = 5 * time.Second
	doctorShellStartTimeout = 10 * time.Second
	doctorProbeTimeout      = 5 * time.Second
	// doctorReadLimit bounds a configuration file the doctor reads. A syntax
	// checker reads the file itself; the doctor only reads the parts it
	// understands, and a file past this bound is reported as too large to read
	// rather than read into memory.
	doctorReadLimit = 1 << 20
)

// doctorScan is one pass over this machine: the host, the platform the installer
// already detected, and the small caches that keep a tool from being looked up
// or started twice by two checks. The caches are per scan on purpose: a check
// must not see a stale answer from an earlier run.
//
// The caches are the one mutable state a scan has, and the walk keeps several
// checks in flight at once, so they are guarded by mu. The guard is held for a
// lookup or for writing an answer back, never across the command that produces
// one: a bound of ten seconds spent holding a lock would put the checks back in
// single file, which is the defect the pool exists to remove. Two checks that
// ask the same question at the same moment can therefore both ask it; they
// answer with the same thing, and the cache is written whichever finishes first.
type doctorScan struct {
	host doctorHost
	info *system.SystemInfo
	now  time.Time

	mu       sync.Mutex
	lookups  map[string]doctorLookup
	versions map[string]doctorVersion
	runs     map[string]doctorCommandResult

	// procRootDir is where /proc lives, "/proc" unless a test says otherwise.
	procRootDir string
}

// doctorLookup is one PATH lookup: the first match, or why there was none.
type doctorLookup struct {
	Path  string
	Error string
}

// found reports whether the name resolved.
func (l doctorLookup) found() bool { return l.Path != "" }

// doctorVersion is one tool's version answer, with the exact command it came
// from so the report can be reproduced by hand.
type doctorVersion struct {
	// Command is the line that was run, in the words a user would type.
	Command string
	// Line is the first line of output, trimmed. It is empty when there is none.
	Line string
	// Error is why there is no line: a non-zero exit, a missing binary, the
	// bound. Empty when the command exited zero.
	Error string
	// TimedOut reports that the bound ended it.
	TimedOut bool
}

// newDoctorScan builds one scan. info may be nil, which is what a model built
// without detection has; every platform question then answers unknown rather
// than guessing.
func newDoctorScan(host doctorHost, info *system.SystemInfo, now time.Time) *doctorScan {
	if host == nil {
		host = osDoctorHost{}
	}
	return &doctorScan{
		host:     host,
		info:     info,
		now:      now,
		lookups:  map[string]doctorLookup{},
		versions: map[string]doctorVersion{},
		runs:     map[string]doctorCommandResult{},
	}
}

func (s *doctorScan) getenv(name string) string {
	if s == nil || s.host == nil {
		return ""
	}
	return s.host.Getenv(name)
}

// environ is the environment as name=value pairs, sorted by name so a finding
// that lists variables is stable between runs.
func (s *doctorScan) environ() []string {
	if s == nil || s.host == nil {
		return nil
	}
	env := s.host.Environ()
	sort.Strings(env)
	return env
}

// lookup resolves one name the way a shell would: the first match in PATH wins,
// and PATH is read once per scan.
func (s *doctorScan) lookup(name string) doctorLookup {
	s.mu.Lock()
	cached, ok := s.lookups[name]
	s.mu.Unlock()
	if ok {
		return cached
	}
	lookup := doctorLookup{}
	if s.host == nil {
		lookup.Error = "there is no host to look the name up on"
	} else if path, err := s.host.LookPath(name); err != nil {
		lookup.Error = err.Error()
	} else {
		lookup.Path = path
	}
	s.mu.Lock()
	s.lookups[name] = lookup
	s.mu.Unlock()
	return lookup
}

// toolVersion asks one tool for its version, once per scan. The command is the
// tool's own convention ("--version" almost everywhere, "-V" for tmux), and the
// answer is the first non-empty line: a multi-line banner (nvim, zellij) still
// states its version on the first line.
func (s *doctorScan) toolVersion(path string, args ...string) doctorVersion {
	key := path + "\x00" + strings.Join(args, "\x00")
	s.mu.Lock()
	cached, ok := s.versions[key]
	s.mu.Unlock()
	if ok {
		return cached
	}
	command := strings.Join(append([]string{filepath.Base(path)}, args...), " ")

	result := s.run(doctorVersionTimeout, path, args...)
	version := doctorVersion{Command: command, TimedOut: result.TimedOut, Error: result.Err}
	if line := doctorFirstLine(result.Stdout); line != "" {
		version.Line = line
	} else if result.Err == "" {
		version.Error = "the command printed nothing"
	} else if line := doctorFirstLine(result.Stderr); line != "" {
		// A tool that prints its version on stderr and exits zero is still
		// answering; the first line is taken from there rather than reported as
		// a failure.
		version.Line = line
		if result.Err == "" {
			version.Error = ""
		}
	}
	if result.TimedOut {
		version.Command = command + " (bounded at " + doctorDuration(doctorVersionTimeout) + ")"
	}
	s.mu.Lock()
	s.versions[key] = version
	s.mu.Unlock()
	return version
}

// run runs one command under a bound. The scan does not cache here: a command
// whose answer a fix has to verify after changing the machine must be asked
// again, and a cache is exactly what would answer with the state before.
func (s *doctorScan) run(timeout time.Duration, name string, args ...string) doctorCommandResult {
	if s == nil || s.host == nil {
		return doctorCommandResult{Code: -1, Err: "there is no host to run the command on"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.host.Run(ctx, name, args...)
}

// probe runs a command once per scan and remembers it, for the questions a
// check asks that are not versions (whether a directory is a mount, whether a
// terminfo entry exists). The key is the whole command line.
func (s *doctorScan) probe(timeout time.Duration, name string, args ...string) doctorCommandResult {
	key := name + "\x00" + strings.Join(args, "\x00")
	s.mu.Lock()
	cached, ok := s.runs[key]
	s.mu.Unlock()
	if ok {
		return cached
	}
	result := s.run(timeout, name, args...)
	s.mu.Lock()
	s.runs[key] = result
	s.mu.Unlock()
	return result
}

func (s *doctorScan) stat(path string) (os.FileInfo, error) {
	if s == nil || s.host == nil {
		return nil, errors.New("there is no host to read the filesystem on")
	}
	return s.host.Stat(path)
}

func (s *doctorScan) lstat(path string) (os.FileInfo, error) {
	if s == nil || s.host == nil {
		return nil, errors.New("there is no host to read the filesystem on")
	}
	return s.host.Lstat(path)
}

func (s *doctorScan) readFile(path string) (string, error) {
	if s == nil || s.host == nil {
		return "", errors.New("there is no host to read the file on")
	}
	info, err := s.host.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > doctorReadLimit {
		return "", fmt.Errorf("the file is larger than the %d bytes the doctor reads", doctorReadLimit)
	}
	data, err := s.host.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *doctorScan) readDir(path string) ([]os.DirEntry, error) {
	if s == nil || s.host == nil {
		return nil, errors.New("there is no host to read the filesystem on")
	}
	entries, err := s.host.ReadDir(path)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

func (s *doctorScan) readlink(path string) (string, error) {
	if s == nil || s.host == nil {
		return "", errors.New("there is no host to read the filesystem on")
	}
	return s.host.Readlink(path)
}

// goos is the platform the doctor itself runs on.
func (s *doctorScan) goos() string {
	if s == nil || s.host == nil {
		return ""
	}
	return s.host.GOOS()
}

// pathEntries is PATH split the way this platform splits it, with the entries
// in the order the shell resolves them. An empty PATH is no entries at all
// rather than one empty entry.
func (s *doctorScan) pathEntries() []string {
	raw := s.getenv("PATH")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return filepath.SplitList(raw)
}

// home is the user's home as the environment names it, falling back to the
// platform's own lookup. It is empty when neither answers, which every check
// that needs it reports as unknown rather than inventing a path.
func (s *doctorScan) home() string {
	if home := s.getenv("HOME"); home != "" {
		return home
	}
	if s.host == nil {
		return ""
	}
	home, err := s.host.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// xdgDir answers one XDG base directory the way the specification does: the
// environment variable when it holds an absolute path, and the documented
// fallback under the home directory otherwise. Relative values are ignored by
// the tools that follow the specification, and the environment check reports
// that as its own finding, so this returns the fallback rather than the
// relative value.
func (s *doctorScan) xdgDir(variable string) string {
	if value := s.getenv(variable); value != "" && filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return s.xdgFallback(variable)
}

// xdgFallback is the home-relative directory the specification names for one
// variable. Each fallback is spelled as a string literal at its own join rather
// than passed in: the home-path guard in install_paths_test.go classifies a home
// destination from the literal the sources join, and refuses to resolve one it
// cannot see, because an unresolvable destination is where an unprotected write
// hides.
func (s *doctorScan) xdgFallback(variable string) string {
	home := s.home()
	if home == "" {
		return ""
	}
	switch variable {
	case "XDG_CONFIG_HOME":
		return filepath.Join(home, ".config")
	case "XDG_DATA_HOME":
		return filepath.Join(home, ".local/share")
	case "XDG_STATE_HOME":
		return filepath.Join(home, ".local/state")
	case "XDG_CACHE_HOME":
		return filepath.Join(home, ".cache")
	}
	return ""
}

// configHome is $XDG_CONFIG_HOME or ~/.config.
func (s *doctorScan) configHome() string { return s.xdgDir("XDG_CONFIG_HOME") }

// dataHome is $XDG_DATA_HOME or ~/.local/share.
func (s *doctorScan) dataHome() string { return s.xdgDir("XDG_DATA_HOME") }

// stateHome is $XDG_STATE_HOME or ~/.local/state.
func (s *doctorScan) stateHome() string { return s.xdgDir("XDG_STATE_HOME") }

// cacheHome is $XDG_CACHE_HOME or ~/.cache.
func (s *doctorScan) cacheHome() string { return s.xdgDir("XDG_CACHE_HOME") }

// procRoot is where this platform keeps its process table, used to read the name
// of the shell this process was started from without running a command. It is a
// field rather than a constant so a test can hand the check a process tree of
// its own, which is the same reason every other read goes through the host.
func (s *doctorScan) procRoot() string {
	if s == nil || s.procRootDir == "" {
		return "/proc"
	}
	return s.procRootDir
}

// systemInfo is the platform detection the installer already ran, or nil when
// there is none. A nil answer is reported as unknown, never guessed.
func (s *doctorScan) systemInfo() *system.SystemInfo {
	if s == nil {
		return nil
	}
	return s.info
}

// isWSL reports whether this really is WSL. The distinction matters more than
// any other platform question here: a check that confuses the Windows host with
// the distribution would tell a Linux user to fix a file they do not have.
func (s *doctorScan) isWSL() bool {
	info := s.systemInfo()
	return info != nil && info.IsWSL
}

// isTermux reports whether this is Termux, where the package manager, the
// prefix and a good few binaries are their own thing.
func (s *doctorScan) isTermux() bool {
	info := s.systemInfo()
	return info != nil && info.IsTermux
}

// isMac reports whether the doctor runs on macOS.
func (s *doctorScan) isMac() bool {
	if info := s.systemInfo(); info != nil {
		return info.OS == system.OSMac
	}
	return s.goos() == "darwin"
}

// platformLabel is how the report names this machine: the operating system the
// detection found, the architecture, and the two environments that change what
// a run does (WSL and Termux). An undetected host says so.
func (s *doctorScan) platformLabel() string {
	info := s.systemInfo()
	if info == nil {
		if goos := s.goos(); goos != "" {
			return goos + " (not detected)"
		}
		return "unknown platform"
	}
	label := info.OSName
	if label == "" {
		label = s.goos()
	}
	if info.Arch != "" {
		label += " " + info.Arch
	}
	switch {
	case info.IsWSL && info.WSLVersion == 2:
		label += ", WSL2"
	case info.IsWSL:
		label += ", WSL"
	case info.IsTermux:
		label += ", Termux"
	}
	return label
}

// ============================================================================
// The report
// ============================================================================

// doctorReport is one finished -- or half-finished -- scan.
type doctorReport struct {
	// Time is when the scan ran, Build the build that ran it: both are written
	// into the report so a reader can tell which run it is.
	Time  time.Time
	Build string
	// Platform is what the detection found, in one line.
	Platform string
	// Checked is how many checks the run scheduled, and Results what came of
	// them. A run that was cancelled carries the results it has and the checks it
	// never reached, already turned into unknown findings by the caller.
	Checked int
	Results []doctorCheckResult
}

// doctorCounts is how many findings of each state a report holds. The section's
// result line and the report's own summary both read it, so the two cannot
// disagree about what a scan found.
type doctorCounts struct {
	OK            int
	Warning       int
	Error         int
	NotApplicable int
	Unknown       int
}

// counts tallies the report's findings by state.
func (r doctorReport) counts() doctorCounts {
	var counts doctorCounts
	for _, result := range r.Results {
		for _, finding := range result.Findings {
			switch finding.Status {
			case doctorOK:
				counts.OK++
			case doctorWarning:
				counts.Warning++
			case doctorError:
				counts.Error++
			case doctorNotApplicable:
				counts.NotApplicable++
			default:
				counts.Unknown++
			}
		}
	}
	return counts
}

// total is how many findings the report holds.
func (c doctorCounts) total() int {
	return c.OK + c.Warning + c.Error + c.NotApplicable + c.Unknown
}

// summary is the one line the section and the report both state: what the scan
// found, in the order that matters. A scan with no findings at all says so
// rather than reading as "everything is fine": it means the checks produced
// nothing, which is not a clean bill of health.
func (c doctorCounts) summary() string {
	if c.total() == 0 {
		return "no findings"
	}
	parts := make([]string, 0, 5)
	add := func(n int, one string) {
		if n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, one))
		}
	}
	add(c.Error, "errors")
	add(c.Warning, "warnings")
	add(c.Unknown, "unknown")
	add(c.NotApplicable, "not applicable")
	add(c.OK, "ok")
	return strings.Join(parts, ", ")
}

// worst is the state the run as a whole has to be reported as: the worst finding
// in it, in the same order worseDoctorStatus aggregates by, and unknown when it
// holds no finding at all -- an empty report is not a clean one.
func (c doctorCounts) worst() doctorStatus {
	switch {
	case c.total() == 0:
		return doctorUnknown
	case c.Error > 0:
		return doctorError
	case c.Warning > 0:
		return doctorWarning
	case c.Unknown > 0:
		return doctorUnknown
	case c.OK > 0:
		return doctorOK
	default:
		return doctorNotApplicable
	}
}

// render writes the report as text: the run's identity, the counts, and then
// every check with its findings. Every line goes through the redactor, so a
// value a check read by accident cannot reach the screen or the file.
func (r doctorReport) render() string {
	var b strings.Builder
	b.WriteString("dotfiles doctor\n")
	if r.Build != "" {
		b.WriteString("Build:    " + r.Build + "\n")
	}
	if !r.Time.IsZero() {
		b.WriteString("When:     " + r.Time.Format("2006-01-02 15:04:05") + "\n")
	}
	if r.Platform != "" {
		b.WriteString("Platform: " + r.Platform + "\n")
	}
	counts := r.counts()
	b.WriteString(fmt.Sprintf("Findings: %s\n", counts.summary()))
	b.WriteString(fmt.Sprintf("Checks:   %d scheduled, %d reported\n", r.Checked, len(r.Results)))
	b.WriteString("Read-only: this report names what was read. The scan changed nothing.\n")

	for _, category := range doctorCategories() {
		var group []doctorCheckResult
		for _, result := range r.Results {
			if result.Category == category {
				group = append(group, result)
			}
		}
		if len(group) == 0 {
			continue
		}
		b.WriteString("\n===== " + category.String() + " =====\n")
		for _, result := range group {
			b.WriteString(fmt.Sprintf("\n-- %s (%s) --\n", result.Title, result.Status))
			for _, finding := range result.Findings {
				b.WriteString(fmt.Sprintf("%-16s %s\n", strings.ToUpper(finding.Status.String()), finding.Subject))
				writeDoctorField(&b, "evidence", finding.Evidence)
				writeDoctorField(&b, "impact", finding.Impact)
				writeDoctorField(&b, "do", finding.Advice)
				if finding.Fix != nil {
					writeDoctorField(&b, "fix", "the doctor offers a deterministic fix: "+finding.Fix.summary())
				}
			}
		}
	}
	return doctorRedact(b.String())
}

// writeDoctorField writes one labelled paragraph, wrapped so the report stays
// readable in a terminal and in a diff.
func writeDoctorField(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	lines := wrapText(doctorSingleLine(value), 96, 0)
	for i, line := range lines {
		if i == 0 {
			b.WriteString(fmt.Sprintf("%-16s %s\n", label+":", line))
			continue
		}
		b.WriteString(fmt.Sprintf("%-16s %s\n", "", line))
	}
}

// doctorSingleLine folds a value onto one line so the report's fields stay one
// paragraph each, whatever the command printed.
func doctorSingleLine(value string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(value, "\n", " ")), " ")
}

// write persists the report in the installer's own state directory, beside the
// record of the last install, and returns the path it wrote. A dry run writes
// nothing and says so by returning the path with a nil error, because the
// screen names where the report is either way.
func (r doctorReport) write(path string) (string, error) {
	if path == "" {
		return "", errors.New("the state directory could not be determined")
	}
	if dryRun() {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), stateDirMode); err != nil {
		return path, err
	}
	if err := os.WriteFile(path, []byte(r.render()), stateFileMode); err != nil {
		return path, err
	}
	return path, nil
}

// doctorReportPath is the file every finished scan is kept in. It is named once
// here so the screen, the writer and the documentation cannot disagree.
func doctorReportPath() string {
	dir := stateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, doctorReportFileName)
}

// doctorReportFileName is the report's name in the state directory.
const doctorReportFileName = "doctor-report.txt"

// doctorExportPath is where a pressed export writes: the same directory, with
// the stamp in the name so an export never overwrites the running record and two
// exports of two different runs are two files.
func doctorExportPath(now time.Time) string {
	dir := stateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "doctor-report-"+now.Format("20060102-150405")+".txt")
}

// ============================================================================
// Redaction
// ============================================================================

// doctorSecretName reports whether a variable's name says its value is a secret.
// The rule is by name, never by value: the doctor does not read a value in order
// to decide whether it looks like a secret, because reading it is the thing the
// rule exists to prevent.
func doctorSecretName(name string) bool {
	upper := strings.ToUpper(name)
	for _, marker := range []string{
		"TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "APIKEY",
		"API_KEY", "PRIVATE_KEY", "ACCESS_KEY", "AUTH", "SESSION", "COOKIE",
		"PASSPHRASE", "NETRC", "BEARER", "_KEY", "KEY_", "PWD",
	} {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}

// doctorEnvEntry is how the doctor states one environment variable: the name,
// whether it is set, and -- when its name does not say it is a secret -- the
// value. A secret-named variable is reported as set or unset and never printed.
func doctorEnvEntry(scan *doctorScan, name string) string {
	value := scan.getenv(name)
	if value == "" {
		return name + " is not set"
	}
	if doctorSecretName(name) {
		return name + " is set (the value is not printed: its name says it is a secret)"
	}
	return fmt.Sprintf("%s=%s", name, value)
}

// doctorRedactPatterns are the value shapes a report must never carry, whatever
// the name of the variable or field they came from.
var doctorRedactPatterns = []struct {
	pattern *regexp.Regexp
	replace string
}{
	// A credential inside a URL: scheme://user:password@host.
	{regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^/\s:@]+):[^/\s@]+@`), "$1:<redacted>@"},
	// An assignment whose name says the value is a secret.
	{regexp.MustCompile(`(?i)\b([A-Za-z0-9_]*(?:TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|APIKEY|API_KEY|PRIVATE_KEY|ACCESS_KEY|PASSPHRASE|BEARER)[A-Za-z0-9_]*)\s*[:=]\s*\S+`), "$1=<redacted>"},
	// A PEM block's body, which is the key itself.
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), "<redacted private key>"},
	// An authorization header, or the same line echoed by a tool.
	{regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]{8,}`), "$1 <redacted>"},
	// A value whose shape is a credential from a service this installer touches:
	// a GitHub token, a GitHub fine-grained token, an OpenAI-style key, a Slack
	// token, an AWS key id. A value with no name and no recognisable shape cannot
	// be told from any other string, which is why no check prints one.
	{regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9]{16,}|github_pat_[A-Za-z0-9_]{20,}|sk-[A-Za-z0-9-]{16,}|xox[baprs]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16})\b`), "<redacted>"},
}

// doctorRedact removes every value the report must not carry. It runs over the
// whole rendered report, not over the fields a check remembered to hide, so a
// check that prints a line of a configuration file cannot leak through it.
func doctorRedact(text string) string {
	for _, rule := range doctorRedactPatterns {
		text = rule.pattern.ReplaceAllString(text, rule.replace)
	}
	return text
}

// ============================================================================
// Small shared helpers
// ============================================================================

// doctorFirstLine is the first non-empty line of a command's output, trimmed and
// bounded: a tool that prints a banner gets one line into the report.
func doctorFirstLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "\r"))
		if line == "" {
			continue
		}
		if len(line) > 200 {
			line = line[:200] + "…"
		}
		return line
	}
	return ""
}

// doctorDuration names a duration the way a person reads it.
func doctorDuration(d time.Duration) string {
	if d%time.Second == 0 {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	return d.String()
}

// doctorCounted names a count with its noun in the singular when there is one,
// because "1 checks" reads as a count nobody looked at.
func doctorCounted(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// doctorPermString prints a file mode the way chmod writes it, so the advice a
// finding gives can be typed as it is read.
func doctorPermString(mode os.FileMode) string {
	return fmt.Sprintf("%04o", mode.Perm())
}

// doctorPathList joins paths for one evidence line, bounded so a directory with
// a thousand entries does not fill the report.
func doctorPathList(paths []string, most int) string {
	if len(paths) <= most {
		return strings.Join(paths, ", ")
	}
	shown := append([]string(nil), paths[:most]...)
	return strings.Join(shown, ", ") + fmt.Sprintf(", and %d more", len(paths)-most)
}

// doctorRelHomePath renders a path under the home directory as ~/…, for the
// lines a fix writes where no scan is at hand (its own summary, the report). It
// is the environment's own answer; doctorRelHome is the scan's.
func doctorRelHomePath(path string) string {
	return doctorRelHomeWith(os.Getenv("HOME"), path)
}

// doctorRelHome renders a path under the home directory as ~/…, which is what
// the user recognises and what keeps a report from carrying an absolute home
// path on every line.
func doctorRelHome(scan *doctorScan, path string) string {
	return doctorRelHomeWith(scan.home(), path)
}

// doctorRelHomeWith is the one rule both renderers use, so the scan's answer and
// the environment's cannot disagree about what ~/ means.
func doctorRelHomeWith(home, path string) string {
	if home == "" || path == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if prefix := home + string(os.PathSeparator); strings.HasPrefix(path, prefix) {
		return "~" + string(os.PathSeparator) + strings.TrimPrefix(path, prefix)
	}
	return path
}
