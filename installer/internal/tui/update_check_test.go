package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// updateAutoCheckDisarm turns the automatic release check off for this whole
// package, once, before any test runs.
//
// It is the one rule a guard cannot state for itself: the check writes a file into
// the run's state directory and reaches api.github.com, and a test must do
// neither. Most tests in this package build a model and run its Init, and the
// model arms the check from the drawing gate -- which several of them turn on --
// so nothing inside the tests being run can be relied on. This runs first for the
// whole package instead, and TestAutoUpdateCheckIsNeverArmedInATestBinary fails
// if it is ever removed.
//
// The guard that pins the arming logic saves this value, puts it back to true for
// its own case, and restores it.
//
// The same init points the state directory at a fresh temporary one for the
// whole package, because the model reads the cached record on the startup path
// and the main menu now draws a row from it. A developer's own update-check.json
// must not add that row to a test: without this, a menu whose rows are counted by
// index would pass on CI and fail on the machine that had once run the installer.
func init() {
	updateAutoCheckAllowed = func() bool { return false }
	if dir, err := os.MkdirTemp("", "dotfiles-tui-state-"); err == nil {
		_ = os.Setenv("XDG_STATE_HOME", dir)
	}
}

// ---------------------------------------------------------------------------
// The installer's own update check: the latest published release, the cache that
// keeps it from asking on every start, and the swap that replaces this binary.
// ---------------------------------------------------------------------------
//
// Four rules the guards below hold, and each one is a rule a release process can
// break silently:
//
//   - A failed check is recorded and reported as unknown. "You are up to date"
//     is a claim about GitHub, so a run that could not reach GitHub must not make
//     it. A hole that is declared is survivable; a hole that is silent is not.
//   - The cache is a file, not a per-start question. Within its lifetime a start
//     makes no request at all, which is what keeps the interface from waiting on
//     the network.
//   - The swap verifies the bytes against the release's own SHA256SUMS before
//     anything moves, keeps the binary it replaced, and moves with one rename.
//     It is the order the OfficeCLI step already uses.
//   - A binary Homebrew installed is Homebrew's. The installer says so and names
//     `brew upgrade dotfiles` instead of writing over a file another program
//     owns -- the same question the theme switch asks of a config file.

// updateTestClient serves the given URLs from memory, so a guard exercises the
// real request, the real status handling and the real body reading without a
// socket. Every request is counted: "the cache was reused" is measured as "no
// request happened", not as "the answer was the same".
type updateTestClient struct {
	files    map[string]string
	status   map[string]int
	failures map[string]error
	requests []string
}

func newUpdateTestClient(files map[string]string) *updateTestClient {
	return &updateTestClient{files: files, status: map[string]int{}, failures: map[string]error{}}
}

func (c *updateTestClient) client() *http.Client {
	return &http.Client{Transport: roundTripFunc(c.roundTrip)}
}

func (c *updateTestClient) roundTrip(r *http.Request) (*http.Response, error) {
	c.requests = append(c.requests, r.URL.String())
	if err := c.failures[r.URL.String()]; err != nil {
		return nil, err
	}
	status := c.status[r.URL.String()]
	if status == 0 {
		status = http.StatusOK
	}
	body, ok := c.files[r.URL.String()]
	if !ok {
		status = http.StatusNotFound
		body = "not found"
	}
	if status >= 400 {
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// isolateUpdateState points the update record at a temporary state directory, so
// a guard reads and writes its own file and never the one the machine running
// the tests keeps.
func isolateUpdateState(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	return dir
}

func releaseLatestBody(tag string) string {
	return `{"tag_name":"` + tag + `","draft":false,"prerelease":false}`
}

func sha256Of(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// writeFakeBinary writes an executable at path and returns its bytes.
func writeFakeBinary(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return content
}

// ---------------------------------------------------------------------------
// Reading the latest release
// ---------------------------------------------------------------------------

func TestLatestReleaseTagReadsTheTagName(t *testing.T) {
	c := newUpdateTestClient(map[string]string{dotfilesReleaseAPIURL: releaseLatestBody("v0.5.1")})

	tag, err := latestReleaseTag(c.client(), dotfilesReleaseAPIURL)
	if err != nil {
		t.Fatalf("latestReleaseTag: %v", err)
	}
	if tag != "v0.5.1" {
		t.Errorf("tag = %q, want v0.5.1", tag)
	}
}

func TestLatestReleaseTagFailsOnANonOKStatus(t *testing.T) {
	c := newUpdateTestClient(nil)
	c.status[dotfilesReleaseAPIURL] = http.StatusForbidden

	if _, err := latestReleaseTag(c.client(), dotfilesReleaseAPIURL); err == nil {
		t.Fatal("a 403 answered with a tag instead of an error: a rate-limited check would look like a result")
	}
}

func TestLatestReleaseTagFailsOnAnAnswerWithoutATag(t *testing.T) {
	c := newUpdateTestClient(map[string]string{dotfilesReleaseAPIURL: `{"message":"Not Found"}`})

	if _, err := latestReleaseTag(c.client(), dotfilesReleaseAPIURL); err == nil {
		t.Fatal("a body with no tag_name answered with a tag instead of an error")
	}
}

// ---------------------------------------------------------------------------
// The cache
// ---------------------------------------------------------------------------

func TestRefreshUpdateRecordStoresTheTagAndTheTime(t *testing.T) {
	isolateUpdateState(t)
	c := newUpdateTestClient(map[string]string{dotfilesReleaseAPIURL: releaseLatestBody("v0.5.1")})
	now := time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC)

	rec := refreshUpdateRecord(c.client(), channelStable, now)

	if rec.Latest != "v0.5.1" {
		t.Errorf("Latest = %q, want v0.5.1", rec.Latest)
	}
	if !rec.CheckedAt.Equal(now) {
		t.Errorf("CheckedAt = %v, want %v", rec.CheckedAt, now)
	}
	if rec.Failed != "" {
		t.Errorf("Failed = %q, want empty on a successful check", rec.Failed)
	}

	back, ok := readUpdateRecord()
	if !ok {
		t.Fatal("the record was refreshed but not written, so the next start asks again")
	}
	if back.Latest != "v0.5.1" || !back.CheckedAt.Equal(now) {
		t.Errorf("stored record = %+v, want the tag and the time that was reported", back)
	}
}

func TestRefreshUpdateRecordRecordsAFailureInsteadOfAtag(t *testing.T) {
	isolateUpdateState(t)
	c := newUpdateTestClient(map[string]string{})
	c.failures[dotfilesReleaseAPIURL] = errors.New("dial tcp: no route to host")
	now := time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC)

	rec := refreshUpdateRecord(c.client(), channelStable, now)

	if rec.Latest != "" {
		t.Errorf("Latest = %q after a failed check, want empty: an unreachable GitHub is not a version", rec.Latest)
	}
	if rec.Failed == "" {
		t.Fatal("a failed check left no reason behind, so the interface cannot tell the user why it does not know")
	}
	if !strings.Contains(rec.Failed, "no route to host") {
		t.Errorf("Failed = %q, want the transport's own reason in it", rec.Failed)
	}
	if !rec.CheckedAt.Equal(now) {
		t.Errorf("CheckedAt = %v, want the attempt's time %v: the record is when we last tried", rec.CheckedAt, now)
	}
	// The failure is cached too: a machine with no network must not retry the
	// same doomed request on every start.
	if _, ok := readUpdateRecord(); !ok {
		t.Error("a failed check was not recorded, so the next start pays for the same timeout again")
	}
}

func TestUpdateCheckDueHonoursTheLifetime(t *testing.T) {
	now := time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		rec  updateRecord
		want bool
	}{
		{"no record at all", updateRecord{}, true},
		{"checked a minute ago", updateRecord{CheckedAt: now.Add(-time.Minute), Latest: "v0.5.1"}, false},
		{"checked just inside the lifetime", updateRecord{CheckedAt: now.Add(-updateCheckTTL + time.Minute)}, false},
		{"checked just outside the lifetime", updateRecord{CheckedAt: now.Add(-updateCheckTTL - time.Minute)}, true},
		{"a failure is retried once it is stale", updateRecord{CheckedAt: now.Add(-updateCheckTTL - time.Hour), Failed: "offline"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := updateCheckDue(tc.rec, now); got != tc.want {
				t.Errorf("updateCheckDue(%+v) = %v, want %v", tc.rec, got, tc.want)
			}
		})
	}
}

