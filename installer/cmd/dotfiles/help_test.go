package main

import (
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestHelpDocumentsEveryFlag guards the class of defect where a flag is added to
// the binary and never reaches the help. It happened with --no-mouse and
// --no-sprite: both shipped, both were read by the model, and neither was
// mentioned in `dotfiles --help`, which is the only place a user on an odd
// terminal would go looking for them.
//
// The list comes from registerFlags rather than from a written-out copy, so a
// flag cannot be added without the help being asked for it.
func TestHelpDocumentsEveryFlag(t *testing.T) {
	fs := flag.NewFlagSet("help-check", flag.ContinueOnError)
	registerFlags(fs, &cliFlags{})

	var names []string
	fs.VisitAll(func(f *flag.Flag) { names = append(names, f.Name) })
	if len(names) == 0 {
		t.Fatal("no flags were registered, so this guard proves nothing")
	}

	var missing []string
	for _, name := range names {
		if !helpDocumentsFlag(name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("dotfiles --help does not mention %d of %d flags: %v\n"+
			"The help is the only place a user can find a switch, so a flag that is not in it is a flag "+
			"nobody has: add it to helpText.", len(missing), len(names), missing)
	}
}

// helpDocumentsFlag reports whether the help shows the flag. A long flag is
// looked up as --name; a one-letter shorthand is looked up as the "-x," the help
// writes for it, so that "-v" is not satisfied by the word "verbose".
func helpDocumentsFlag(name string) bool {
	if len(name) > 1 {
		return strings.Contains(helpText, "--"+name)
	}
	return strings.Contains(helpText, "-"+name+",")
}

// TestHelpDocumentsTheDisplayGates fails when the help stops explaining a switch
// a user needs when the drawing is wrong.
//
// This is a written list on purpose, and not every DOTFILES_* variable the source
// reads: most of them are internal wiring - test mode, dry run, the WSL host, the
// Alpine release pin - and demanding a place in the help for each would fill it
// with switches nobody sets by hand. The line is "a person needs this to fix what
// they are looking at", and every entry here is on that side of it. A new gate
// arrives with a flag, and TestHelpDocumentsEveryFlag covers that.
func TestHelpDocumentsTheDisplayGates(t *testing.T) {
	gates := []string{
		"--no-anim", "DOTFILES_ANIM",
		"--no-mouse", "DOTFILES_MOUSE",
		"--no-sprite", "DOTFILES_SPRITE",
		"DOTFILES_SYNC",
		"DOTFILES_VERBOSE",
	}

	for _, gate := range gates {
		if !strings.Contains(helpText, gate) {
			t.Errorf("dotfiles --help no longer mentions %s, which is one of the switches a user needs "+
				"when the terminal does not draw the installer well.", gate)
		}
	}
}

// dotfilesEnvNameRE matches the environment variables the help documents.
var dotfilesEnvNameRE = regexp.MustCompile(`DOTFILES_[A-Z0-9_]+`)

// TestBrandingDocListsEveryHelpEnvironmentVariable guards the class of defect
// where the help documents a switch and the branding guide's variable table
// omits it. The table presented four names as the inventory while the help
// documented `DOTFILES_ANIM`, `DOTFILES_MOUSE`, `DOTFILES_SPRITE` and
// `DOTFILES_SYNC`, so a reader looking up the variable the help had just named
// found nothing.
//
// The names come from the help text itself, so a variable cannot be documented
// to a user without the branding guide being asked for it. Internal variables
// the help does not name are deliberately out of scope: the guide should list
// what a user sets, not every string the source reads.
func TestBrandingDocListsEveryHelpEnvironmentVariable(t *testing.T) {
	names := dotfilesEnvNameRE.FindAllString(helpText, -1)
	if len(names) == 0 {
		t.Fatal("the help documents no DOTFILES_* variable, so this guard proves nothing")
	}

	doc, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "BRANDING.md"))
	if err != nil {
		t.Fatalf("read docs/BRANDING.md: %v", err)
	}

	seen := make(map[string]bool, len(names))
	var missing []string
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		if !docNamesEnvironmentVariable(string(doc), name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("docs/BRANDING.md does not name %d of the %d environment variables dotfiles --help documents: %v\n"+
			"A reader who follows the help to the branding guide finds no entry for the switch they were just told about.",
			len(missing), len(seen), missing)
	}
}

