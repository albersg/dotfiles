package tui

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The guard below reads release prose from the repository and compares it with
// the workflow that is supposed to make it true. It exists because a release
// document is prose about a machine, and prose about a machine goes stale
// silently: docs/contributing.md once told the reader to hand-build
// `installer/dotfiles-installer-<os>-<arch>` binaries and attach them with a
// manual `gh release create`, while .github/workflows/release.yml builds
// `dotfiles-<os>-<arch>` from the tag and creates the draft release itself. A
// reader who followed the old page produced files no release ever carried.

// releaseProseDocs are the documents that describe how a release is cut. The
// asset names they may name and the steps they may describe both have to match
// release.yml, so every one of them is checked.
//
// The list used to be three: docs/RELEASING.md, docs/release-checklist.md and
// docs/contributing.md. The checklist was retired the way docs/RELEASES.md was
// - the links were repointed at the one runbook first and the file was removed
// only after that - so the list is two now, and the checks below move with it:
// the runbook has to name every job the workflow declares and carry both the
// pre-flight and the after-the-tag half it absorbed, and the document that hands
// the mechanics over has to keep pointing at it. A shorter list of documents is
// only worth anything if the guards on it got stronger, which is what these do.
var releaseProseDocs = []string{
	filepath.Join("docs", "RELEASING.md"),
	filepath.Join("docs", "contributing.md"),
}

// workflowArtifactLineRE matches a build-matrix entry's artifact name: the
// `artifact:` key indented under `matrix.include`. Matching the key rather than
// any occurrence of a `dotfiles-` string is what keeps `${{ matrix.artifact }}`
// references out of the derived set.
var workflowArtifactLineRE = regexp.MustCompile(`(?m)^[ \t]+artifact:[ \t]+(\S+)[ \t]*$`)

// workflowJobLineRE matches a job's name: the key indented two spaces under
// `jobs:`. The section is sliced out before this runs, because `on:`'s own keys
// are indented the same way and are not jobs.
var workflowJobLineRE = regexp.MustCompile(`(?m)^  ([a-z][a-z0-9-]*):[ \t]*$`)

// workflowLineCitationRE matches a citation of a line in the workflow, the shape
// the retired pages used (`release.yml:49-54`). A line number is a claim that
// goes stale the moment anything above it moves, which is exactly how
// docs/RELEASES.md came to describe a workflow that no longer existed.
var workflowLineCitationRE = regexp.MustCompile(`release\.yml:[0-9]`)

// TestReleaseDocsNameTheWorkflowArtifacts guards the class of defect where a
// release document names an artifact the workflow never builds. The four asset
// names come from release.yml's build matrix, so a target added, renamed or
// removed there moves this test instead of leaving a document to promise a file
// that is never uploaded.
//
// It also rejects the two shapes the retired procedure used: the
// `dotfiles-installer-` prefix no matrix entry carries, and a manual
// `gh release create`, which the workflow's release job does for the reader.
func TestReleaseDocsNameTheWorkflowArtifacts(t *testing.T) {
	workflow := readRepoFile(t, filepath.Join(".github", "workflows", "release.yml"))

	assets := workflowArtifacts(workflow)
	if len(assets) == 0 {
		t.Fatal("no `artifact:` names were found in release.yml's build matrix, so this guard proves nothing")
	}
	t.Logf("release.yml builds and uploads: %v", assets)

	runbook := readRepoFile(t, filepath.Join("docs", "RELEASING.md"))
	for _, asset := range assets {
		if !strings.Contains(runbook, asset) {
			t.Errorf("release.yml builds %s, but docs/RELEASING.md never names it.\n"+
				"The runbook has to list every asset the workflow uploads; add %s to the table of what it builds.",
				asset, asset)
		}
	}

	for _, doc := range releaseProseDocs {
		text := readRepoFile(t, doc)
		if strings.Contains(text, "dotfiles-installer-") {
			t.Errorf("%s names an artifact the release workflow never builds (dotfiles-installer-*).\n"+
				"release.yml builds %v; a reader who follows this document waits for files that do not exist.",
				doc, assets)
		}
		if strings.Contains(text, "gh release create") {
			t.Errorf("%s documents a manual `gh release create` step.\n"+
				"release.yml's release job creates the draft release itself (softprops/action-gh-release), "+
				"so the document describes a step that has nothing left to do.", doc)
		}
	}
}