func TestLoadUpdateStateReadsTheCacheWithoutAskingAnyone(t *testing.T) {
	isolateUpdateState(t)
	checked := time.Date(2026, time.October, 4, 9, 0, 0, 0, time.UTC)
	if err := writeUpdateRecord(updateRecord{CheckedAt: checked, Latest: "v0.5.1"}); err != nil {
		t.Fatalf("writeUpdateRecord: %v", err)
	}

	state := loadUpdateState()

	if !state.Present() {
		t.Fatal("a cached record did not reach the state, so a start knows nothing until it asks GitHub")
	}
	if state.Latest != "v0.5.1" || !state.CheckedAt.Equal(checked) {
		t.Errorf("state = %+v, want the cached tag and time", state)
	}
}

func TestLoadUpdateStateIsEmptyWhenNothingWasEverChecked(t *testing.T) {
	isolateUpdateState(t)

	state := loadUpdateState()

	if state.Present() {
		t.Fatalf("an empty state directory produced %+v, so the interface would report a check that never happened", state)
	}
	if state.UpToDate() {
		t.Error("an empty state claims the install is up to date")
	}
}

// ---------------------------------------------------------------------------
// Comparing versions, and saying so honestly
// ---------------------------------------------------------------------------

func TestVersionIsNewerComparesNumbersNotText(t *testing.T) {
	previous := Version
	t.Cleanup(func() { Version = previous })

	cases := []struct {
		current, candidate string
		want               bool
	}{
		{"v0.5.0", "v0.5.1", true},
		{"v0.5.1", "v0.5.0", false},
		{"v0.5.1", "v0.5.1", false},
		{"0.5.1", "v0.5.10", true},   // text comparison would say no
		{"v0.10.0", "v0.9.0", false}, // and would say yes here
		{"v0.5.1", "v1.0.0", true},
		{"dev", "v0.5.1", true}, // a source build is behind every release
		{"v0.5.1", "", false},   // nothing published is not a newer release
	}
	for _, tc := range cases {
		t.Run(tc.current+" -> "+tc.candidate, func(t *testing.T) {
			if got := versionIsNewer(tc.current, tc.candidate); got != tc.want {
				t.Errorf("versionIsNewer(%q, %q) = %v, want %v", tc.current, tc.candidate, got, tc.want)
			}
		})
	}
}

func TestUpdateStateNeverClaimsUpToDateWithoutAnAnswer(t *testing.T) {
	cases := []struct {
		name       string
		state      updateState
		upToDate   bool
		summaryHas []string
		summaryNot []string
	}{
		{
			name:       "never checked",
			state:      updateState{},
			upToDate:   false,
			summaryHas: []string{"not checked"},
			summaryNot: []string{"up to date", "latest"},
		},
		{
			name:       "the check failed",
			state:      updateState{Failed: "dial tcp: no route to host"},
			upToDate:   false,
			summaryHas: []string{"unknown", "no route to host"},
			summaryNot: []string{"up to date"},
		},
		{
			name:       "behind",
			state:      updateState{Latest: "v0.5.1"},
			upToDate:   false,
			summaryHas: []string{"v0.5.1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.state.UpToDate(); got != tc.upToDate {
				t.Errorf("UpToDate() = %v, want %v", got, tc.upToDate)
			}
			summary := tc.state.Summary()
			if summary == "" {
				t.Fatal("Summary() is empty, so the row cannot say anything at all")
			}
			lower := strings.ToLower(summary)
			for _, want := range tc.summaryHas {
				if !strings.Contains(lower, strings.ToLower(want)) {
					t.Errorf("Summary() = %q, want it to carry %q", summary, want)
				}
			}
			for _, not := range tc.summaryNot {
				if strings.Contains(lower, strings.ToLower(not)) {
					t.Errorf("Summary() = %q, which contains %q: a state that is not known to be current must not read as current", summary, not)
				}
			}
		})
	}
}

func TestUpdateStateNamesTheAgeOfAStaleRecord(t *testing.T) {
	previous := Version
	Version = "v0.5.0"
	t.Cleanup(func() { Version = previous })

	now := time.Now()
	state := updateState{Latest: "v0.5.0", CheckedAt: now.Add(-30 * time.Hour)}

	if !state.Stale() {
		t.Fatal("a 30 hour old record is not reported stale, so a row would present it as this run's answer")
	}
	if !strings.Contains(strings.ToLower(state.Summary()), "up to date") {
		// The stale-but-equal case is the one place the words are allowed: the
		// answer is known, and the record says when it was read.
		t.Errorf("Summary() = %q, want the known-current case to say so", state.Summary())
	}
	if !strings.Contains(strings.ToLower(state.Summary()), "stale") &&
		!strings.Contains(state.Summary(), "30") {
		t.Errorf("Summary() = %q, want the age of the answer in it", state.Summary())
	}
}

// ---------------------------------------------------------------------------
// The assets
// ---------------------------------------------------------------------------

