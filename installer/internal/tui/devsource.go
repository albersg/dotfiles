package tui

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The dev channel serves the installer as it is on the remote main branch. There is
// no release per commit: the channel resolves the last commit of main, fetches that
// commit's tree and builds it here, with the same link-time identity a release
// carries. It needs the Go toolchain on the machine, and it says so when the
// toolchain is missing rather than pretending it installed something.
const (
	devRepository    = "albersg/dotfiles"
	devMainCommitURL = "https://api.github.com/repos/" + devRepository + "/commits/main"
	devSourceURLBase = "https://codeload.github.com/" + devRepository + "/tar.gz/"
	devModuleDir     = "installer"
	devBuildPackage  = "./cmd/dotfiles"
	// devSourceMaxBytes bounds the tree a build will unpack. The repository carries
	// assets and themes, so it is larger than a release binary and still far below
	// this.
	devSourceMaxBytes = 64 << 20
	// devBuildTimeout bounds the compile: a cold module cache on a slow machine is
	// minutes, and a build that never ends is a build this installer must stop.
	devBuildTimeout = 10 * time.Minute
)

// devCommit is what the dev channel resolved: the commit main points at, and when
// it was made.
type devCommit struct {
	SHA  string
	Date time.Time
}

// short is the seven characters a person reads and a tag can carry.
func (c devCommit) short() string {
	if len(c.SHA) > 7 {
		return c.SHA[:7]
	}
	return c.SHA
}

// label is how the dev answer is named in a record and on a screen: the branch it
// came from and the commit it resolved.
func (c devCommit) label() string { return "main " + c.short() }

