package tui

import (
	"bytes"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The dev channel reads GitHub's releases list, which is ordered by creation date
// rather than by version. Taking the first pre-release in that list therefore serves
// whichever pre-release was published last, and a pre-release cut after a higher one
// -- a hotfix branch, or a re-cut of an older line -- is served over the newer
// version. A channel called dev should serve the highest version it can see.
func TestTheDevChannelServesTheHighestVersionNotTheNewestByDate(t *testing.T) {
	// rc1 is published AFTER rc2, and rc2 is the higher version. GitHub would put
	// rc1 first; the answer must still be rc2.
	list := `[
	  {"tag_name":"v0.8.0-rc1","prerelease":true,"draft":false,"published_at":"2026-10-20T10:00:00Z"},
	  {"tag_name":"v0.8.0-rc2","prerelease":true,"draft":false,"published_at":"2026-10-15T10:00:00Z"}
	]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(list))
	}))
	defer server.Close()

	answer, err := latestPrerelease(server.Client(), server.URL)
	if err != nil {
		t.Fatalf("reading the list failed: %v", err)
	}
	if answer.Tag != "v0.8.0-rc2" {
		t.Errorf("the dev channel served %q, and v0.8.0-rc2 is the higher version:\nthe list was ordered by date, and the channel took the first entry", answer.Tag)
	}
	// The date, not the instant: the served answer carries the publication time too.
	if got := answer.PublishedAt.Format(time.DateOnly); got != "2026-10-15" {
		t.Errorf("the served pre-release reports publication date %s, want 2026-10-15", got)
	}
}

// A draft is not published, and a final release is not a pre-release, so neither may
// be served by dev even when it carries the highest version in the list.
func TestTheDevChannelSkipsDraftsAndFinalsWhenChoosingTheHighest(t *testing.T) {
	list := `[
	  {"tag_name":"v0.9.0","prerelease":false,"draft":false,"published_at":"2026-10-20T10:00:00Z"},
	  {"tag_name":"v0.8.1-rc9","prerelease":true,"draft":true,"published_at":"2026-10-19T10:00:00Z"},
	  {"tag_name":"v0.8.0-rc3","prerelease":true,"draft":false,"published_at":"2026-10-18T10:00:00Z"},
	  {"tag_name":"v0.7.0-rc1","prerelease":true,"draft":false,"published_at":"2026-10-17T10:00:00Z"}
	]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(list))
	}))
	defer server.Close()

	answer, err := latestPrerelease(server.Client(), server.URL)
	if err != nil {
		t.Fatalf("reading the list failed: %v", err)
	}
	if answer.Tag != "v0.8.0-rc3" {
		t.Errorf("the dev channel served %q, and v0.8.0-rc3 is the highest published pre-release", answer.Tag)
	}
}

// A tag that is not a version cannot be ordered, so it is never served; and a list
// with no servable pre-release is an error rather than an invented answer.
func TestTheDevChannelRefusesWhenNothingIsServable(t *testing.T) {
	list := `[
	  {"tag_name":"nightly","prerelease":true,"draft":false,"published_at":"2026-10-20T10:00:00Z"},
	  {"tag_name":"","prerelease":true,"draft":false,"published_at":"2026-10-20T10:00:00Z"}
	]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(list))
	}))
	defer server.Close()

	if answer, err := latestPrerelease(server.Client(), server.URL); err == nil {
		t.Errorf("a list of tags that are not versions returned %q, and it must report that nothing is servable", answer.Tag)
	}
}

// The dev channel's choice is described in five places -- the panel sentence a person
// reads, three comments, and a test's message -- and the sentence the person reads is
// the one nothing asserted. Claiming the channel follows the publishing order when it
// compares versions would be a lie a reader has no way to check. This guard keeps every
// source file honest about it: the wording that described the old behaviour may not come
// back, wherever it would have been written.
//
// It scans source, not prose files: the changelog is history and keeps its old wording,
// because the limit it recorded was real at the time.
func TestNoSourceStillClaimsTheDevChannelFollowsThePublishOrder(t *testing.T) {
	stale := []string{
		"newest pre-release",
		"newest published pre-release",
		"by publish order",
		"takes the publish order",
	}

	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolving the package directory failed: %v", err)
	}
	scanned := 0
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scanned++
		for _, phrase := range stale {
			if bytes.Contains(body, []byte(phrase)) {
				t.Errorf("%s still claims the dev channel uses %q, and it compares versions instead", filepath.Base(path), phrase)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the source failed: %v", err)
	}
	if scanned == 0 {
		t.Fatal("scanned no Go files, so this guard is looking at nothing")
	}
}