func TestDotfilesReleaseAssetNamesTheFourPublishedBinaries(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
		ok           bool
	}{
		{"linux", "amd64", "dotfiles-linux-amd64", true},
		{"linux", "arm64", "dotfiles-linux-arm64", true},
		{"darwin", "amd64", "dotfiles-darwin-amd64", true},
		{"darwin", "arm64", "dotfiles-darwin-arm64", true},
		{"linux", "386", "", false},
		{"windows", "amd64", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			got, ok := dotfilesReleaseAsset(tc.goos, tc.goarch)
			if ok != tc.ok || got != tc.want {
				t.Errorf("dotfilesReleaseAsset(%q, %q) = %q, %v, want %q, %v", tc.goos, tc.goarch, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestReleaseSumsLineFindsTheAssetByName(t *testing.T) {
	// The shapes are the two the releases actually carry: the checksums job
	// writes "./<artifact>/<asset>" and a hand-made file writes the bare name.
	sums := strings.Join([]string{
		sha256Of("arch64") + "  ./dotfiles-linux-arm64/dotfiles-linux-arm64",
		sha256Of("amd64") + "  ./dotfiles-linux-amd64/dotfiles-linux-amd64",
		sha256Of("darwin") + "  dotfiles-darwin-amd64",
		"",
	}, "\n")

	cases := []struct {
		asset string
		want  string
	}{
		{"dotfiles-linux-amd64", sha256Of("amd64")},
		{"dotfiles-linux-arm64", sha256Of("arch64")},
		{"dotfiles-darwin-amd64", sha256Of("darwin")},
	}
	for _, tc := range cases {
		t.Run(tc.asset, func(t *testing.T) {
			got, err := releaseSumsLine(sums, tc.asset)
			if err != nil {
				t.Fatalf("releaseSumsLine(%q): %v", tc.asset, err)
			}
			if got != tc.want {
				t.Errorf("hash = %q, want %q: the hash has to be matched by asset name, never by line number", got, tc.want)
			}
		})
	}

	if _, err := releaseSumsLine(sums, "dotfiles-windows-amd64"); err == nil {
		t.Error("a sums file with no line for the asset answered with a hash")
	}
	if _, err := releaseSumsLine("", "dotfiles-linux-amd64"); err == nil {
		t.Error("an empty sums file answered with a hash")
	}
	if _, err := releaseSumsLine("not a checksums file", "dotfiles-linux-amd64"); err == nil {
		t.Error("a file with no checksum lines answered with a hash")
	}
}

// ---------------------------------------------------------------------------
// Refusing a binary somebody else owns
// ---------------------------------------------------------------------------

func TestPackageManagerOwnerNamesHomebrew(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/opt/homebrew/Cellar/dotfiles/0.5.0/bin/dotfiles", "Homebrew"},
		{"/home/linuxbrew/.linuxbrew/Cellar/dotfiles/0.5.0/bin/dotfiles", "Homebrew"},
		{"/home/alber/.local/bin/dotfiles", ""},
		{"/usr/local/bin/dotfiles", ""},
		{"/opt/dotfiles/bin/dotfiles", ""},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := packageManagerOwner(tc.path); got != tc.want {
				t.Errorf("packageManagerOwner(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestApplySelfUpdateRefusesABinaryHomebrewOwns(t *testing.T) {
	isolateUpdateState(t)
	binDir := filepath.Join(t.TempDir(), "Cellar", "dotfiles", "0.5.0", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(binDir, "dotfiles")
	before := writeFakeBinary(t, path, "the binary Homebrew installed")

	c := newUpdateTestClient(map[string]string{
		dotfilesReleaseURLBase + "v0.5.1/" + releaseSumsAssetName: "irrelevant",
	})
	target := updateTarget{Path: path, Asset: "dotfiles-linux-amd64"}

	_, err := applySelfUpdate(c.client(), "v0.5.1", target)
	if err == nil {
		t.Fatal("the installer replaced a binary a package manager owns")
	}
	if !strings.Contains(err.Error(), "brew upgrade dotfiles") {
		t.Errorf("refusal = %q, want it to name the command that does own this binary: `brew upgrade dotfiles`", err.Error())
	}
	if !strings.Contains(err.Error(), "Homebrew") {
		t.Errorf("refusal = %q, want it to name who owns the file", err.Error())
	}
	if len(c.requests) != 0 {
		t.Errorf("the refusal still made %d requests: the ownership question is answered from the path, before anything is downloaded", len(c.requests))
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("read back: %v", readErr)
	}
	if string(after) != before {
		t.Error("the refused binary was modified")
	}
	if _, statErr := os.Stat(path + updatePreviousSuffix); statErr == nil {
		t.Error("a refused update left a previous-binary copy behind")
	}
}

// ---------------------------------------------------------------------------
// The swap
// ---------------------------------------------------------------------------

// updateFixture is a release served from memory plus a binary on disk, which is
// everything applySelfUpdate needs and nothing it does not.
type updateFixture struct {
	client  *updateTestClient
	target  updateTarget
	path    string
	asset   string
	before  string
	newBins string
	tag     string
}

func newUpdateFixture(t *testing.T, assetContent string) updateFixture {
	t.Helper()
	isolateUpdateState(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "dotfiles")
	before := writeFakeBinary(t, path, "the version that is installed")

	const (
		tag   = "v0.5.1"
		asset = "dotfiles-linux-amd64"
	)
	sums := sha256Of(assetContent) + "  ./" + asset + "/" + asset + "\n"

	return updateFixture{
		client: newUpdateTestClient(map[string]string{
			dotfilesReleaseURLBase + tag + "/" + releaseSumsAssetName: sums,
			dotfilesReleaseURLBase + tag + "/" + asset:                assetContent,
		}),
		target:  updateTarget{Path: path, Asset: asset},
		path:    path,
		asset:   asset,
		before:  before,
		newBins: assetContent,
		tag:     tag,
	}
}

func TestApplySelfUpdateRefusesATamperedAsset(t *testing.T) {
	// The sums file names the hash the release published; the asset that arrives
	// does not hash to it. Nothing may move.
	f := newUpdateFixture(t, "the bytes that were actually served")
	f.client.files[dotfilesReleaseURLBase+f.tag+"/"+releaseSumsAssetName] =
		sha256Of("the bytes the release published") + "  ./" + f.asset + "/" + f.asset + "\n"

	_, err := applySelfUpdate(f.client.client(), f.tag, f.target)
	if err == nil {
		t.Fatal("a binary that does not match the release's own checksum was installed")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "checksum") {
		t.Errorf("refusal = %q, want it to name the checksum", err.Error())
	}

	after, readErr := os.ReadFile(f.path)
	if readErr != nil {
		t.Fatalf("read back: %v", readErr)
	}
	if string(after) != f.before {
		t.Error("a refused update changed the installed binary")
	}
	if _, statErr := os.Stat(f.path + updatePreviousSuffix); statErr == nil {
		t.Error("a refused update left a previous-binary copy behind")
	}
	if leftovers := updateStagingLeftovers(t, filepath.Dir(f.path)); len(leftovers) != 0 {
		t.Errorf("a failed update left staging directories behind: %v", leftovers)
	}
}

func TestApplySelfUpdateReplacesTheBinaryAndKeepsTheOldOne(t *testing.T) {
	f := newUpdateFixture(t, "the version that was published")

	kept, err := applySelfUpdate(f.client.client(), f.tag, f.target)
	if err != nil {
		t.Fatalf("applySelfUpdate: %v", err)
	}

	after, readErr := os.ReadFile(f.path)
	if readErr != nil {
		t.Fatalf("read back: %v", readErr)
	}
	if string(after) != f.newBins {
		t.Errorf("installed binary = %q, want the verified release bytes %q", string(after), f.newBins)
	}
	info, statErr := os.Stat(f.path)
	if statErr != nil {
		t.Fatalf("stat: %v", statErr)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("installed binary mode = %v, want it executable", info.Mode().Perm())
	}
	if kept != f.path+updatePreviousSuffix {
		t.Errorf("kept path = %q, want %q", kept, f.path+updatePreviousSuffix)
	}
	previous, readErr := os.ReadFile(kept)
	if readErr != nil {
		t.Fatalf("read the kept binary: %v", readErr)
	}
	if string(previous) != f.before {
		t.Errorf("kept binary = %q, want the bytes that were replaced %q", string(previous), f.before)
	}
	if leftovers := updateStagingLeftovers(t, filepath.Dir(f.path)); len(leftovers) != 0 {
		t.Errorf("a successful update left staging directories behind: %v", leftovers)
	}
}

func TestApplySelfUpdateRefusesAnAssetTheReleaseDoesNotCarry(t *testing.T) {
	f := newUpdateFixture(t, "unused")
	f.client.files[dotfilesReleaseURLBase+f.tag+"/"+releaseSumsAssetName] =
		sha256Of("something else") + "  ./dotfiles-darwin-arm64/dotfiles-darwin-arm64\n"

	_, err := applySelfUpdate(f.client.client(), f.tag, f.target)
	if err == nil {
		t.Fatal("an asset the release does not list was installed anyway")
	}
	if !strings.Contains(err.Error(), f.asset) {
		t.Errorf("refusal = %q, want it to name the asset it could not find", err.Error())
	}
	after, readErr := os.ReadFile(f.path)
	if readErr != nil {
		t.Fatalf("read back: %v", readErr)
	}
	if string(after) != f.before {
		t.Error("a refused update changed the installed binary")
	}
}

// updateStagingLeftovers returns the staging directories left in dir.
func updateStagingLeftovers(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var leftovers []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), updateStagingPrefix) {
			leftovers = append(leftovers, entry.Name())
		}
	}
	return leftovers
}

// ---------------------------------------------------------------------------
// What --version says
// ---------------------------------------------------------------------------

func TestBuildLabelNamesTheCommitAndTheDate(t *testing.T) {
	version, commit, date := Version, BuildCommit, BuildDate
	t.Cleanup(func() { Version, BuildCommit, BuildDate = version, commit, date })

	Version, BuildCommit, BuildDate = "v0.5.1", "9f3c2a1d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b", "2026-10-04T12:00:00Z"
	label := BuildLabel()
	for _, want := range []string{"v0.5.1", "9f3c2a1", "2026-10-04"} {
		if !strings.Contains(label, want) {
			t.Errorf("BuildLabel() = %q, want it to carry %q: an update that cannot be checked against the release it came from is not verifiable", label, want)
		}
	}

	// A source build has neither, and must not pretend to.
	Version, BuildCommit, BuildDate = "dev", "", ""
	if label := BuildLabel(); !strings.Contains(label, "dev build") {
		t.Errorf("BuildLabel() = %q, want the dev build named", label)
	}

	Version, BuildCommit, BuildDate = "v0.5.1", "", ""
	if label := BuildLabel(); strings.Contains(label, "@") || strings.Contains(label, "unknown") {
		t.Errorf("BuildLabel() = %q, want no placeholder for a commit that was not injected", label)
	}
}

// ---------------------------------------------------------------------------
// The gate: a start may only ask on its own when the run is a real one
// ---------------------------------------------------------------------------

func TestAutoUpdateCheckIsArmedForEveryRealRun(t *testing.T) {
	isolateUpdateState(t)
	allowed := updateAutoCheckAllowed
	t.Cleanup(func() { updateAutoCheckAllowed = allowed })

	updateAutoCheckAllowed = func() bool { return true }

	// The animation gate must not decide this. DOTFILES_ANIM=0 is the user's own
	// reported symptom: the check was armed on that gate, so a quiet run never
	// asked. The check is a command, so arming it here cannot delay the first
	// frame whatever the gate says.
	for _, animating := range []bool{true, false} {
		m := NewModel()
		m.Animating = animating
		if cmd := m.updateCheckCmdFor(); cmd == nil {
			t.Errorf("Animating=%v: the run was not allowed to check for a release on its own", animating)
		}
	}
}

func TestAutoUpdateCheckIsNeverArmedInATestBinary(t *testing.T) {
	if updateAutoCheckAllowed() {
		t.Fatal("updateAutoCheckAllowed() is true inside a test binary: a test would reach the network and write to the machine's real state directory")
	}
}

func TestAutoUpdateCheckIsNotArmedWhileARecordIsFresh(t *testing.T) {
	isolateUpdateState(t)
	allowed := updateAutoCheckAllowed
	t.Cleanup(func() { updateAutoCheckAllowed = allowed })
	updateAutoCheckAllowed = func() bool { return true }

	if err := writeUpdateRecord(updateRecord{CheckedAt: updateClock(), Latest: "v0.5.1"}); err != nil {
		t.Fatalf("writeUpdateRecord: %v", err)
	}

	m := NewModel()
	m.Animating = true
	m.UpdateCheck = loadUpdateState()

	if cmd := m.updateCheckCmdFor(); cmd != nil {
		t.Error("a fresh record was ignored and the start asked GitHub again")
	}
}

// ---------------------------------------------------------------------------
// The screen
// ---------------------------------------------------------------------------

// updateRowsContain reports whether a screen's options offer a row. It is how a
// guard says "this row is offered" without depending on where in the list it is.
func updateRowsContain(options []string, row string) bool {
	for _, option := range options {
		if option == row {
			return true
		}
	}
	return false
}

// installableUpdateModel is the update screen with a newer release published and
// a binary at a path the installer owns, so the guards measure the offering half
// of the screen rather than a host that happens to be behind.
func installableUpdateModel(t *testing.T) Model {
	t.Helper()

	m := NewModel()
	m.Width, m.Height = 100, 40
	m.Screen = ScreenUpdate
	m.UpdateCheck = updateState{
		Latest:    "v9.9.9",
		CheckedAt: time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC),
	}
	m.UpdateTarget = updateTarget{Path: "/home/alber/.local/bin/dotfiles", Asset: "dotfiles-linux-amd64"}
	m.UpdateTargetErr = ""
	return m
}

// TestTheUtilitiesSectionNoLongerOffersTheUpdateRow is the regression for the
// move: the installer's own release is not a utility any more. The row is a
// button on the main menu, so the section must not carry it -- not even the
// sentence that used to announce it -- or the same job would have two homes.
func TestTheUtilitiesSectionNoLongerOffersTheUpdateRow(t *testing.T) {
	isolateUpdateState(t)

	m := NewModel()
	m.Width, m.Height = 100, 40
	m.Screen = ScreenUtilities
	m.UpdateCheck = updateState{Latest: "v9.9.9", CheckedAt: time.Now()}

	if updateRowsContain(m.GetCurrentOptions(), updateInstallerRow) {
		t.Fatalf("the utilities section still offers %q: %v", updateInstallerRow, m.GetCurrentOptions())
	}
	for _, entry := range m.utilitiesPanelEntries() {
		if entry.label == "Update" {
			t.Error("the utilities panel still carries an Update entry for the installer's own release")
		}
	}
	view := ansiEscape.ReplaceAllString(m.View(), "")
	if strings.Contains(view, updateInstallerRow) {
		t.Errorf("the rendered utilities section still names %q:\n%s", updateInstallerRow, view)
	}
}

// updateReadyModel is the main menu with a newer release published and a binary
// at a path the installer owns, so the guards measure the button rather than a
// host that happens to be current.
func updateReadyModel(t *testing.T) Model {
	t.Helper()

	m := NewModel()
	m.Width, m.Height = 100, 40
	m.Screen = ScreenMainMenu
	m.UpdateCheck = updateState{
		Latest:    "v9.9.9",
		CheckedAt: time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC),
	}
	m.UpdateTarget = updateTarget{Path: "/home/alber/.local/bin/dotfiles", Asset: "dotfiles-linux-amd64"}
	m.UpdateTargetErr = ""
	return m
}

// TestMainMenuOffersTheUpdateRowOnlyWhenAnUpdateIsAvailable is the offer rule:
// the main menu carries a button only when there is something for it to do -- a
// later release is published and this file is the installer's own to replace.
// A run that is current, or does not know, shows no row rather than one that
// would refuse when it was pressed.
func TestMainMenuOffersTheUpdateRowOnlyWhenAnUpdateIsAvailable(t *testing.T) {
	isolateUpdateState(t)

	quiet := NewModel()
	quiet.Screen = ScreenMainMenu
	if updateRowsContain(quiet.GetCurrentOptions(), updateInstallerRow) {
		t.Fatal("the main menu offered the update row with nothing published")
	}

	ready := updateReadyModel(t)
	if !updateRowsContain(ready.GetCurrentOptions(), updateInstallerRow) {
		t.Fatalf("the main menu did not offer the update row with a newer release: %v", ready.GetCurrentOptions())
	}

	installed := updateReadyModel(t)
	installed.UpdateCheck.Installed = true
	if updateRowsContain(installed.GetCurrentOptions(), updateInstallerRow) {
		t.Error("the update row is still offered after this run installed the published release")
	}

	// A binary Homebrew owns is not the installer's to replace, so the button is
	// withdrawn rather than offered and then refused.
	homebrew := updateReadyModel(t)
	homebrew.UpdateTarget = updateTarget{Path: "/opt/homebrew/Cellar/dotfiles/0.5.0/bin/dotfiles", Asset: "dotfiles-darwin-arm64"}
	if updateRowsContain(homebrew.GetCurrentOptions(), updateInstallerRow) {
		t.Error("the update row was offered over a binary a package manager owns")
	}
}

// TestMainMenuUpdateRowFitsTheMeasuredFloor is the fit guard for the row the main
// menu gained: measured at the 80x24 floor and the 60x20 the trainer documents as
// too small, with the restore row beside it and the update result already in its
// notice slot. The row and the notice may not push the menu past the frame on the
// size where the budget is tightest.
func TestMainMenuUpdateRowFitsTheMeasuredFloor(t *testing.T) {
	notice := "Updated to v9.9.9; the previous binary is kept at /home/alber/.local/bin/dotfiles.previous. " +
		"Restart dotfiles to run the new release."

	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"60x20", 60, 20},
	} {
		size := size
		t.Run(size.name, func(t *testing.T) {
			for _, withBackups := range []bool{false, true} {
				m := updateReadyModel(t)
				m.Width, m.Height = size.width, size.height
				if withBackups {
					m.AvailableBackups = append(m.AvailableBackups, testBackupInfo(), testBackupInfo())
				}
				if !updateRowsContain(m.GetCurrentOptions(), updateInstallerRow) {
					t.Fatalf("the update row is not offered: %v", m.GetCurrentOptions())
				}
				assertScreenFitsTerminal(t, "main-menu-update", size.width, size.height, m.View())

				// The result lands in the notice slot above the menu, so that state has
				// to fit the same frame as the row that produced it.
				m.UpdateCheck.Notice = notice
				assertScreenFitsTerminal(t, "main-menu-update-result", size.width, size.height, m.View())
			}
		})
	}
}