// fetchDevMainCommit reads the commit main points at, from the repository's own
// API. A missing answer is an error rather than an empty commit: a build of nothing
// is not a build of main.
func fetchDevMainCommit(client *http.Client) (devCommit, error) {
	request, err := http.NewRequest(http.MethodGet, devMainCommitURL, nil)
	if err != nil {
		return devCommit{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := client.Do(request)
	if err != nil {
		return devCommit{}, fmt.Errorf("could not read the last commit on main: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return devCommit{}, fmt.Errorf("could not read the last commit on main: %s", response.Status)
	}
	var answer struct {
		SHA    string `json:"sha"`
		Commit struct {
			Committer struct {
				Date time.Time `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&answer); err != nil {
		return devCommit{}, fmt.Errorf("could not read the last commit on main: %w", err)
	}
	if len(answer.SHA) < 7 {
		return devCommit{}, errors.New("the last commit on main has no id this installer can build")
	}
	return devCommit{SHA: answer.SHA, Date: answer.Commit.Committer.Date}, nil
}

// downloadDevSource fetches the commit's tree into dest. It is the same bounded
// download the release assets use, with a bound that fits a source archive.
func downloadDevSource(client *http.Client, sha, dest string) error {
	url := devSourceURLBase + sha
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("could not download %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("could not download %s: %s", url, response.Status)
	}
	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("could not create %s: %w", dest, err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, devSourceMaxBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("could not download %s: %w", url, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("could not write %s: %w", dest, closeErr)
	}
	if written > devSourceMaxBytes {
		return fmt.Errorf("could not download %s: the tree is larger than %d bytes", url, devSourceMaxBytes)
	}
	return nil
}

// extractDevSource unpacks the archive into dir and returns the root the archive
// carries. Every entry must live under that root, so a crafted archive cannot write
// outside the staging directory.
func extractDevSource(archive, dir string) (string, error) {
	file, err := os.Open(archive)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("the downloaded tree is not a gzip archive: %w", err)
	}
	defer gzipReader.Close()

	root := ""
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("the downloaded tree cannot be read: %w", err)
		}
		// GitHub's archives open with a PAX global header before the tree: Typeflag
		// 'g', named "pax_global_header", carrying no content. It is bookkeeping for
		// the reader, not a directory in the archive, and taking its name as the root
		// made the real tree look like a second one.
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader {
			continue
		}
		name := filepath.Clean(header.Name)
		if name == "." {
			continue
		}
		if filepath.IsAbs(name) || strings.HasPrefix(name, "..") {
			return "", fmt.Errorf("the downloaded tree carries an entry outside it: %q", header.Name)
		}
		parts := strings.SplitN(name, string(os.PathSeparator), 2)
		if root == "" {
			root = parts[0]
		} else if parts[0] != root {
			return "", fmt.Errorf("the downloaded tree carries two roots: %q and %q", root, parts[0])
		}
		target := filepath.Join(dir, name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			written, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(written, reader); err != nil {
				written.Close()
				return "", err
			}
			if err := written.Close(); err != nil {
				return "", err
			}
		default:
			// Links and devices in a source archive are not things this installer
			// builds from, and skipping them cannot break a Go module.
			continue
		}
	}
	if root == "" {
		return "", errors.New("the downloaded tree is empty")
	}
	return filepath.Join(dir, root), nil
}

// buildDevBinary builds the installer from the tree with the commit stamped into
// it. The command is the one a developer runs; the identity flags are the ones the
// release workflow passes, so a dev binary names its commit the same way.
func buildDevBinary(ctx context.Context, root string, commit devCommit, out string) error {
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("the dev channel builds the last commit of main, which needs Go on PATH; " +
			"install Go or switch to the stable channel")
	}
	module := filepath.Join(root, devModuleDir)
	if info, err := os.Stat(filepath.Join(module, "go.mod")); err != nil || info.IsDir() {
		return fmt.Errorf("the downloaded tree has no Go module at %s", module)
	}
	ctx, cancel := context.WithTimeout(ctx, devBuildTimeout)
	defer cancel()
	flags := fmt.Sprintf("-s -w -X main.Version=dev-%s -X main.Commit=%s -X main.Date=%s",
		commit.short(), commit.SHA, time.Now().UTC().Format(time.RFC3339))
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", out, "-ldflags", flags, devBuildPackage)
	command.Dir = module
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := command.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("building main %s took longer than %s", commit.short(), devBuildTimeout)
		}
		return fmt.Errorf("could not build main %s: %w\n%s", commit.short(), err, strings.TrimSpace(string(output)))
	}
	return nil
}

// applyDevSelfUpdate replaces target with a build of the last commit on main and
// returns the copy of the binary it kept, the same contract the release path has.
func applyDevSelfUpdate(client *http.Client, target updateTarget) (string, devCommit, error) {
	if owner := packageManagerOwner(target.Path); owner != "" {
		return "", devCommit{}, packageManagerRefusal(owner, target.Path)
	}
	commit, err := fetchDevMainCommit(client)
	if err != nil {
		return "", devCommit{}, err
	}

	dir := filepath.Dir(target.Path)
	staging, err := os.MkdirTemp(dir, updateStagingPrefix)
	if err != nil {
		return "", devCommit{}, fmt.Errorf("could not create a staging directory beside %s: %w", target.Path, err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	archive := filepath.Join(staging, "main.tar.gz")
	if err := downloadDevSource(client, commit.SHA, archive); err != nil {
		return "", devCommit{}, err
	}
	root, err := extractDevSource(archive, staging)
	if err != nil {
		return "", devCommit{}, err
	}
	built := filepath.Join(staging, "dotfiles")
	if err := buildDevBinary(context.Background(), root, commit, built); err != nil {
		return "", devCommit{}, err
	}
	if err := verifyDevBinary(built, commit); err != nil {
		return "", devCommit{}, err
	}

	kept := target.Path + updatePreviousSuffix
	if err := copyUpdateFile(target.Path, kept); err != nil {
		return "", devCommit{}, fmt.Errorf("could not keep a copy of the binary being replaced: %w", err)
	}
	if err := os.Rename(built, target.Path); err != nil {
		return "", devCommit{}, fmt.Errorf("could not move the built binary into place: %w", err)
	}
	return kept, commit, nil
}

// verifyDevBinary reads the commit back out of the binary that was just built. A
// build that does not name the commit it came from is not the build this channel
// promised, and it is not installed.
func verifyDevBinary(path string, commit devCommit) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("the binary built from main %s does not run: %w", commit.short(), err)
	}
	if !strings.Contains(string(output), commit.short()) {
		return fmt.Errorf("the binary built from main %s does not name its commit: %q",
			commit.short(), strings.TrimSpace(string(output)))
	}
	return nil
}