// TestReleaseDocsNameEveryWorkflowJob is the guard for the other half of the
// same claim: a document that describes the process has to name the jobs the
// process runs. The names are derived from release.yml's `jobs:` block, so a job
// added to the workflow is asked for in the runbook rather than left to a
// paragraph that still describes the four jobs it used to have.
func TestReleaseDocsNameEveryWorkflowJob(t *testing.T) {
	workflow := readRepoFile(t, filepath.Join(".github", "workflows", "release.yml"))

	jobs := workflowJobs(workflow)
	if len(jobs) < 3 {
		t.Fatalf("release.yml declares %d jobs, so this guard proves nothing: %v", len(jobs), jobs)
	}
	t.Logf("release.yml runs: %v", jobs)

	runbook := readRepoFile(t, filepath.Join("docs", "RELEASING.md"))
	var missing []string
	for _, job := range jobs {
		if !strings.Contains(runbook, job) {
			missing = append(missing, job)
		}
	}
	if len(missing) > 0 {
		t.Errorf("docs/RELEASING.md does not name %d of the %d jobs release.yml runs: %v\n"+
			"A reader following the runbook cannot tell which job failed, or what the one they are "+
			"watching is supposed to prove. Name every job the workflow declares.",
			len(missing), len(jobs), missing)
	}
}

// TestReleaseProseDoesNotCiteWorkflowLineNumbers rejects the shape that made the
// retired pages stale: a citation of a line in release.yml. Every line number in
// a document is a claim that a later edit silently falsifies, and the job and
// step names are the citation that survives an edit.
func TestReleaseProseDoesNotCiteWorkflowLineNumbers(t *testing.T) {
	checked := 0
	for _, doc := range releaseProseDocs {
		text := readRepoFile(t, doc)
		checked++
		if citation := workflowLineCitationRE.FindString(text); citation != "" {
			t.Errorf("%s cites %q.\n"+
				"A line number in the workflow is a claim that any edit above it falsifies, and the "+
				"documents that described the old procedure went stale exactly that way. Cite the job or "+
				"the step by name instead.", doc, citation)
		}
	}
	if checked == 0 {
		t.Fatal("no release document was read, so this guard proves nothing")
	}
}

// TestContributingDelegatesTheReleaseMechanicsToTheRunbook guards the handover
// itself: docs/contributing.md deliberately describes the release *process* and
// hands the mechanics to one page, and the whole point of retiring the second
// document is that the handover keeps working. A contributor who follows the
// process page has to land on the runbook.
func TestContributingDelegatesTheReleaseMechanicsToTheRunbook(t *testing.T) {
	contributing := readRepoFile(t, filepath.Join("docs", "contributing.md"))
	if !strings.Contains(contributing, "RELEASING.md") {
		t.Error("docs/contributing.md no longer points at docs/RELEASING.md, so the document that " +
			"describes the release process does not lead to the page that describes the mechanics")
	}
}

// TestTheSingleRunbookKeepsWhatTheTwoDocumentsHeld guards the failure mode the
// merge introduced: a page that absorbs another one and quietly loses half of
// it. The retired checklist held what has to be true before the tag exists; the
// runbook held what happens after it. The merged page has to carry both halves,
// and it has to name the verification the workflow performs, so a document that
// got shorter by forgetting a half fails here rather than at a tag.
func TestTheSingleRunbookKeepsWhatTheTwoDocumentsHeld(t *testing.T) {
	runbook := readRepoFile(t, filepath.Join("docs", "RELEASING.md"))

	// The pre-flight half: the tick-list, with the checks the repository really
	// runs and the two commands that decide whether a tag may exist at all.
	if items := strings.Count(runbook, "- [ ]"); items < 10 {
		t.Errorf("the runbook carries %d pre-flight items; the retired checklist held eleven and the "+
			"merge is not allowed to drop them", items)
	}
	for _, want := range []string{"make preflight", "git tag", "CHANGELOG.md"} {
		if !strings.Contains(runbook, want) {
			t.Errorf("the runbook no longer carries %q, which was in the pre-flight half of the "+
				"retired checklist", want)
		}
	}

	// The after-the-tag half, including the checks this release process gained:
	// the chain verification, the manifest that makes a download verifiable, and
	// the attestation that says which workflow and commit produced the bytes.
	for _, want := range []string{"verify-published", "SHA256SUMS", "MANIFEST.txt", "attestation"} {
		if !strings.Contains(runbook, want) {
			t.Errorf("the runbook does not name %q, so a reader cannot find that half of the release "+
				"process", want)
		}
	}
}

// workflowArtifacts returns every `artifact:` name under release.yml's build
// matrix, in file order. Reading the names from the workflow is what keeps the
// expected set out of this test.
func workflowArtifacts(workflow string) []string {
	matches := workflowArtifactLineRE.FindAllStringSubmatch(workflow, -1)
	assets := make([]string, 0, len(matches))
	for _, match := range matches {
		assets = append(assets, match[1])
	}
	return assets
}

// workflowJobs returns every job name release.yml declares, in file order. The
// `jobs:` block is sliced out first so the trigger's own keys, which are
// indented the same way, are not read as jobs.
func workflowJobs(workflow string) []string {
	at := strings.Index(workflow, "\njobs:\n")
	if at < 0 {
		return nil
	}
	matches := workflowJobLineRE.FindAllStringSubmatch(workflow[at:], -1)
	jobs := make([]string, 0, len(matches))
	for _, match := range matches {
		jobs = append(jobs, match[1])
	}
	return jobs
}