// TestALateUpdateRowKeepsTheCursorOnTheRowItNamed is the A5 lesson applied to the
// row that arrives after the first frame. The check is asynchronous, so with the
// cursor already on Utilities its answer inserts a row above it; an index-based
// cursor would then point at the update row and hand the next Enter to the wrong
// choice. The label is kept and re-found, so the key that was aimed at Utilities
// still lands on Utilities.
func TestALateUpdateRowKeepsTheCursorOnTheRowItNamed(t *testing.T) {
	isolateUpdateState(t)

	m := NewModel()
	m.Width, m.Height = 100, 40
	m.Screen = ScreenMainMenu
	m.UpdateTarget = updateTarget{Path: "/home/alber/.local/bin/dotfiles", Asset: "dotfiles-linux-amd64"}
	m.UpdateTargetErr = ""

	utilities := -1
	for i, option := range m.GetCurrentOptions() {
		if option == "Utilities" {
			utilities = i
			break
		}
	}
	if utilities < 0 {
		t.Fatalf("the main menu has no Utilities row: %v", m.GetCurrentOptions())
	}
	m.Cursor = utilities
	if got := m.selectedOption(); got != "Utilities" {
		t.Fatalf("the cursor starts on %q, want Utilities", got)
	}

	next, _ := m.Update(updateCheckMsg{record: updateRecord{
		CheckedAt: time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC),
		Latest:    "v9.9.9",
	}})
	after, ok := next.(Model)
	if !ok {
		t.Fatal("Update did not return a Model")
	}
	if got := after.selectedOption(); got != "Utilities" {
		t.Errorf("the late update row moved the cursor to %q, want Utilities: %v", got, after.GetCurrentOptions())
	}
}