// installerEnvLiteralRE matches a Go string literal that is exactly one installer
// environment variable name. Matching the whole literal is what keeps a name the
// installer merely interpolates into a string, such as the `# DOTFILES_DEFAULT_SHELL`
// placeholder or the shell snippet carrying `$DOTFILES_SHELL_STARTED`, out of the
// derived set.
var installerEnvLiteralRE = regexp.MustCompile(`^DOTFILES_[A-Z0-9_]+$`)

// TestBrandingDocNamesEveryInstallerEnvironmentVariable guards the class of defect
// where the installer reads or writes a DOTFILES_* name and the branding guide's
// inventory does not carry it. The guide presented its table as the user-facing set
// while the installer also read `DOTFILES_SKIP_DEPS`, `DOTFILES_SKIP_TOOLSET`, the
// WSL layout overrides and `DOTFILES_REPO_REF`, and it mislabelled two variables
// that `docs/manual-installation.md` tells the reader to set.
//
// The names come from the installer's own source rather than from a written-out
// list, so a variable cannot be added without the guide being asked for it. The
// guide decides how to present each one - a switch to set, an override, or a hook
// to leave alone - but it has to name them all.
//
// The set is what the installer names as environment variables in its own code. A
// name it only writes into a snippet it hands to the user's shell is not derived
// here, and the guide names one of those (`DOTFILES_SHELL_STARTED`) anyway.
func TestBrandingDocNamesEveryInstallerEnvironmentVariable(t *testing.T) {
	names := installerEnvNamesFromSource(t)
	if len(names) < 10 {
		t.Fatalf("found only %d environment variable names in the installer source, so this "+
			"guard proves nothing: %v", len(names), names)
	}

	doc, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "BRANDING.md"))
	if err != nil {
		t.Fatalf("read docs/BRANDING.md: %v", err)
	}

	var missing []string
	for _, name := range names {
		if !docNamesEnvironmentVariable(string(doc), name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("docs/BRANDING.md does not name %d of the %d environment variables the installer "+
			"reads or writes: %v\n"+
			"A reader who follows the source to the branding guide finds no entry, and cannot tell whether "+
			"the variable is a switch to set or a hook to ignore.", len(missing), len(names), missing)
	}
}

// docNamesEnvironmentVariable reports whether the document carries the name as a
// whole variable. The boundaries are what stop a longer name from satisfying a
// shorter one: `DOTFILES_SKIP_TOOLSET_X` must not count as `DOTFILES_SKIP_TOOLSET`,
// or the guide could quietly drop a row and still pass this guard.
func docNamesEnvironmentVariable(doc, name string) bool {
	re := regexp.MustCompile(`(^|[^A-Z0-9_])` + regexp.QuoteMeta(name) + `([^A-Z0-9_]|$)`)
	return re.MatchString(doc)
}

// installerEnvNamesFromSource parses the installer's non-test Go sources and
// returns every distinct string literal that is exactly a DOTFILES_* variable name,
// in sorted order. It reads the packages the binary is built from - installer/cmd
// and installer/internal - so the answer tracks the code rather than a copy.
func installerEnvNamesFromSource(t *testing.T) []string {
	t.Helper()

	root := repoRoot(t)
	seen := map[string]bool{}
	for _, dir := range []string{
		filepath.Join(root, "installer", "cmd"),
		filepath.Join(root, "installer", "internal"),
	} {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if parseErr != nil {
				return parseErr
			}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				value, unquoteErr := strconv.Unquote(lit.Value)
				if unquoteErr != nil {
					return true
				}
				if installerEnvLiteralRE.MatchString(value) {
					seen[value] = true
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", dir, err)
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// repoRoot resolves the repository checkout from the package directory, so the
// tests read the very documents the repository ships.
func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "installer")); err != nil {
		t.Fatalf("repository root not found at %s: %v", root, err)
	}
	return root
}
