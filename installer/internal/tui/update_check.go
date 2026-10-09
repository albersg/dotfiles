package tui

// The installer's own update check.
//
// The installer is a release asset like the ones it installs, and this file is
// the half of the release process that lives in the binary: it reads which
// release is the latest, remembers the answer in a file so a start does not
// have to ask, and can replace this binary with the published one after
// verifying it.
//
// Three rules run through everything here, and each of them is a rule a release
// process can break silently:
//
//   - **A failed check is reported as unknown, never as current.** "You are up
//     to date" is a claim about GitHub. A run that could not reach GitHub says
//     why it does not know. A hole that is declared is survivable; a hole that
//     is silent is not.
//   - **Nothing is asked on the startup path.** The cached answer is read with
//     one file read, and the request itself runs behind a command on the same
//     gate the drawing uses, so the interface never waits on the network. A
//     record younger than updateCheckTTL means a start makes no request at all.
//   - **A binary another program owns is not replaced.** Homebrew installs
//     dotfiles into its own Cellar, and that copy belongs to Homebrew: the
//     installer says so and names `brew upgrade dotfiles` instead of writing
//     over a file it does not own. It is the same ownership question the theme
//     switch asks of a config file, answered from the path.

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// The build's identity
// ---------------------------------------------------------------------------

// BuildCommit and BuildDate are the rest of the build's identity, injected at
// link time by the release workflow beside main.Version
// (-X main.Commit=<sha> -X main.Date=<rfc3339>). They are why --version can be
// checked against a release at all: the tag names the release, and the commit
// says which bytes it was built from.
var (
	BuildCommit = ""
	BuildDate   = ""
)

// buildCommitLength is how much of the commit is shown. Seven characters is what
// git itself abbreviates to and what every other tool prints, so a reader can
// compare it against a log without counting.
const buildCommitLength = 7

// BuildLabel renders the whole identity for --version: the version, the commit it
// was built from and the day it was built.
//
// A part that was not injected is left out rather than filled in with a
// placeholder: "dev build" already says a source build has no release behind it,
// and a "unknown" beside it would be a second way of saying nothing. That is why
// a build with neither commit nor date prints exactly what it always printed.
func BuildLabel() string {
	label := VersionLabel()

	var parts []string
	if commit := displayCommit(BuildCommit); commit != "" {
		parts = append(parts, commit)
	}
	if date := displayBuildDate(BuildDate); date != "" {
		parts = append(parts, date)
	}
	if len(parts) == 0 {
		return label
	}
	return label + " (" + strings.Join(parts, ", ") + ")"
}

// displayCommit shortens a commit to the length git abbreviates to. A value that
// is not hexadecimal is returned unchanged: it is still what the build was told
// it is, and truncating something that is not a hash would invent a hash.
func displayCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return ""
	}
	if _, err := hex.DecodeString(padHex(commit)); err == nil && len(commit) > buildCommitLength {
		return commit[:buildCommitLength]
	}
	return commit
}

// padHex pads an odd-length string so hex.DecodeString can decide whether the
// value is hexadecimal. The padding is never returned.
func padHex(value string) string {
	if len(value)%2 == 1 {
		return value + "0"
	}
	return value
}

// displayBuildDate renders the build's timestamp as a day. It accepts the RFC
// 3339 the workflow injects and a bare date; anything else is dropped, because a
// value that is not a date cannot be read as one.
func displayBuildDate(date string) string {
	date = strings.TrimSpace(date)
	if date == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, date); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// The release this installer comes from
// ---------------------------------------------------------------------------

const (
	// dotfilesReleaseRepo is where the installer's own releases live. It is the
	// repository the binary is built from, not an upstream: a downstream build
	// checks downstream releases.
	dotfilesReleaseRepo = "albersg/dotfiles"

	// dotfilesReleaseAPIURL answers "which release is the latest". The API is
	// used rather than the /releases/latest redirect because the answer is then a
	// tag and not a Location header to parse. The latest endpoint excludes
	// pre-releases by definition, which is exactly what the stable channel wants.
	dotfilesReleaseAPIURL = "https://api.github.com/repos/" + dotfilesReleaseRepo + "/releases/latest"

	// dotfilesReleaseListURL answers "which releases exist", newest first, and it
	// is the list the dev channel reads: a pre-release is a release in this list
	// whose prerelease flag is set, and GitHub's latest endpoint cannot return one.
	dotfilesReleaseListURL = "https://api.github.com/repos/" + dotfilesReleaseRepo + "/releases?per_page=30"

	// dotfilesReleaseURLBase is the download prefix for one release's assets. The
	// tag is appended, then the asset name.
	dotfilesReleaseURLBase = "https://github.com/" + dotfilesReleaseRepo + "/releases/download/"

	// releaseSumsAssetName is the checksums file every release carries. It is the
	// chain that makes a download verifiable: the bytes are checked against the
	// line this file holds for their asset name.
	releaseSumsAssetName = "SHA256SUMS"

	// updateStateFile is this check's record, beside theme.json and
	// last-install.json, in the state directory. It is this program's own state
	// directory, so nothing here configures the installer.
	updateStateFile = "update-check.json"

	// updateCheckTTL is how long an answer is trusted. A day is short enough that
	// a release is noticed the next day and long enough that a machine which
	// starts the installer ten times does not ask GitHub ten times.
	updateCheckTTL = 24 * time.Hour

	// updateCheckTimeout bounds one request. The interface must never wait on the
	// network, so a GitHub that accepts a connection and then says nothing is
	// abandoned and reported as a failed check.
	updateCheckTimeout = 3 * time.Second

	// updatePreviousSuffix names the copy of the binary an update replaced, beside
	// it. It is kept rather than deleted -- that is the point of the suffix -- so a
	// release that turns out to be worse can be put back by hand.
	updatePreviousSuffix = ".previous"

	// updateStagingPrefix names the directory a download is assembled in before
	// anything moves. It sits in the binary's own directory, so the final rename
	// stays inside one filesystem and is atomic, exactly as the OfficeCLI step
	// stages beside its destination.
	updateStagingPrefix = ".dotfiles-update-"

	// homebrewOwner is what the ownership check calls the one package manager this
	// repository knows about.
	homebrewOwner = "Homebrew"

	// homebrewUpgradeCommand is what the refusal tells the user to run instead.
	homebrewUpgradeCommand = "brew upgrade dotfiles"
)