// TestPressingTheUpdateRowRunsTheVerifiedSwap is the action rule: the row is a
// button, and pressing it starts the same verified swap `--self-update` runs --
// off the update loop -- without taking the run to another screen.
func TestPressingTheUpdateRowRunsTheVerifiedSwap(t *testing.T) {
	isolateUpdateState(t)
	m := updateReadyModel(t)

	row := -1
	for i, option := range m.GetCurrentOptions() {
		if option == updateInstallerRow {
			row = i
			break
		}
	}
	if row < 0 {
		t.Fatalf("the update row is not offered: %v", m.GetCurrentOptions())
	}
	m.Cursor = row

	next, cmd := m.handleMainMenuKeys("enter")
	if cmd == nil {
		t.Fatal("pressing the update row started nothing")
	}
	started, ok := next.(Model)
	if !ok {
		t.Fatal("the key handler did not return a model")
	}
	if started.Screen != ScreenMainMenu {
		t.Errorf("pressing the row moved to %v, want to stay on the main menu", started.Screen)
	}
	if !started.UpdateCheck.InFlight {
		t.Error("the swap was started but the model does not know it is running")
	}
}

// TestTheUpdateResultIsSaidWhereTheUserIs is the result rule the theme switch
// paid for: the outcome lands in the slot the press came from on the main menu,
// in one line, instead of replacing the screen with a result view.
func TestTheUpdateResultIsSaidWhereTheUserIs(t *testing.T) {
	isolateUpdateState(t)
	m := updateReadyModel(t)

	next, _ := m.Update(updateAppliedMsg{kept: "/home/alber/.local/bin/dotfiles.previous"})
	after, ok := next.(Model)
	if !ok {
		t.Fatal("Update did not return a Model")
	}
	if after.Screen != ScreenMainMenu {
		t.Errorf("the result took the run to %v, want the main menu", after.Screen)
	}
	if !after.UpdateCheck.Installed {
		t.Error("the published release was installed but the model still offers it")
	}
	if after.UpdateCheck.InFlight {
		t.Error("the update attempt finished but the in-flight mark is still set")
	}
	if !strings.Contains(after.UpdateCheck.Notice, ".previous") {
		t.Errorf("the result does not name the kept binary: %q", after.UpdateCheck.Notice)
	}
	view := ansiEscape.ReplaceAllString(after.View(), "")
	if !strings.Contains(view, "Updated to") {
		t.Errorf("the main menu does not show the update result:\n%s", view)
	}
	if updateRowsContain(after.GetCurrentOptions(), updateInstallerRow) {
		t.Error("the row is still offered after the update it performed")
	}
}

