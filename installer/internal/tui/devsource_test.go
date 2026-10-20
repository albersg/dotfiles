package tui

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDevArchive(t *testing.T, entries map[string]string) string {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, body := range entries {
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "main.tar.gz")
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheDevSourceExtractsTheTreeUnderOneRoot(t *testing.T) {
	archive := writeDevArchive(t, map[string]string{
		"dotfiles-abc1234/installer/go.mod": "module example\n",
		"dotfiles-abc1234/README.md":        "hello\n",
	})
	dir := t.TempDir()
	root, err := extractDevSource(archive, dir)
	if err != nil {
		t.Fatalf("extractDevSource: %v", err)
	}
	if filepath.Base(root) != "dotfiles-abc1234" {
		t.Fatalf("root = %q, want the archive's own top directory", root)
	}
	body, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("the file was not extracted: %v", err)
	}
	if strings.TrimSpace(string(body)) != "hello" {
		t.Fatalf("the extracted file reads %q", body)
	}
}

func TestTheDevSourceRefusesAnEntryOutsideItsRoot(t *testing.T) {
	archive := writeDevArchive(t, map[string]string{
		"dotfiles-abc1234/installer/go.mod": "module example\n",
		"../escape.txt":                     "no\n",
	})
	if _, err := extractDevSource(archive, t.TempDir()); err == nil {
		t.Fatal("an archive carrying an entry outside its root was accepted")
	}
}

func TestTheDevBuildStampsTheCommitItBuilds(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, devModuleDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, devModuleDir, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	args := filepath.Join(t.TempDir(), "args.txt")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$DEV_BUILD_ARGS"
out=""
prev=""
for a in "$@"; do
  if [ "$prev" = "-o" ]; then out="$a"; fi
  prev="$a"
done
: > "$out"
`
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DEV_BUILD_ARGS", args)

	commit := devCommit{SHA: "abcdef1234567890abcdef1234567890abcdef12"}
	out := filepath.Join(t.TempDir(), "dotfiles")
	if err := buildDevBinary(context.Background(), root, commit, out); err != nil {
		t.Fatalf("buildDevBinary: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("the build wrote nothing at the requested output: %v", err)
	}
	recorded, err := os.ReadFile(args)
	if err != nil {
		t.Fatal(err)
	}
	text := string(recorded)
	for _, want := range []string{"-X main.Commit=" + commit.SHA, "main.Version=dev-abcdef1", devBuildPackage} {
		if !strings.Contains(text, want) {
			t.Errorf("the build command does not carry %q:\n%s", want, text)
		}
	}
}

func TestTheDevBuildRefusesWithoutGo(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, devModuleDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, devModuleDir, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	err := buildDevBinary(context.Background(), root, devCommit{SHA: "abcdef1234567890"}, filepath.Join(t.TempDir(), "out"))
	if err == nil {
		t.Fatal("a machine without Go built the dev channel anyway")
	}
	if !strings.Contains(err.Error(), "needs Go") {
		t.Errorf("error = %q, want it to say the dev channel needs Go", err)
	}
}