// updateDownloadMaxBytes bounds one download. The assets are a few megabytes; a
// release that answered with something far larger is not the release being
// asked for, and refusing a runaway body keeps a hostile or broken answer from
// filling the disk before the checksum ever runs. It is a variable so a guard
// can lower it instead of writing sixty-four megabytes.
var updateDownloadMaxBytes = int64(64 << 20)

// updateHTTPClient is the client every request goes through: the API read and
// both downloads. It is a function so a guard substitutes one whose transport
// serves from memory, which is how the whole path is exercised without a socket
// and without ever reaching the network.
var updateHTTPClient = func() *http.Client { return &http.Client{Timeout: updateCheckTimeout} }

// updateClock is the clock the record is written and judged with, so a guard
// pins a time instead of waiting a day for a record to go stale.
var updateClock = time.Now

// selfBinaryPath resolves the file an update would replace: the running
// executable with its symlinks followed, because a Homebrew install puts a
// symlink in bin/ and the Cellar is what actually owns the file.
var selfBinaryPath = defaultSelfBinaryPath

// updateAutoCheckAllowed is the gate the automatic check runs behind. It is true
// in a real run and false in this package's tests, which is what keeps a test
// from reaching the network and from writing into the machine's own state
// directory. It is a variable rather than a condition so both sides are
// reachable: update_check_test.go turns it off for the whole package, and the
// guard that pins the arming turns it back on for its own case.
var updateAutoCheckAllowed = func() bool { return true }

// defaultSelfBinaryPath is the real resolution: the running executable, with
// symlinks followed where that is possible.
func defaultSelfBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(exe); resolveErr == nil {
		return resolved, nil
	}
	return exe, nil
}

// dotfilesReleaseAsset maps a target platform to the release asset name. The four
// answers are the four artifacts .github/workflows/release.yml builds, named
// exactly as the matrix names them, so an asset name cannot be invented here that
// no release carries. The second value is false for a platform no release is
// built for, which the caller reports instead of guessing.
func dotfilesReleaseAsset(goos, goarch string) (string, bool) {
	switch goos {
	case "linux", "darwin":
		switch goarch {
		case "amd64", "arm64":
			return "dotfiles-" + goos + "-" + goarch, true
		}
	}
	return "", false
}

// updateTarget is the file a self-update would replace and the release asset that
// would replace it.
type updateTarget struct {
	Path  string
	Asset string
}