// TestAFailedUpdatePressCanBeAskedForAgain is the error half of the same rule:
// a refusal or a failed download is a result the menu reports, not a state that
// leaves the button permanently disabled. The in-flight mark is cleared, so the
// next press starts another attempt.
func TestAFailedUpdatePressCanBeAskedForAgain(t *testing.T) {
	isolateUpdateState(t)
	m := updateReadyModel(t)
	m.UpdateCheck.InFlight = true

	next, _ := m.Update(updateAppliedMsg{err: errors.New("dial tcp: no route to host")})
	after, ok := next.(Model)
	if !ok {
		t.Fatal("Update did not return a Model")
	}
	if after.UpdateCheck.InFlight {
		t.Error("the failed attempt left the in-flight mark set, so the button can never be pressed again")
	}
	if updateRowsContain(after.GetCurrentOptions(), updateInstallerRow) == false {
		t.Errorf("the failed attempt withdrew the update row instead of reporting the reason: %v", after.GetCurrentOptions())
	}
	if !strings.Contains(ansiEscape.ReplaceAllString(after.View(), ""), "no route to host") {
		t.Errorf("the main menu does not report why the update failed:\n%s", after.View())
	}
}

func TestTheUpdateScreenRefusesToInstallOverABinaryHomebrewOwns(t *testing.T) {
	isolateUpdateState(t)
	m := installableUpdateModel(t)
	// The Cellar path is what a resolved Homebrew symlink points at.
	m.UpdateTarget = updateTarget{
		Path:  "/opt/homebrew/Cellar/dotfiles/0.5.0/bin/dotfiles",
		Asset: "dotfiles-darwin-arm64",
	}

	if m.UpdateInstallable() {
		t.Fatal("the screen still considers this binary installable")
	}
	if updateRowsContain(m.GetCurrentOptions(), updateInstallRow) {
		t.Error("the install row was offered over a binary a package manager owns, so the press would only ever be refused")
	}

	view := ansiEscape.ReplaceAllString(m.View(), "")
	for _, want := range []string{"Homebrew", homebrewUpgradeCommand, m.UpdateTarget.Path} {
		if !strings.Contains(view, want) {
			t.Errorf("the refusal does not name %q:\n%s", want, view)
		}
	}
}

func TestTheUpdateScreenNeverClaimsUpToDateWithoutAnAnswer(t *testing.T) {
	isolateUpdateState(t)
	m := installableUpdateModel(t)
	m.UpdateCheck = updateState{
		CheckedAt: time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC),
		Failed:    "dial tcp: no route to host",
	}

	view := ansiEscape.ReplaceAllString(m.View(), "")
	if strings.Contains(strings.ToLower(view), "up to date") {
		t.Errorf("a failed check is drawn as an up-to-date install:\n%s", view)
	}
	for _, want := range []string{"unknown", "no route to host"} {
		if !strings.Contains(strings.ToLower(view), want) {
			t.Errorf("the screen does not carry %q:\n%s", want, view)
		}
	}
}

func TestTheUpdateScreenChecksOneAtATime(t *testing.T) {
	isolateUpdateState(t)
	m := installableUpdateModel(t)

	// The install row is first when it is offered, so the re-check row is the
	// second: the cursor is put on it explicitly rather than counted on.
	if options := m.GetCurrentOptions(); len(options) < 2 || options[1] != updateCheckRow {
		t.Fatalf("the re-check row is not where these guards look for it: %v", options)
	}
	m.Cursor = 1

	next, cmd := m.handleUpdateKeys("enter")
	if cmd == nil {
		t.Fatal("pressing the re-check row started nothing")
	}
	started, ok := next.(Model)
	if !ok {
		t.Fatal("the key handler did not return a model")
	}
	if !started.UpdateCheck.InFlight {
		t.Error("the check was started but the screen does not know it is running")
	}
	if updateRowsContain(started.GetCurrentOptions(), updateCheckRow) {
		t.Error("the re-check row is still offered while a check is running, so a second press cannot do anything")
	}
	if again := started.startUpdateCheck(); again != nil {
		t.Error("a second check was started beside the one that was already running")
	}
}

// ---------------------------------------------------------------------------
// The two commands behind the flags
// ---------------------------------------------------------------------------

func swapUpdateTransport(t *testing.T, c *updateTestClient) {
	t.Helper()
	saved := updateHTTPClient
	t.Cleanup(func() { updateHTTPClient = saved })
	updateHTTPClient = func() *http.Client { return c.client() }
}

func TestCheckForUpdateReportsWhatWasPublished(t *testing.T) {
	isolateUpdateState(t)
	c := newUpdateTestClient(map[string]string{dotfilesReleaseAPIURL: releaseLatestBody("v9.9.9")})
	swapUpdateTransport(t, c)

	report := CheckForUpdate()

	if !report.Known || !report.Newer {
		t.Fatalf("report = %+v, want a known newer release", report)
	}
	if !strings.Contains(report.Summary, "v9.9.9") {
		t.Errorf("summary = %q, want the published tag in it", report.Summary)
	}
}

func TestCheckForUpdateReportsUnknownRatherThanCurrent(t *testing.T) {
	isolateUpdateState(t)
	c := newUpdateTestClient(nil)
	c.failures[dotfilesReleaseAPIURL] = errors.New("dial tcp: no route to host")
	swapUpdateTransport(t, c)

	report := CheckForUpdate()

	if report.Known {
		t.Error("a failed check reported a known release")
	}
	if report.Newer {
		t.Error("a failed check reported a newer release")
	}
	if !strings.Contains(strings.ToLower(report.Summary), "unknown") {
		t.Errorf("summary = %q, want it to say the answer is unknown", report.Summary)
	}
}

