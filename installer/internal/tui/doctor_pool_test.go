package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// The walk's width
// ============================================================================
//
// The walk over the machine is a small pool, not one check per message: several
// checks are in flight at once, the answers land in the run's map keyed by check
// ID, and the report is still built in run.Order. These tests drive the run the
// way the runtime does -- every command in its own goroutine, a batch expanded
// into the commands it holds, every answer fed back -- so what they assert is
// what a user waits through.

// doctorPoolChecks builds a plan of checks that each say when they have started
// and then wait to be let go. The arrivals are what a test counts: a walk that
// starts one check per message can only ever produce one of them.
func doctorPoolChecks(count int, arrived chan<- struct{}, release <-chan struct{}) []doctorCheck {
	checks := make([]doctorCheck, 0, count)
	for index := 0; index < count; index++ {
		id := fmt.Sprintf("test.pool.%02d", index)
		checks = append(checks, doctorCheck{
			ID:       id,
			Category: doctorCategoryEnvironment,
			Title:    "the pool check " + id,
			Run: func(*doctorScan) []doctorFinding {
				arrived <- struct{}{}
				<-release
				return []doctorFinding{{
					Check: id, Subject: id, Status: doctorOK,
					Evidence: "the check answered", Impact: "none", Advice: "nothing",
				}}
			},
		})
	}
	return checks
}

// doctorAnswerableChecks builds a plan of checks that answer immediately, for
// the tests that hold the commands instead of running them.
func doctorAnswerableChecks(count int) []doctorCheck {
	checks := make([]doctorCheck, 0, count)
	for index := 0; index < count; index++ {
		id := fmt.Sprintf("test.pool.%02d", index)
		checks = append(checks, doctorCheck{
			ID:       id,
			Category: doctorCategoryEnvironment,
			Title:    "the pool check " + id,
			Run: func(*doctorScan) []doctorFinding {
				return []doctorFinding{{
					Check: id, Subject: id, Status: doctorOK,
					Evidence: "the check answered", Impact: "none", Advice: "nothing",
				}}
			},
		})
	}
	return checks
}

// doctorStartedCommands expands the command a run starts into the commands it
// put in flight. It fails when the run started only one, because that is the
// contract this file exists to hold.
func doctorStartedCommands(t *testing.T, cmd tea.Cmd) []tea.Cmd {
	t.Helper()
	if cmd == nil {
		t.Fatal("the run started nothing")
	}
	message := cmd()
	batch, ok := message.(tea.BatchMsg)
	if !ok {
		t.Fatalf("the run started one command (%T) at a time", message)
	}
	return []tea.Cmd(batch)
}