// resolveUpdateTarget resolves both halves for the running build.
func resolveUpdateTarget() (updateTarget, error) {
	path, err := selfBinaryPath()
	if err != nil {
		return updateTarget{}, fmt.Errorf("could not resolve the running binary: %w", err)
	}
	asset, ok := dotfilesReleaseAsset(runtime.GOOS, runtime.GOARCH)
	if !ok {
		return updateTarget{}, fmt.Errorf("no dotfiles release is built for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return updateTarget{Path: path, Asset: asset}, nil
}

// ---------------------------------------------------------------------------
// The record, and the answer it holds
// ---------------------------------------------------------------------------

// updateRecord is the installer's memory of one check: when it was made, the
// channel it was made on, the tag that was read, or the reason nothing could be
// read. Exactly one of Latest and Failed is set for a completed attempt, and
// neither is set for a record that was never written. A record may carry a
// channel and no answer: that is the remembered choice, written when the user
// switches channel and before the new check lands.
type updateRecord struct {
	// Channel is the channel this check followed: stable or dev. It is written
	// with every answer, so an answer without its source is not possible. A record
	// written before channels existed has no channel and reads as stable, which was
	// the only channel then.
	Channel   string    `json:"channel,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest,omitempty"`
	Failed    string    `json:"failed,omitempty"`
}

// updateState is what the model draws: the record's contents plus the state of
// the attempt that is running right now.
type updateState struct {
	// Channel is the channel the answer came from, and the channel this run is
	// following. It is empty until a record names one, which is why a run that has
	// never checked offers no channel row yet.
	Channel releaseChannel
	// Latest is the published tag. It is empty when nothing could be read, which
	// is the difference between "current" and "unknown".
	Latest string
	// CheckedAt is when the answer or the failure was obtained.
	CheckedAt time.Time
	// Failed is why there is no answer.
	Failed string
	// InFlight is true while a check is running, so a row can say so instead of
	// showing the answer it is about to replace.
	InFlight bool
	// Installed is true once this run has replaced its own binary with the
	// published release. The running process is still the old build until it is
	// restarted, but the file on disk is the published one, so the main menu must
	// stop offering the update it just performed.
	Installed bool
	// Notice is the one-line result of a self-update, cleared when the screen is
	// left, exactly as ThemeNotice is.
	Notice string
}

// updateStatePath is the exact file the record lives in. It is named once, so the
// writer, the reader and the prose cannot disagree.
func updateStatePath() string {
	dir := stateDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, updateStateFile)
}

// readUpdateRecord returns the stored record, or false when there is none. A
// missing, unreadable or corrupt file reads as no record: a cache that cannot be
// parsed is not an answer, and the check then runs again rather than reporting
// half a file.
func readUpdateRecord() (updateRecord, bool) {
	path := updateStatePath()
	if path == "" {
		return updateRecord{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return updateRecord{}, false
	}
	var rec updateRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return updateRecord{}, false
	}
	if rec.CheckedAt.IsZero() && rec.Channel == "" {
		return updateRecord{}, false
	}
	return rec, true
}

// writeUpdateRecord stores the record. Its error is reported to the caller rather
// than swallowed, because a refresh that cannot be remembered is a refresh whose
// result the next start pays for again.
func writeUpdateRecord(rec updateRecord) error {
	path := updateStatePath()
	if path == "" {
		return errors.New("could not determine the state directory")
	}
	if err := os.MkdirAll(filepath.Dir(path), stateDirMode); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, stateFileMode)
}

// loadUpdateState reads the cached answer. It is a file read and nothing else:
// the startup path calls it, so it must never wait on anything.
func loadUpdateState() updateState {
	rec, ok := readUpdateRecord()
	if !ok {
		return updateState{}
	}
	return stateFromRecord(rec)
}

// updateCheckDue reports whether a check is worth making now. It is what keeps a
// machine that starts the installer ten times from asking GitHub ten times: a
// record younger than updateCheckTTL is an answer, and a record that failed is an
// answer too -- the failure is retried only once it is stale.
func updateCheckDue(rec updateRecord, now time.Time) bool {
	if rec.CheckedAt.IsZero() {
		return true
	}
	return now.Sub(rec.CheckedAt) >= updateCheckTTL
}

// latestReleaseTag reads the latest published release's tag.
func latestReleaseTag(client *http.Client, apiURL string) (string, error) {
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", apiURL, resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", err
	}
	if strings.TrimSpace(body.TagName) == "" {
		return "", fmt.Errorf("GET %s: the answer carries no tag_name", apiURL)
	}
	return strings.TrimSpace(body.TagName), nil
}

// ---------------------------------------------------------------------------
// The two channels a run can follow
// ---------------------------------------------------------------------------

// releaseChannel names which stream of releases a run follows. It is a string
// type so the record, the flag and the row all spell it the reader's way.
type releaseChannel string

const (
	// channelStable is the default: the latest published release. It is what the
	// check has always read, and GitHub's latest endpoint cannot return a
	// pre-release, so following it is the same promise as before.
	channelStable releaseChannel = "stable"
	// channelDev is the newest pre-release. The release workflow builds its four
	// assets from the tag a run is started from, so a pre-release is the artifact
	// the dev channel serves -- and the only one the installer can verify, because
	// it downloads a published asset rather than compiling one.
	channelDev releaseChannel = "dev"
)

// updateChannelFlag is the --channel value main handed in, "" when the flag was
// not given. An empty value is not "stable": it leaves the choice to the channel
// the record remembers.
var updateChannelFlag = ""

// ParseReleaseChannel reads a channel name as the flag, the record and the row
// spell it. It is exported because --channel is the binary's own flag: main
// validates the value before a run starts, and an unknown channel is named
// rather than silently treated as stable. An empty value is the default, stable.
func ParseReleaseChannel(value string) (releaseChannel, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "stable":
		return channelStable, nil
	case "dev":
		return channelDev, nil
	}
	return "", fmt.Errorf("unknown release channel %q: use stable or dev", value)
}