func TestUpdateSelfRefusesABinaryHomebrewOwnsAndNamesTheCommand(t *testing.T) {
	isolateUpdateState(t)

	binDir := filepath.Join(t.TempDir(), "Cellar", "dotfiles", "0.5.0", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(binDir, "dotfiles")
	writeFakeBinary(t, path, "the binary Homebrew installed")

	saved := selfBinaryPath
	t.Cleanup(func() { selfBinaryPath = saved })
	selfBinaryPath = func() (string, error) { return path, nil }

	c := newUpdateTestClient(map[string]string{dotfilesReleaseAPIURL: releaseLatestBody("v9.9.9")})
	swapUpdateTransport(t, c)

	_, err := UpdateSelf()
	if err == nil {
		t.Fatal("the installer replaced the binary a package manager owns")
	}
	if !strings.Contains(err.Error(), homebrewUpgradeCommand) {
		t.Errorf("refusal = %q, want it to name `%s`", err.Error(), homebrewUpgradeCommand)
	}
	if len(c.requests) != 0 {
		t.Errorf("the refusal made %d requests before deciding: the ownership question is answered from the path", len(c.requests))
	}
}

// ---------------------------------------------------------------------------
// The two channels: stable is what shipped, dev is the newest pre-release
// ---------------------------------------------------------------------------
//
// The check has always read the latest published release. That is the stable
// channel, and it stays the default. The dev channel serves the newest
// pre-release instead: the release workflow builds its four assets from the tag a
// run is started from, so a pre-release is the artifact a downloading installer
// can verify, and it is what a build of main is published as. If no pre-release
// exists, dev says so rather than serving the stable release under a dev name.

func TestParseReleaseChannelNamesTheTwoChannels(t *testing.T) {
	cases := []struct {
		value string
		want  releaseChannel
		ok    bool
	}{
		{"", channelStable, true},
		{"stable", channelStable, true},
		{"STABLE", channelStable, true},
		{"dev", channelDev, true},
		{" Dev ", channelDev, true},
		{"nightly", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			got, err := ParseReleaseChannel(tc.value)
			if tc.ok && err != nil {
				t.Fatalf("ParseReleaseChannel(%q): %v", tc.value, err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("ParseReleaseChannel(%q) accepted an unknown channel as %q", tc.value, got)
			}
			if tc.ok && got != tc.want {
				t.Errorf("ParseReleaseChannel(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestDevChannelReadsTheNewestPreReleaseNotTheStableRelease(t *testing.T) {
	c := newUpdateTestClient(map[string]string{
		dotfilesReleaseAPIURL: releaseLatestBody("v1.0.0"),
		dotfilesReleaseListURL: `[
			{"tag_name":"v1.1.0","prerelease":false,"draft":false},
			{"tag_name":"v1.2.0-dev.1","prerelease":true,"draft":false}
		]`,
	})

	tag, err := latestReleaseTagForChannel(c.client(), channelDev)
	if err != nil {
		t.Fatalf("the dev channel could not be read: %v", err)
	}
	if tag != "v1.2.0-dev.1" {
		t.Errorf("dev tag = %q, want the newest pre-release v1.2.0-dev.1: a dev channel that answers with the stable release is the stable channel", tag)
	}
	if len(c.requests) != 1 || c.requests[0] != dotfilesReleaseListURL {
		t.Errorf("dev requests = %v, want exactly [%s]: the dev channel reads the release list, not /releases/latest", c.requests, dotfilesReleaseListURL)
	}
}

func TestDevChannelSkipsDraftsAndReleasedEntries(t *testing.T) {
	c := newUpdateTestClient(map[string]string{
		dotfilesReleaseListURL: `[
			{"tag_name":"v2.0.0-draft","prerelease":true,"draft":true},
			{"tag_name":"v1.9.0","prerelease":true,"draft":false}
		]`,
	})

	tag, err := latestReleaseTagForChannel(c.client(), channelDev)
	if err != nil {
		t.Fatalf("latestReleaseTagForChannel(dev): %v", err)
	}
	if tag != "v1.9.0" {
		t.Errorf("dev tag = %q, want v1.9.0: a draft is not published, so it is not an artifact this channel can serve", tag)
	}
}

func TestDevChannelSaysUnknownWhenNoPreReleaseIsPublished(t *testing.T) {
	c := newUpdateTestClient(map[string]string{
		dotfilesReleaseListURL: `[{"tag_name":"v1.0.0","prerelease":false,"draft":false}]`,
	})

	if _, err := latestReleaseTagForChannel(c.client(), channelDev); err == nil {
		t.Fatal("a release list with no pre-release answered with a tag: dev would serve the stable release under a dev name")
	}
}

func TestTheRecordNamesTheChannelTheAnswerCameFrom(t *testing.T) {
	isolateUpdateState(t)
	c := newUpdateTestClient(map[string]string{
		dotfilesReleaseListURL: `[{"tag_name":"v1.2.0-dev.1","prerelease":true,"draft":false}]`,
	})
	now := time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC)

	rec := refreshUpdateRecord(c.client(), channelDev, now)

	if rec.Channel != string(channelDev) {
		t.Errorf("record channel = %q, want dev", rec.Channel)
	}
	if rec.Latest != "v1.2.0-dev.1" {
		t.Errorf("record latest = %q, want the dev tag", rec.Latest)
	}
	back, ok := readUpdateRecord()
	if !ok {
		t.Fatal("the record was refreshed but not written")
	}
	if back.Channel != string(channelDev) {
		t.Errorf("stored channel = %q, want dev: an answer without its source is an opinion", back.Channel)
	}
}

func TestAChannelChoiceIsRememberedWithoutAnAnswer(t *testing.T) {
	isolateUpdateState(t)
	if err := writeUpdateRecord(updateRecord{Channel: string(channelDev)}); err != nil {
		t.Fatalf("writeUpdateRecord: %v", err)
	}

	rec, ok := readUpdateRecord()
	if !ok {
		t.Fatal("a record that names a channel but carries no answer was unreadable, so the choice is forgotten")
	}
	if recordChannel(rec) != channelDev {
		t.Errorf("remembered channel = %q, want dev", rec.Channel)
	}
}

func TestTheStateNamesTheDevChannel(t *testing.T) {
	state := stateFromRecord(updateRecord{Channel: "dev", CheckedAt: time.Now(), Latest: "v1.2.0-dev.1"})

	if state.Channel != channelDev {
		t.Fatalf("state.Channel = %q, want dev", state.Channel)
	}
	if !strings.Contains(strings.ToLower(state.Summary()), "dev") {
		t.Errorf("Summary() = %q, want it to name the dev channel: an answer without its source is an opinion", state.Summary())
	}
	found := false
	for _, row := range state.displayRows() {
		if row.Label == "Channel" && row.Value == string(channelDev) {
			found = true
		}
	}
	if !found {
		t.Errorf("displayRows() = %+v, want a Channel row naming dev", state.displayRows())
	}
}

func TestSwitchingToDevDropsTheStableAnswerAndRemembersTheChoice(t *testing.T) {
	isolateUpdateState(t)
	saved := updateChannelFlag
	t.Cleanup(func() { updateChannelFlag = saved })
	updateChannelFlag = ""

	if err := writeUpdateRecord(updateRecord{Channel: "stable", CheckedAt: time.Now(), Latest: "v1.0.0"}); err != nil {
		t.Fatalf("writeUpdateRecord: %v", err)
	}
	m := NewModel()
	m.UpdateCheck = loadUpdateState()
	if m.UpdateCheck.Latest != "v1.0.0" {
		t.Fatalf("setup: cached latest = %q", m.UpdateCheck.Latest)
	}

	switched := m.setUpdateChannel(channelDev)

	if switched.updateChannel() != channelDev {
		t.Errorf("channel after the switch = %q, want dev", switched.updateChannel())
	}
	if switched.UpdateCheck.Latest != "" {
		t.Errorf("the stable answer %q is still on the state: a stable answer is not a dev answer", switched.UpdateCheck.Latest)
	}
	back, ok := readUpdateRecord()
	if !ok || recordChannel(back) != channelDev {
		t.Errorf("the record = %+v, want it to remember dev before the new check lands", back)
	}
}

func TestDevChannelWithoutANetworkSaysUnknownNotCurrent(t *testing.T) {
	isolateUpdateState(t)
	c := newUpdateTestClient(nil)
	c.failures[dotfilesReleaseListURL] = errors.New("dial tcp: no route to host")

	rec := refreshUpdateRecord(c.client(), channelDev, time.Now())
	if rec.Latest != "" {
		t.Errorf("Latest = %q after a failed dev check, want empty: an unreachable GitHub is not a version", rec.Latest)
	}
	state := stateFromRecord(rec)
	if state.UpToDate() {
		t.Error("a dev check that could not reach GitHub reported the install as up to date")
	}
	if !strings.Contains(strings.ToLower(state.Summary()), "unknown") {
		t.Errorf("Summary() = %q, want unknown", state.Summary())
	}
	if !strings.Contains(strings.ToLower(state.Summary()), "dev") {
		t.Errorf("Summary() = %q, want the dev channel named even when the check failed", state.Summary())
	}
	if state.Channel != channelDev {
		t.Errorf("state.Channel = %q, want dev even on failure", state.Channel)
	}
}

func TestCheckForUpdateUsesTheChosenChannel(t *testing.T) {
	isolateUpdateState(t)
	saved := updateChannelFlag
	t.Cleanup(func() { updateChannelFlag = saved })
	updateChannelFlag = ""

	c := newUpdateTestClient(map[string]string{
		dotfilesReleaseAPIURL:  releaseLatestBody("v1.0.0"),
		dotfilesReleaseListURL: `[{"tag_name":"v1.2.0-dev.1","prerelease":true,"draft":false}]`,
	})
	swapUpdateTransport(t, c)
	if err := SetUpdateChannel("dev"); err != nil {
		t.Fatalf("SetUpdateChannel: %v", err)
	}

	report := CheckForUpdate()

	if report.Channel != string(channelDev) {
		t.Errorf("report channel = %q, want dev: --check-update must use the channel it was given", report.Channel)
	}
	if !report.Known || !report.Newer {
		t.Fatalf("report = %+v, want a known newer pre-release", report)
	}
	if !strings.Contains(report.Summary, "v1.2.0-dev.1") {
		t.Errorf("summary = %q, want the dev tag in it", report.Summary)
	}
	for _, req := range c.requests {
		if req == dotfilesReleaseAPIURL {
			t.Errorf("the dev check read %s: the dev channel must read the release list", dotfilesReleaseAPIURL)
		}
	}
}

func TestMainMenuOffersTheChannelRowAndSwitchesIt(t *testing.T) {
	isolateUpdateState(t)
	saved := updateChannelFlag
	t.Cleanup(func() { updateChannelFlag = saved })
	updateChannelFlag = ""

	m := NewModel()
	m.Width, m.Height = 100, 40
	m.Screen = ScreenMainMenu
	m.UpdateCheck = updateState{Channel: channelStable, CheckedAt: time.Now()}

	if !updateRowsContain(m.GetCurrentOptions(), updateChannelRow(channelStable)) {
		t.Fatalf("the main menu does not offer the channel row: %v", m.GetCurrentOptions())
	}

	row := -1
	for i, option := range m.GetCurrentOptions() {
		if option == updateChannelRow(channelStable) {
			row = i
			break
		}
	}
	m.Cursor = row

	next, _ := m.handleMainMenuKeys("enter")
	after, ok := next.(Model)
	if !ok {
		t.Fatal("the key handler did not return a model")
	}
	if after.updateChannel() != channelDev {
		t.Errorf("channel after the press = %q, want dev", after.updateChannel())
	}
	if !updateRowsContain(after.GetCurrentOptions(), updateChannelRow(channelDev)) {
		t.Errorf("the row does not name the channel it switched to: %v", after.GetCurrentOptions())
	}
	back, ok := readUpdateRecord()
	if !ok || recordChannel(back) != channelDev {
		t.Errorf("the press did not remember the channel: %+v", back)
	}
}

func TestMainMenuOffersNoChannelRowBeforeAChannelIsKnown(t *testing.T) {
	isolateUpdateState(t)
	saved := updateChannelFlag
	t.Cleanup(func() { updateChannelFlag = saved })
	updateChannelFlag = ""

	m := NewModel()
	m.Screen = ScreenMainMenu
	m.UpdateCheck = updateState{}

	for _, option := range m.GetCurrentOptions() {
		if strings.HasPrefix(option, updateChannelRowPrefix) {
			t.Fatalf("the main menu offered %q with no channel known: %v", option, m.GetCurrentOptions())
		}
	}
}

func TestACheckAnswerFromTheChannelThisRunLeftDoesNotOverwriteTheState(t *testing.T) {
	isolateUpdateState(t)
	saved := updateChannelFlag
	t.Cleanup(func() { updateChannelFlag = saved })
	updateChannelFlag = ""

	m := NewModel()
	m.UpdateCheck = updateState{Channel: channelDev, CheckedAt: time.Now()}

	// A stable answer arrives after the run switched to dev, because the older
	// check was still in flight when the row was pressed.
	next, _ := m.Update(updateCheckMsg{
		channel: channelStable,
		record:  updateRecord{Channel: "stable", CheckedAt: time.Now(), Latest: "v1.0.0"},
	})
	after, ok := next.(Model)
	if !ok {
		t.Fatal("Update did not return a Model")
	}
	if after.UpdateCheck.Channel != channelDev {
		t.Errorf("channel = %q, want dev: an answer from a channel this run left must not overwrite the state", after.UpdateCheck.Channel)
	}
	if after.UpdateCheck.Latest == "v1.0.0" {
		t.Error("the stable answer replaced the dev state")
	}
}

func TestTheAutomaticCheckReadsTheChannelTheRunFollows(t *testing.T) {
	isolateUpdateState(t)
	saved := updateChannelFlag
	t.Cleanup(func() { updateChannelFlag = saved })
	updateChannelFlag = ""

	c := newUpdateTestClient(map[string]string{
		dotfilesReleaseListURL: `[{"tag_name":"v1.2.0-dev.1","prerelease":true,"draft":false}]`,
	})
	swapUpdateTransport(t, c)

	m := NewModel()
	m.UpdateCheck = updateState{Channel: channelDev}
	cmd := m.startUpdateCheck()
	if cmd == nil {
		t.Fatal("startUpdateCheck returned no command")
	}

	msg, ok := cmd().(updateCheckMsg)
	if !ok {
		t.Fatalf("the command returned %T, want updateCheckMsg", cmd())
	}
	if msg.channel != channelDev {
		t.Errorf("the check was made on channel %q, want dev: the automatic check must read the channel the run follows", msg.channel)
	}
	if msg.record.Latest != "v1.2.0-dev.1" {
		t.Errorf("record latest = %q, want the dev tag", msg.record.Latest)
	}
	if !strings.Contains(c.requests[0], "/releases?") {
		t.Errorf("the automatic dev check read %s, want the release list", c.requests[0])
	}
}

// ---------------------------------------------------------------------------
// The installer and the workflow cannot drift apart
// ---------------------------------------------------------------------------

func TestTheWorkflowBuildsExactlyTheAssetsTheInstallerCanResolve(t *testing.T) {
	workflow := readRepoFile(t, filepath.Join(".github", "workflows", "release.yml"))
	built := workflowArtifacts(workflow)
	if len(built) == 0 {
		t.Fatal("release.yml names no artifacts, so this guard proves nothing")
	}

	resolve := map[string]bool{}
	for _, asset := range sortedUpdateAssets() {
		resolve[asset] = true
	}

	// Both directions: an asset the workflow builds that the installer cannot
	// resolve is a platform the self-update refuses, and one the installer
	// resolves that the workflow never builds is an asset name invented here.
	for _, asset := range built {
		if !resolve[asset] {
			t.Errorf("release.yml builds %s, but dotfilesReleaseAsset never resolves that name: a self-update on that platform would ask for a file no release carries", asset)
		}
	}
	for _, asset := range sortedUpdateAssets() {
		found := false
		for _, candidate := range built {
			if candidate == asset {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the installer resolves %s, but release.yml never builds it", asset)
		}
	}
}