// TestDoctorWalkKeepsSeveralChecksInFlight is #373's first decision: the walk
// runs a bounded pool instead of one check per message, so a slow check holds
// back three answers rather than every answer behind it.
func TestDoctorWalkKeepsSeveralChecksInFlight(t *testing.T) {
	m := doctorScreenModel(t)

	arrived := make(chan struct{}, 2*doctorConcurrency)
	release := make(chan struct{})
	var released sync.Once
	letGo := func() { released.Do(func() { close(release) }) }
	defer letGo()

	checks := doctorPoolChecks(doctorConcurrency, arrived, release)
	cmd := m.doctorStartRun(checks, true)
	if cmd == nil {
		t.Fatal("a run of four checks started nothing")
	}

	// The runtime's shape: every command runs in its own goroutine, and a batch
	// hands its commands back to be run the same way.
	answers := make(chan tea.Msg, 2*doctorConcurrency)
	var start func(tea.Cmd)
	start = func(next tea.Cmd) {
		go func() {
			message := next()
			if batch, ok := message.(tea.BatchMsg); ok {
				for _, inner := range batch {
					start(inner)
				}
				return
			}
			answers <- message
		}()
	}
	start(cmd)

	// More than one check has to be in flight before the first answer is fed
	// back. A walk that starts one check per message leaves the first of them
	// waiting alone, and this is where that shows.
	limit := time.After(10 * time.Second)
	for inFlight := 1; inFlight <= doctorConcurrency; inFlight++ {
		select {
		case <-arrived:
		case <-limit:
			t.Fatalf("%d of %d checks were in flight at once: the walk still starts one check per message",
				inFlight-1, doctorConcurrency)
		}
	}

	// The notice counts answers and says how many are out at once, so the line
	// tells the truth about a walk that is busy rather than idle.
	if !strings.Contains(m.Doctor.Run.Message, fmt.Sprintf("Checked 0 of %d, %d at once", len(checks), len(checks))) {
		t.Errorf("the progress line does not say how many checks are in flight: %q", m.Doctor.Run.Message)
	}

	letGo()
	for answered := 0; answered < len(checks); answered++ {
		select {
		case message := <-answers:
			next, follow := m.Update(message)
			m = next.(Model)
			if follow != nil {
				t.Errorf("the run started more work although every check had already been started")
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%d of %d checks answered", answered, len(checks))
		}
	}

	run := m.Doctor.Run
	if run.running() {
		t.Fatal("the run is still in flight after every check answered")
	}
	if len(run.Results) != len(checks) {
		t.Fatalf("the run kept %d answers for %d checks", len(run.Results), len(checks))
	}
	for _, check := range checks {
		if result, ok := run.Results[check.ID]; !ok || result.Status != doctorOK {
			t.Errorf("the check %q came back %q, want ok", check.ID, result.Status)
		}
	}

	// The answers arrive out of order and the report does not care: it is built
	// from run.Order, which is the catalogue's order.
	var reported []string
	for _, result := range run.report().Results {
		reported = append(reported, result.ID)
	}
	var plan []string
	for _, check := range checks {
		plan = append(plan, check.ID)
	}
	if !slices.Equal(reported, plan) {
		t.Errorf("the report lists %v, want the order the plan runs in %v", reported, plan)
	}
}

// TestDoctorCancelledRunDropsTheAnswersStillInFlight is #373's second decision:
// Escape cancels the checks in flight. The run ends there, and the answers that
// arrive afterwards are dropped rather than written into a run that has already
// been reported -- a cancelled run must not rewrite its own report from behind.
func TestDoctorCancelledRunDropsTheAnswersStillInFlight(t *testing.T) {
	m := doctorScreenModel(t)
	checks := doctorAnswerableChecks(doctorConcurrency)
	inFlight := doctorStartedCommands(t, m.doctorStartRun(checks, true))
	if len(inFlight) != len(checks) {
		t.Fatalf("the run put %d of %d checks in flight", len(inFlight), len(checks))
	}

	next, cmd := m.handleDoctorKeys("esc")
	m = next.(Model)
	if cmd != nil {
		t.Error("cancelling started some work")
	}
	run := m.Doctor.Run
	if run.running() {
		t.Fatal("the run is still in flight after Escape, so the checks in flight were not cancelled")
	}
	if !run.Canceled {
		t.Fatal("escape did not mark the run as cancelled")
	}
	if !strings.Contains(run.Message, "Cancelled") {
		t.Errorf("the screen does not say the run was cancelled: %q", run.Message)
	}
	if run.ReportPath == "" {
		t.Fatal("the cancelled run wrote no report")
	}
	if !strings.Contains(run.Message, fmt.Sprintf("0 of %d", len(checks))) {
		t.Errorf("the closing line does not say how far the run got: %q", run.Message)
	}

	message := run.Message
	answers := len(run.Results)
	results := make(map[string]doctorCheckResult, len(run.Results))
	for id, result := range run.Results {
		results[id] = result
	}
	written, err := os.ReadFile(run.ReportPath)
	if err != nil {
		t.Fatalf("the cancelled run's report is not there: %v", err)
	}

	// The checks that were in flight are still on their way; each of their
	// answers arrives after the run ended.
	for _, check := range inFlight {
		answer := check()
		next, follow := m.Update(answer)
		m = next.(Model)
		if follow != nil {
			t.Errorf("a cancelled run started more work")
		}
	}

	if m.Doctor.Run.Message != message {
		t.Errorf("a late answer changed the closing line: %q, want %q", m.Doctor.Run.Message, message)
	}
	if len(m.Doctor.Run.Results) != answers {
		t.Errorf("a late answer changed the number of answers: %d, want %d", len(m.Doctor.Run.Results), answers)
	}
	if !reflect.DeepEqual(m.Doctor.Run.Results, results) {
		t.Errorf("a late answer was written into a cancelled run: %v, want %v", m.Doctor.Run.Results, results)
	}
	after, err := os.ReadFile(m.Doctor.Run.ReportPath)
	if err != nil {
		t.Fatalf("the report the cancelled run wrote is gone: %v", err)
	}
	if !bytes.Equal(after, written) {
		t.Error("a late answer rewrote the report of a cancelled run")
	}
}

// doctorPoolHostImpl is a host with no machine behind it and no state of its
// own: every name resolves to the same path and every command answers the same
// line. The walk holds the scan's caches from several checks at once, and a
// host that recorded its reads (which is what the fake machine does) would race
// on itself rather than on the caches under test.
type doctorPoolHostImpl struct{}

func (doctorPoolHostImpl) Getenv(string) string                  { return "/pool" }
func (doctorPoolHostImpl) Environ() []string                     { return []string{"PATH=/pool"} }
func (doctorPoolHostImpl) LookPath(name string) (string, error)  { return "/pool/" + name, nil }
func (doctorPoolHostImpl) Lstat(string) (os.FileInfo, error)     { return nil, os.ErrNotExist }
func (doctorPoolHostImpl) Stat(string) (os.FileInfo, error)      { return nil, os.ErrNotExist }
func (doctorPoolHostImpl) ReadDir(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist }
func (doctorPoolHostImpl) ReadFile(string) ([]byte, error)       { return nil, os.ErrNotExist }
func (doctorPoolHostImpl) Readlink(string) (string, error)       { return "", os.ErrNotExist }
func (doctorPoolHostImpl) UserHomeDir() (string, error)          { return "/pool/home", nil }
func (doctorPoolHostImpl) GOOS() string                          { return "linux" }
func (doctorPoolHostImpl) Run(context.Context, string, ...string) doctorCommandResult {
	return doctorCommandResult{Stdout: "pool 1.0.0\n"}
}
func (doctorPoolHostImpl) MkdirAll(string, os.FileMode) error          { return nil }
func (doctorPoolHostImpl) Chmod(string, os.FileMode) error             { return nil }
func (doctorPoolHostImpl) WriteFile(string, []byte, os.FileMode) error { return nil }

// TestDoctorScanCachesSurviveConcurrentChecks is the other half of the pool: the
// walk keeps several checks in flight, and every one of them reads the same
// scan. The caches are the only mutable state a scan has, so they have to
// survive being asked from several checks at once -- a map written from two
// goroutines is a crash, not a cache. Run with -race, this is where an
// unsynchronized cache shows.
func TestDoctorScanCachesSurviveConcurrentChecks(t *testing.T) {
	scan := newDoctorScan(doctorPoolHostImpl{}, nil, time.Now())
	const workers = 2 * doctorConcurrency

	type answer struct {
		lookup  doctorLookup
		version doctorVersion
		probe   doctorCommandResult
	}
	answers := make([]answer, workers)
	var waiting sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		waiting.Add(1)
		go func(index int) {
			defer waiting.Done()
			answers[index] = answer{
				lookup:  scan.lookup("git"),
				version: scan.toolVersion("/pool/git", "--version"),
				probe:   scan.probe(doctorProbeTimeout, "/pool/git", "probe"),
			}
		}(worker)
	}
	waiting.Wait()

	for index, got := range answers {
		if got.lookup.Path != "/pool/git" {
			t.Errorf("worker %d resolved git to %q, want /pool/git", index, got.lookup.Path)
		}
		if got.version.Line != "pool 1.0.0" {
			t.Errorf("worker %d read the version as %q, want the line the tool printed", index, got.version.Line)
		}
		if got.probe.Stdout != "pool 1.0.0\n" {
			t.Errorf("worker %d read the probe as %q", index, got.probe.Stdout)
		}
	}
	if len(scan.lookups) != 1 || len(scan.versions) != 1 || len(scan.runs) != 1 {
		t.Errorf("the caches hold %d lookups, %d versions and %d probes, want one of each",
			len(scan.lookups), len(scan.versions), len(scan.runs))
	}
}
