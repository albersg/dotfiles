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
var releaseProseDocs = []string{
	filepath.Join("docs", "RELEASING.md"),
	filepath.Join("docs", "release-checklist.md"),
	filepath.Join("docs", "contributing.md"),
}

// workflowArtifactLineRE matches a build-matrix entry's artifact name: the
// `artifact:` key indented under `matrix.include`. Matching the key rather than
// any occurrence of a `dotfiles-` string is what keeps `${{ matrix.artifact }}`
// references out of the derived set.
var workflowArtifactLineRE = regexp.MustCompile(`(?m)^[ \t]+artifact:[ \t]+(\S+)[ \t]*$`)

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