// SetUpdateChannel records the --channel a run was started with. An empty value
// clears the flag, so a test can put the package back the way it found it.
func SetUpdateChannel(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		updateChannelFlag = ""
		return nil
	}
	channel, err := ParseReleaseChannel(value)
	if err != nil {
		return err
	}
	updateChannelFlag = string(channel)
	return nil
}

// recordChannel is the channel a stored record names. A record written before
// channels existed names none and reads as stable, because it was the only
// channel then.
func recordChannel(rec updateRecord) releaseChannel {
	if channel, err := ParseReleaseChannel(rec.Channel); err == nil {
		return channel
	}
	return channelStable
}

// chosenChannel resolves the channel a command-line run uses: the flag when it
// was given, the channel the record remembers otherwise, and stable when neither
// exists. It is the one place the two entries and the interface agree on which
// channel is being asked about.
func chosenChannel() releaseChannel {
	if updateChannelFlag != "" {
		if channel, err := ParseReleaseChannel(updateChannelFlag); err == nil {
			return channel
		}
	}
	if rec, ok := readUpdateRecord(); ok {
		return recordChannel(rec)
	}
	return channelStable
}

// updateChannel is the channel this run follows: the one the state names, the
// flag that was given, or stable. The state first, because a check that has
// landed is the answer the run last asked for.
func (m Model) updateChannel() releaseChannel {
	if m.UpdateCheck.Channel != "" {
		return m.UpdateCheck.Channel
	}
	if updateChannelFlag != "" {
		if channel, err := ParseReleaseChannel(updateChannelFlag); err == nil {
			return channel
		}
	}
	return channelStable
}

// toggle is the other channel, which is what pressing the main menu's channel
// row does.
func (ch releaseChannel) toggle() releaseChannel {
	if ch == channelDev {
		return channelStable
	}
	return channelDev
}

// latestPrereleaseTag reads the newest pre-release's tag from the releases list.
// The list is ordered newest first and carries drafts only for an authenticated
// caller, so the first non-draft entry with prerelease set is the answer. A list
// with no such entry is an error, not an empty answer: it is how the dev channel
// says there is nothing to serve rather than inventing a release.
func latestPrereleaseTag(client *http.Client, apiURL string) (string, error) {
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", apiURL, resp.Status)
	}
	var body []struct {
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
		Draft      bool   `json:"draft"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", err
	}
	for _, release := range body {
		if release.Draft || !release.Prerelease {
			continue
		}
		if tag := strings.TrimSpace(release.TagName); tag != "" {
			return tag, nil
		}
	}
	return "", fmt.Errorf("GET %s: no pre-release is published", apiURL)
}

// latestReleaseTagForChannel reads the tag the given channel offers: the latest
// published release for stable, the newest pre-release for dev. It is the one
// place the two channels differ, so nothing downstream can serve one channel's
// answer under the other channel's name.
func latestReleaseTagForChannel(client *http.Client, channel releaseChannel) (string, error) {
	if channel == channelDev {
		return latestPrereleaseTag(client, dotfilesReleaseListURL)
	}
	return latestReleaseTag(client, dotfilesReleaseAPIURL)
}

// refreshUpdateRecord makes one check and stores what it found. It never returns
// an error: a check that failed is a result -- the reason is what the record
// holds -- because the caller's job is to report honestly, not to fail a run
// over a release it could not read.
func refreshUpdateRecord(client *http.Client, channel releaseChannel, now time.Time) updateRecord {
	rec := updateRecord{Channel: string(channel), CheckedAt: now}
	tag, err := latestReleaseTagForChannel(client, channel)
	if err != nil {
		rec.Failed = err.Error()
	} else {
		rec.Latest = tag
	}
	// The write is best effort for the caller's sake, but not silent: it is the
	// only thing that keeps a failed check from being paid for on every start.
	_ = writeUpdateRecord(rec)
	return rec
}

// ---------------------------------------------------------------------------
// Comparing versions, and saying what is known
// ---------------------------------------------------------------------------

// versionIsNewer reports whether candidate is a later release than current.
//
// The comparison is numeric segment by segment, never textual: "0.10.0" is later
// than "0.9.9", which a string comparison gets backwards. A candidate that is not
// a version is not a newer release -- nothing published is not an update -- and a
// current build that is not a version either (the "dev" a source build carries)
// is behind every release, because a release exists and it does not.
func versionIsNewer(current, candidate string) bool {
	latest, ok := parseVersion(candidate)
	if !ok {
		return false
	}
	built, ok := parseVersion(current)
	if !ok {
		return true
	}
	return compareVersions(latest, built) > 0
}

// parseVersion reads a release tag into its numeric segments. The leading "v" is
// dropped, and a pre-release or build suffix after "-" or "+" is dropped with it:
// v0.6.0-rc1 is the 0.6.0 release as far as "is there something newer" is
// concerned, and it is the latest published release by definition.
func parseVersion(version string) ([]int, bool) {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(strings.TrimPrefix(version, "v"), "V")
	if version == "" {
		return nil, false
	}
	if at := strings.IndexAny(version, "-+"); at >= 0 {
		version = version[:at]
	}
	if version == "" {
		return nil, false
	}

	parts := strings.Split(version, ".")
	segments := make([]int, 0, len(parts))
	for _, part := range parts {
		segment, err := strconv.Atoi(part)
		if err != nil || segment < 0 {
			return nil, false
		}
		segments = append(segments, segment)
	}
	return segments, true
}

// compareVersions orders two parsed versions, treating a missing segment as zero
// so "1.0" and "1.0.0" are the same release.
func compareVersions(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		var left, right int
		if i < len(a) {
			left = a[i]
		}
		if i < len(b) {
			right = b[i]
		}
		if left != right {
			if left > right {
				return 1
			}
			return -1
		}
	}
	return 0
}

// Present reports whether this run has anything to say about the published
// release: an answer, a failure, or a check that is running. It is the row's gate,
// so a run that has never checked and cannot check shows no row rather than a row
// that opens onto nothing.
func (u updateState) Present() bool {
	return !u.CheckedAt.IsZero() || u.InFlight
}

// answered reports whether a tag was actually read.
func (u updateState) answered() bool { return u.Latest != "" }

// Newer reports whether a later release than this build is published. It is false
// whenever nothing was read -- a build that does not know must not be told to
// update -- and false once this run has already installed the published release,
// even though the running process is still the old build until it restarts.
func (u updateState) Newer() bool {
	return u.answered() && !u.Installed && versionIsNewer(Version, u.Latest)
}

// UpToDate reports whether this build is the published one. It is true only when
// a tag was read and is not newer, which is the only case in which "up to date"
// is a claim about GitHub rather than about this process.
func (u updateState) UpToDate() bool { return u.answered() && !versionIsNewer(Version, u.Latest) }

// Stale reports whether the answer is older than the lifetime the record is
// trusted for. A stale answer is still an answer, and it is declared, so a row
// can carry the age beside it. A record that was never written is due a check and
// therefore stale as well: Present() is what tells the two apart.
func (u updateState) Stale() bool {
	return updateCheckDue(updateRecord{CheckedAt: u.CheckedAt}, updateClock())
}

// Summary is the one honest line about the published release: what is known, how
// old it is, and -- when nothing could be read -- why not. It never says "up to
// date" without an answer to say it about.
func (u updateState) Summary() string {
	age := ""
	if !u.CheckedAt.IsZero() {
		age = " (" + formatUpdateAge(updateClock().Sub(u.CheckedAt)) + ")"
	}

	// The channel is named only when it is not the default. Stable is what the
	// check has always read, so naming it every time would be noise; a dev answer
	// without the word dev is an answer without its source.
	source := ""
	if u.Channel == channelDev {
		source = "dev channel: "
	}

	switch {
	case u.InFlight:
		return source + "Checking the published release…"
	case u.Newer():
		return source + fmt.Sprintf("%s is published; this build is %s%s", u.Latest, VersionLabel(), age)
	case u.answered():
		line := fmt.Sprintf("Up to date with %s", u.Latest)
		if u.Stale() {
			line += " — that answer is stale"
		}
		return source + line + age
	case u.Failed != "":
		return source + fmt.Sprintf("Unknown — the published release could not be read: %s%s", u.Failed, age)
	default:
		return source + "Not checked yet — this build does not know what has been published"
	}
}

// formatUpdateAge writes how long ago an answer was read, in the largest unit
// that keeps it one number.
func formatUpdateAge(age time.Duration) string {
	switch {
	case age < 0:
		return "just now"
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return strconv.Itoa(int(age.Minutes())) + "m ago"
	case age < 48*time.Hour:
		return strconv.Itoa(int(age.Hours())) + "h ago"
	default:
		return strconv.Itoa(int(age.Hours()/24)) + "d ago"
	}
}

// ---------------------------------------------------------------------------
// Reading the release's own checksums
// ---------------------------------------------------------------------------

// releaseSumsLine returns the SHA-256 the release's SHA256SUMS file records for
// asset. It matches by file name and never by position: the checksums job writes
// the lines in the filesystem's order, so the fourth line is not the fourth
// asset. Two lines for the same name with different hashes are refused rather
// than resolved, because a file that disagrees with itself proves nothing.
func releaseSumsLine(sums, asset string) (string, error) {
	found := ""
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 {
			continue
		}
		if !isSHA256Hex(fields[0]) {
			continue
		}
		if filepath.Base(strings.TrimPrefix(fields[1], "*")) != asset {
			continue
		}
		if found != "" && found != fields[0] {
			return "", fmt.Errorf("the release's %s file records two different checksums for %s", releaseSumsAssetName, asset)
		}
		found = fields[0]
	}
	if found == "" {
		return "", fmt.Errorf("the release's %s file records no checksum for %s", releaseSumsAssetName, asset)
	}
	return found, nil
}

// isSHA256Hex reports whether a value is a SHA-256 in hexadecimal, in either
// case. A line whose hash field is not one is not a checksums line.
func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(strings.ToLower(value))
	return err == nil
}

// ---------------------------------------------------------------------------
// Who owns the binary
// ---------------------------------------------------------------------------

// homebrewRoots are the prefixes a Homebrew installation keeps its Cellar under:
// Apple silicon, Intel macOS, Linuxbrew, and the Cellar itself.
var homebrewRoots = []string{
	"/opt/homebrew/",
	"/usr/local/Homebrew/",
	"/usr/local/Cellar/",
	"/home/linuxbrew/.linuxbrew/",
}

// packageManagerOwner names the program that installed the binary at path, or
// returns "" when the binary is the installer's own to replace. The answer comes
// from the path and nothing else: the running binary has already had its symlinks
// followed, so a Homebrew install is seen at its Cellar path even though the user
// runs the symlink in bin/.
func packageManagerOwner(path string) string {
	clean := filepath.Clean(path)
	for _, root := range homebrewRoots {
		if strings.HasPrefix(clean, root) {
			return homebrewOwner
		}
	}
	if strings.Contains(clean, "/Cellar/") {
		return homebrewOwner
	}
	return ""
}

// packageManagerRefusal is the refusal for a binary a package manager owns. It
// names the file, who owns it, the command that does own the update, and the way
// out for someone who did not use a package manager. A refusal that says only
// "no" is a closed door with no sign on it.
func packageManagerRefusal(owner, path string) error {
	upgrade := "your package manager's upgrade command"
	if owner == homebrewOwner {
		upgrade = homebrewUpgradeCommand
	}
	return fmt.Errorf(
		"%s was installed by %s, so this copy at %s is not the installer's to replace; "+
			"what you can do: run `%s` -- the tap carries this release -- "+
			"or install the release asset yourself if you want to manage this file on your own",
		"dotfiles", owner, path, upgrade)
}

// ---------------------------------------------------------------------------
// The swap
// ---------------------------------------------------------------------------

// applySelfUpdate replaces the binary at target.Path with the published asset
// named by target.Asset from release tag, and returns the path of the copy of the
// binary it kept.
//
// The order is the one the OfficeCLI step already uses, for the same reasons:
//
//  1. The ownership question is answered first, from the path, before a single
//     byte is fetched. A binary Homebrew owns is never downloaded over.
//  2. The download and the checksums file are staged beside the destination, so
//     the final rename stays inside one filesystem and is atomic.
//  3. The asset's bytes are verified against the release's own SHA256SUMS for its
//     asset name before anything is made executable or moved.
//  4. The binary being replaced is copied aside, and then the verified bytes are
//     renamed over it. A rename is the swap; there is no window in which the
//     destination is missing or half written.
//
// Every failure after step 2 leaves the staging directory removed by defer and
// the destination untouched except for its kept copy, and step 4 is the only
// write to the destination itself.
func applySelfUpdate(client *http.Client, tag string, target updateTarget) (string, error) {
	if owner := packageManagerOwner(target.Path); owner != "" {
		return "", packageManagerRefusal(owner, target.Path)
	}
	if tag == "" {
		return "", errors.New("no release tag to update to")
	}
	if target.Asset == "" {
		return "", errors.New("no release asset to update from")
	}

	dir := filepath.Dir(target.Path)
	staging, err := os.MkdirTemp(dir, updateStagingPrefix)
	if err != nil {
		return "", fmt.Errorf("could not create a staging directory beside %s: %w", target.Path, err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	sumsURL := dotfilesReleaseURLBase + tag + "/" + releaseSumsAssetName
	sumsPath := filepath.Join(staging, releaseSumsAssetName)
	if err := downloadReleaseFile(client, sumsURL, sumsPath); err != nil {
		return "", err
	}
	sums, err := os.ReadFile(sumsPath)
	if err != nil {
		return "", fmt.Errorf("could not read the downloaded %s: %w", releaseSumsAssetName, err)
	}
	expected, err := releaseSumsLine(string(sums), target.Asset)
	if err != nil {
		return "", err
	}

	assetURL := dotfilesReleaseURLBase + tag + "/" + target.Asset
	staged := filepath.Join(staging, target.Asset)
	if err := downloadReleaseFile(client, assetURL, staged); err != nil {
		return "", err
	}
	if err := verifyFileSHA256(staged, expected); err != nil {
		return "", fmt.Errorf(
			"the downloaded %s does not match the %s the release publishes for it: %w",
			target.Asset, releaseSumsAssetName, err)
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return "", fmt.Errorf("could not mark the verified binary executable: %w", err)
	}

	// Keep the binary being replaced before the swap, so the release that was
	// installed can be put back by hand if the new one is worse.
	kept := target.Path + updatePreviousSuffix
	if err := copyUpdateFile(target.Path, kept); err != nil {
		return "", fmt.Errorf("could not keep a copy of the binary being replaced: %w", err)
	}
	if err := os.Rename(staged, target.Path); err != nil {
		return "", fmt.Errorf("could not move the verified binary into place: %w", err)
	}
	return kept, nil
}

// downloadReleaseFile fetches url into dest through the same client every other
// request uses, so the whole path is exercised by a guard without a socket. A
// non-OK status is an error rather than a body to interpret: a 404 page is not a
// binary. The body is bounded, so a runaway answer cannot fill the disk before
// the checksum runs.
func downloadReleaseFile(client *http.Client, url, dest string) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("could not download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not download %s: %s", url, resp.Status)
	}

	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("could not create %s: %w", dest, err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(resp.Body, updateDownloadMaxBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("could not download %s: %w", url, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("could not write %s: %w", dest, closeErr)
	}
	if written > updateDownloadMaxBytes {
		return fmt.Errorf("could not download %s: the answer is larger than %d bytes, which is not an asset this installer publishes", url, updateDownloadMaxBytes)
	}
	return nil
}

// copyUpdateFile copies src to dest with the source's permissions. It is how the
// replaced binary is kept: a copy leaves the destination in place, so the swap
// that follows is a single rename.
func copyUpdateFile(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return err
	}
	return target.Close()
}

// ---------------------------------------------------------------------------
// The commands the interface runs
// ---------------------------------------------------------------------------

// updateCheckMsg carries one finished check. It carries the record rather than an
// error because a failed check is a result: the message says what was found, and
// "nothing" comes with its reason.
type updateCheckMsg struct {
	channel releaseChannel
	record  updateRecord
}

// updateAppliedMsg carries the outcome of a self-update: the path of the binary
// that was kept, or why nothing was replaced.
type updateAppliedMsg struct {
	kept string
	err  error
}

// updateCheckCmd performs one check off the update loop. It is a command and not
// a call on the startup path because the network must never be waited on.
func updateCheckCmd(channel releaseChannel) tea.Cmd {
	return func() tea.Msg {
		return updateCheckMsg{channel: channel, record: refreshUpdateRecord(updateHTTPClient(), channel, updateClock())}
	}
}

// startUpdateCheck marks a check in flight and returns the command that makes it,
// or nil when one is already running. Every entry point that asks again -- the
// row on the screen, the row on the section, the automatic arm -- goes through
// this one place, so they cannot disagree about the mark or start two at once.
func (m *Model) startUpdateCheck() tea.Cmd {
	if m.UpdateCheck.InFlight {
		return nil
	}
	m.UpdateCheck.InFlight = true
	return updateCheckCmd(m.updateChannel())
}

// updateCheckCmdFor returns the check a start may make on its own, or nil.
//
// It is armed for every real run, whether or not the run may animate: the user
// asked for the check to happen when dotfiles opens, and a run with
// DOTFILES_ANIM=0 or a piped stdout is still an open. The request is a command,
// never a call here, so it cannot delay the first frame, and a failed check is
// recorded rather than shown so a dev build with no network is not handed an
// error it did not ask for. The record must actually be due, and a test run
// never reaches the network. `dotfiles --check-update` remains the scriptable
// form.
func (m Model) updateCheckCmdFor() tea.Cmd {
	if !updateAutoCheckAllowed() {
		return nil
	}
	if m.UpdateCheck.InFlight {
		return nil
	}
	if !updateCheckDue(updateRecord{CheckedAt: m.UpdateCheck.CheckedAt}, updateClock()) {
		return nil
	}
	return updateCheckCmd(m.updateChannel())
}

// updateApplyCmd downloads, verifies and swaps the binary off the update loop, so
// a slow download cannot freeze the interface.
func updateApplyCmd(tag string, target updateTarget) tea.Cmd {
	return func() tea.Msg {
		kept, err := applySelfUpdate(updateHTTPClient(), tag, target)
		return updateAppliedMsg{kept: kept, err: err}
	}
}

// ---------------------------------------------------------------------------
// The same two jobs, shaped for a command line
// ---------------------------------------------------------------------------
//
// `dotfiles --check-update` and `dotfiles --self-update` are the scriptable form
// of what the utilities section offers, and they run the code above rather than a
// second copy of it: one function, two entries. A second route would be a second
// set of checks, and two routes diverge.

// UpdateReport is the result of one check, shaped for a command line: the honest
// sentence, the channel it came from, and the two facts a script reads it for.
type UpdateReport struct {
	// Summary is the one-line answer, the same sentence the interface shows.
	Summary string
	// Channel is the channel the answer was read from: stable or dev.
	Channel string
	// Known is false when the answer could not be read. A caller must check it
	// before treating Newer as meaningful: a failed check is not a green light.
	Known bool
	// Newer is true when a later release than this build is published.
	Newer bool
}

// CheckForUpdate asks GitHub which release is the latest and reports it beside
// this build. It always asks, because it was asked for now; the cache is what
// keeps the *automatic* check off the startup path, not what keeps this one from
// answering.
func CheckForUpdate() UpdateReport {
	channel := chosenChannel()
	state := stateFromRecord(refreshUpdateRecord(updateHTTPClient(), channel, updateClock()))
	return UpdateReport{Summary: state.Summary(), Channel: string(channel), Known: state.answered(), Newer: state.Newer()}
}

// UpdateSelf installs the latest published release over this binary and returns
// the sentence describing what happened.
//
// It is honest about every branch it can take: nothing to do when this build is
// the published one, the refusal when Homebrew owns the file (with the command
// that does own the update), the reason when the release could not be read, and
// where the replaced binary was kept when the swap succeeded.
func UpdateSelf() (string, error) {
	target, err := resolveUpdateTarget()
	if err != nil {
		return "", err
	}
	// The ownership question is answered before any request: a copy Homebrew
	// installed is never replaced, whatever the answer turns out to be.
	if owner := packageManagerOwner(target.Path); owner != "" {
		return "", packageManagerRefusal(owner, target.Path)
	}

	state := loadUpdateState()
	channel := chosenChannel()
	if !state.answered() || !state.Newer() || state.Channel != channel {
		state = stateFromRecord(refreshUpdateRecord(updateHTTPClient(), channel, updateClock()))
	}

	switch {
	case !state.answered():
		return "", fmt.Errorf("the published release could not be read: %s", state.Failed)
	case !state.Newer():
		return fmt.Sprintf(
			"dotfiles %s is the published release (%s); nothing to do",
			VersionLabel(), target.Path), nil
	}

	if dryRun() {
		return fmt.Sprintf(
			"dry run: %s would be verified against the release's %s and installed over %s; the binary being replaced would be kept as %s",
			state.Latest, releaseSumsAssetName, target.Path, target.Path+updatePreviousSuffix), nil
	}

	kept, err := applySelfUpdate(updateHTTPClient(), state.Latest, target)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"dotfiles %s installed over %s; the binary it replaced is kept at %s",
		state.Latest, target.Path, kept), nil
}

// stateFromRecord folds a stored or freshly read record into the state the
// interface draws.
func stateFromRecord(rec updateRecord) updateState {
	return updateState{Channel: recordChannel(rec), Latest: rec.Latest, CheckedAt: rec.CheckedAt, Failed: rec.Failed}
}

// setUpdateChannel changes which channel this run follows. The choice is written
// to the same record the answers live in before any request, so quitting before
// the check lands does not forget it; the answer that belonged to the channel
// being left is dropped, because a stable answer is not a dev answer.
func (m Model) setUpdateChannel(channel releaseChannel) Model {
	notice := m.UpdateCheck.Notice
	m.UpdateCheck = updateState{Channel: channel, Notice: notice}
	// The write is best effort, exactly as the check's own is: the choice applies
	// to this run either way, and the next check writes the record whole.
	_ = writeUpdateRecord(updateRecord{Channel: string(channel)})
	return m
}

// updateCheckDisplayRows returns the state's facts as label/value rows for the
// screen's panel, in the order they are worth reading: what this build is, what
// is published, when that was read, and -- when it was not -- why not.
func (u updateState) displayRows() []updateDisplayRow {
	rows := []updateDisplayRow{
		{Label: "This build", Value: BuildLabel()},
	}

	// The channel is a fact like any other: the screen names the source of every
	// answer it shows. It is left out only when no record has named one.
	if u.Channel != "" {
		rows = append(rows, updateDisplayRow{Label: "Channel", Value: string(u.Channel)})
	}

	switch {
	case u.InFlight:
		rows = append(rows, updateDisplayRow{Label: "Published", Value: "checking…"})
	case u.answered():
		rows = append(rows, updateDisplayRow{Label: "Published", Value: u.Latest})
	case u.Failed != "":
		rows = append(rows, updateDisplayRow{Label: "Published", Value: "unknown"})
	default:
		rows = append(rows, updateDisplayRow{Label: "Published", Value: "not checked yet"})
	}

	if !u.CheckedAt.IsZero() {
		when := u.CheckedAt.UTC().Format("2006-01-02 15:04 MST")
		if u.Stale() {
			when += " (stale)"
		}
		rows = append(rows, updateDisplayRow{Label: "Checked", Value: when})
	}
	if u.Failed != "" {
		rows = append(rows, updateDisplayRow{Label: "Why not", Value: u.Failed})
	}
	if u.Notice != "" {
		rows = append(rows, updateDisplayRow{Label: "Last update", Value: u.Notice})
	}
	return rows
}

// updateDisplayRow is one label/value pair of the update screen's panel. The
// label is what the fact is, the value is the fact.
type updateDisplayRow struct {
	Label string
	Value string
}

// sortedUpdateAssets returns the asset names a release carries, derived from the
// build matrix's four targets rather than written out, in a stable order. It is
// what the documentation guard asks for when it checks that the runbook names
// every asset the workflow uploads.
func sortedUpdateAssets() []string {
	var assets []string
	for _, goos := range []string{"darwin", "linux"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			if asset, ok := dotfilesReleaseAsset(goos, goarch); ok {
				assets = append(assets, asset)
			}
		}
	}
	sort.Strings(assets)
	return assets
}
