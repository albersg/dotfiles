package tui

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Stage 3 of fish-parity.md is the four commands zsh has and fish had not. Their
// bodies cannot be compared as text -- the two languages spell a branch, a loop and a
// message to the error stream differently -- so what is compared here is the policy
// they carry: which format needs which tool, with which words, which tool is asked
// about before it is run, and what a person is told when something goes wrong. A
// guard comparing the text would be a guard against translating at all.
//
// The two configuration paths are declared in shell_listing_parity_test.go, which
// compares the listings' aliases the same way.

// parityFunctions are the four commands stage 3 covers.
var parityFunctions = []string{"cat", "extract", "compress", "serve"}

// readShellFunction returns the body of one function a configuration declares. Only
// the name asked for is read, so nothing in another function can swallow it: depth is
// counted from that function's own declaration. zsh's closing brace is not always in
// the first column -- source_first_existing's is indented -- so a column rule would
// lose every function after it, which is how this guard first read `cat` as absent.
func readShellFunction(t *testing.T, path, shell, name string) []string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	declaration := regexp.MustCompile(`^(?:function\s+)?` + regexp.QuoteMeta(name) + `\(\)\s*\{$`)
	if shell == "fish" {
		declaration = regexp.MustCompile(`^function\s+` + regexp.QuoteMeta(name) + `$`)
	}
	opener := regexp.MustCompile(`^(?:if|for|while|switch|begin)\b`)

	lines := strings.Split(string(body), "\n")
	start := -1
	for i, line := range lines {
		if declaration.MatchString(strings.TrimSpace(line)) {
			start = i
			break
		}
	}
	if start < 0 {
		return nil
	}

	found := []string{}
	depth := 1
	for _, line := range lines[start+1:] {
		trimmed := strings.TrimSpace(line)
		if shell == "zsh" {
			switch trimmed {
			case "}":
				depth--
			case "{":
				depth++
			}
		} else {
			if opener.MatchString(trimmed) {
				depth++
			}
			if trimmed == "end" {
				depth--
			}
		}
		if depth <= 0 {
			break
		}
		found = append(found, line)
	}
	return found
}

// parityBranch is one format's policy: the tool it needs, and the words that tool is
// given. The words are the ones both shells have to agree on -- a flag, or a
// subcommand such as 7z's `a` -- and never a file name, which the two spell
// differently.
type parityBranch struct {
	tool  string
	words []string
}

func (b parityBranch) String() string { return b.tool + " " + strings.Join(b.words, " ") }

// policyWords keeps the words of a command that both shells have to agree on. A word
// carrying a `$` is a value whose spelling differs between them, and a redirection
// says where the output goes rather than what runs.
func policyWords(words []string) []string {
	kept := []string{}
	for _, word := range words {
		if strings.ContainsAny(word, "$") || strings.HasPrefix(word, ">") || word == ";;" {
			continue
		}
		kept = append(kept, strings.Trim(word, `"'`))
	}
	return kept
}

var (
	zshCaseLine  = regexp.MustCompile(`^([^)]+)\)\s*(.*)$`)
	zshNeedIn    = regexp.MustCompile(`need=([A-Za-z0-9_.-]+)`)
	needAssignAt = "need="
	setDashDash  = "set -- "
)

// zshBranches reads the format table out of an extract or compress body. Every one
// of zsh's branch lines carries its own patterns and what that format runs, so the
// line is split at the closing parenthesis of its pattern list.
func zshBranches(body []string) map[string]parityBranch {
	branches := map[string]parityBranch{}
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		match := zshCaseLine.FindStringSubmatch(trimmed)
		if match == nil || !strings.Contains(match[1], "*") {
			continue
		}
		rest := strings.TrimSpace(match[2])
		tool := ""
		if m := zshNeedIn.FindStringSubmatch(rest); m != nil {
			tool = m[1]
		}
		if at := strings.Index(rest, setDashDash); at >= 0 {
			rest = rest[at+len(setDashDash):]
		} else if at := strings.Index(rest, needAssignAt); at == 0 {
			// `need=gzip ;;` names the tool with no command beside it: the command
			// for that format is the redirection branch further down.
			rest = ""
		}
		words := policyWords(strings.Fields(rest))
		if tool == "" && len(words) > 0 {
			tool = words[0]
		}
		words = dropWord(words, tool)
		for _, pattern := range strings.Split(match[1], "|") {
			pattern = strings.TrimSpace(pattern)
			if pattern == "*" {
				// The catch-all is not a format: it is what happens to one that is not in
				// the table, and both shells have one.
				continue
			}
			branches[pattern] = parityBranch{tool: tool, words: words}
		}
	}
	return branches
}

var (
	fishCaseLine = "case "
	fishToolSet  = regexp.MustCompile(`^set\s+(?:-l\s+)?tool\s+(.*)$`)
	fishCmdSet   = regexp.MustCompile(`^set\s+(?:-l\s+)?cmd\s+(.*)$`)
)

// fishBranches reads the same table out of a fish body: a `case` line carries the
// patterns, the line under it names the tool, and the line under that builds the
// command, which is run once after the table the way zsh runs it.
func fishBranches(body []string) map[string]parityBranch {
	branches := map[string]parityBranch{}
	patterns := []string{}
	tool := ""
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "#"):
			continue
		case strings.HasPrefix(trimmed, fishCaseLine):
			patterns = splitFishPatterns(strings.TrimPrefix(trimmed, fishCaseLine))
			tool = ""
		case fishToolSet.MatchString(trimmed):
			tool = strings.Trim(strings.TrimSpace(fishToolSet.FindStringSubmatch(trimmed)[1]), `"'`)
		case fishCmdSet.MatchString(trimmed):
			words := dropWord(policyWords(strings.Fields(fishCmdSet.FindStringSubmatch(trimmed)[1])), tool)
			for _, pattern := range patterns {
				if pattern == "*" {
					// The catch-all is not a format: it is what happens to one that is not
					// in the table, and both shells have one.
					continue
				}
				branches[pattern] = parityBranch{tool: tool, words: words}
			}
		}
	}
	return branches
}

// splitFishPatterns reads the patterns out of a `case` line: fish takes several glob
// patterns as alternatives, quoted or not.
func splitFishPatterns(rest string) []string {
	patterns := []string{}
	for _, field := range strings.Fields(rest) {
		patterns = append(patterns, strings.Trim(field, `"'`))
	}
	return patterns
}

func dropWord(words []string, drop string) []string {
	kept := []string{}
	for _, word := range words {
		if word == drop {
			continue
		}
		kept = append(kept, word)
	}
	return kept
}

// compareBranches reports every pattern where the two shells disagree about the tool
// or about the words it is given.
func compareBranches(t *testing.T, what string, zsh, fish map[string]parityBranch) {
	t.Helper()
	if len(zsh) < 4 {
		t.Fatalf("only %d %s patterns were read out of %s, so this guard would be comparing almost nothing",
			len(zsh), what, zshShippedConfig)
	}
	for _, pattern := range sortedKeys(zsh) {
		fromFish, ok := fish[pattern]
		if !ok {
			t.Errorf("%s handles %s in %s and not in %s, so that format is one shell's alone",
				what, pattern, zshShippedConfig, fishShippedConfig)
			continue
		}
		if fromFish.tool != zsh[pattern].tool || strings.Join(fromFish.words, " ") != strings.Join(zsh[pattern].words, " ") {
			t.Errorf("%s runs %q for %s under zsh and %q under fish, so the same format is handled by two different commands",
				what, zsh[pattern].String(), pattern, fromFish.String())
		}
	}
	for _, pattern := range sortedKeys(fish) {
		if _, ok := zsh[pattern]; !ok {
			t.Errorf("%s handles %s in %s and not in %s, so that format is fish's alone",
				what, pattern, fishShippedConfig, zshShippedConfig)
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// toolVariable reads the name of the variable a branch table assigns the tool to:
// `need=` for zsh, `set -l tool` for fish. Referencing that name is how the guard
// below stays true if both shells rename it.
func toolVariable(body []string, shell string) string {
	for _, line := range body {
		if shell == "zsh" {
			if m := zshNeedIn.FindStringSubmatch(line); m != nil {
				return m[1]
			}
			continue
		}
		if m := fishToolSet.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			return "tool"
		}
	}
	return ""
}

// errorMessages returns every string a body writes to the error stream, with the
// variable names normalised: which variable holds the value is the shell's business,
// and it is the value a person sees.
var shellVariable = regexp.MustCompile(`\$\{?[A-Za-z0-9_]+\}?`)

func errorMessages(body []string, shell string) []string {
	quoted := regexp.MustCompile(`'([^']*)'|"([^"]*)"`)
	messages := []string{}
	for _, line := range body {
		// Only what the error-writing verb is given: zsh writes it after `print -u2`
		// and fish before `>&2`, and the rest of the line can hold strings that are
		// not messages at all. One of them was: zsh's `esac || { rm -f "$archive";
		// print -u2 "…"; return 1 }` put the target of the rm in the comparison.
		marker := "print -u2"
		if shell == "fish" {
			marker = ">&2"
		}
		at := strings.Index(line, marker)
		if at < 0 {
			continue
		}
		if shell == "fish" {
			line = line[:at]
		} else {
			line = line[at+len(marker):]
		}
		for _, match := range quoted.FindAllStringSubmatch(line, -1) {
			message := match[1]
			if message == "" {
				message = match[2]
			}
			messages = append(messages, shellVariable.ReplaceAllString(message, "$…"))
		}
	}
	sort.Strings(messages)
	return messages
}

// commandLine returns the words of the first line in a body that runs the named
// command, so a policy expressed by one command can be compared between the shells.
func commandLine(body []string, command string) []string {
	for _, line := range body {
		words := strings.Fields(strings.TrimSpace(line))
		if len(words) > 0 && words[0] == command {
			return policyWords(words)
		}
	}
	return nil
}

// TestFishDeclaresTheCommandsZshDeclares is stage 3's first rule: fish is not missing
// one. Each name is declared in both configurations, and fish declares it as a
// function rather than an alias, which is what keeps a script from inheriting it.
func TestFishDeclaresTheCommandsZshDeclares(t *testing.T) {
	for _, name := range parityFunctions {
		if len(readShellFunction(t, zshShippedConfig, "zsh", name)) == 0 {
			t.Errorf("%s declares no %s, so this guard is comparing against nothing", zshShippedConfig, name)
		}
		if len(readShellFunction(t, fishShippedConfig, "fish", name)) == 0 {
			t.Errorf("%s declares no %s, so fish is missing a command zsh has: %s",
				fishShippedConfig, name, strings.Join(parityFunctions, ", "))
		}
	}
}

// TestBothShellsExtractTheSameFormatsWithTheSameTools: the format table is the same
// policy in both shells, so an archive one shell can unpack the other can unpack, with
// the same tool and the same flags.
func TestBothShellsExtractTheSameFormatsWithTheSameTools(t *testing.T) {
	zsh := readShellFunction(t, zshShippedConfig, "zsh", "extract")
	fish := readShellFunction(t, fishShippedConfig, "fish", "extract")
	if len(fish) == 0 {
		t.Fatalf("%s declares no extract, so every archive zsh unpacks has to be unpacked by hand under fish", fishShippedConfig)
	}
	compareBranches(t, "extract", zshBranches(zsh), fishBranches(fish))
}

// TestBothShellsCompressToTheSameFormatsWithTheSameTools is the same rule for the
// other direction: the archive formats fish can write are the ones zsh can write.
func TestBothShellsCompressToTheSameFormatsWithTheSameTools(t *testing.T) {
	zsh := readShellFunction(t, zshShippedConfig, "zsh", "compress")
	fish := readShellFunction(t, fishShippedConfig, "fish", "compress")
	if len(fish) == 0 {
		t.Fatalf("%s declares no compress, so none of the archives zsh writes can be written under fish", fishShippedConfig)
	}
	compareBranches(t, "compress", zshBranches(zsh), fishBranches(fish))
}

// TestExtractAndCompressAskWhetherTheToolResolves: a tool the host has not got is
// reported by name, which only works if the tool the table chose is the one that is
// asked about before it runs.
func TestExtractAndCompressAskWhetherTheToolResolves(t *testing.T) {
	for _, name := range []string{"extract", "compress"} {
		for _, shell := range []struct{ shell, path, guard string }{
			{"zsh", zshShippedConfig, "command -v"},
			{"fish", fishShippedConfig, "command -q"},
		} {
			body := readShellFunction(t, shell.path, shell.shell, name)
			if len(body) == 0 {
				continue
			}
			variable := toolVariable(body, shell.shell)
			if variable == "" {
				t.Errorf("%s' %s names no tool, so this guard cannot tell whether it is asked about", shell.path, name)
				continue
			}
			joined := strings.Join(body, "\n")
			if !strings.Contains(joined, shell.guard) {
				t.Errorf("%s' %s runs a tool without asking %s whether it resolves, so a host without it gets a bare command-not-found instead of being told which tool is missing",
					shell.path, name, shell.guard)
				continue
			}
			if !strings.Contains(joined, variable) {
				t.Errorf("%s' %s asks about a tool other than the one its table chose (%s), so the message names the wrong tool",
					shell.path, name, variable)
			}
		}
	}
}

// TestTheErrorMessagesAreTheSameInBothShells: which shell is running does not change
// what a mistake says. The variable names are normalised, because which variable
// holds the value is the shell's business and the value is what a person reads.
func TestTheErrorMessagesAreTheSameInBothShells(t *testing.T) {
	for _, name := range []string{"extract", "compress", "serve"} {
		zsh := errorMessages(readShellFunction(t, zshShippedConfig, "zsh", name), "zsh")
		fishBody := readShellFunction(t, fishShippedConfig, "fish", name)
		if len(fishBody) == 0 {
			t.Errorf("%s declares no %s, so it has no message of its own for this guard to compare", fishShippedConfig, name)
			continue
		}
		fish := errorMessages(fishBody, "fish")
		if strings.Join(zsh, "\n") != strings.Join(fish, "\n") {
			t.Errorf("%s tells the user %v and %s tells them %v, so the same mistake reads differently depending on the shell",
				zshShippedConfig, zsh, fishShippedConfig, fish)
		}
	}
}

// TestCatKeepsItsPolicyInBothShells: `cat` is a function in both, an argument that
// begins with a dash reaches the real cat, so the flags keep working, and a plain
// invocation reaches bat with the same words.
func TestCatKeepsItsPolicyInBothShells(t *testing.T) {
	zshBody := readShellFunction(t, zshShippedConfig, "zsh", "cat")
	fishBody := readShellFunction(t, fishShippedConfig, "fish", "cat")
	if len(fishBody) == 0 {
		t.Fatalf("%s declares no cat, so a plain `cat` prints what it always did while zsh's prints bat's plain output", fishShippedConfig)
	}
	zsh := commandLine(zshBody, "bat")
	fish := commandLine(fishBody, "bat")
	if strings.Join(zsh, " ") != strings.Join(fish, " ") {
		t.Errorf("cat runs %v under zsh and %v under fish, so the two shells show a file differently", zsh, fish)
	}
	// Flags have to reach the real cat: that is what the function is for.
	for _, body := range []struct {
		path  string
		lines []string
	}{{zshShippedConfig, zshBody}, {fishShippedConfig, fishBody}} {
		if !strings.Contains(strings.Join(body.lines, "\n"), "command cat") {
			t.Errorf("%s' cat never reaches the real cat, so `cat -n file` stops working: the flags have nowhere to go",
				body.path)
		}
	}
}

// TestServeKeepsItsPolicyInBothShells: the same server, on the same interface, with
// the port validated the same way.
func TestServeKeepsItsPolicyInBothShells(t *testing.T) {
	zshBody := readShellFunction(t, zshShippedConfig, "zsh", "serve")
	fishBody := readShellFunction(t, fishShippedConfig, "fish", "serve")
	if len(fishBody) == 0 {
		t.Fatalf("%s declares no serve, so a directory zsh can serve over HTTP cannot be served under fish", fishShippedConfig)
	}
	zsh := commandLine(zshBody, "python3")
	fish := commandLine(fishBody, "python3")
	if strings.Join(zsh, " ") != strings.Join(fish, " ") {
		t.Errorf("serve runs %v under zsh and %v under fish, so the two shells serve a different thing", zsh, fish)
	}
	// The port is refused rather than passed on, in both.
	for _, served := range []struct {
		path string
		body []string
	}{{zshShippedConfig, zshBody}, {fishShippedConfig, fishBody}} {
		if !strings.Contains(strings.Join(served.body, "\n"), "65535") {
			t.Errorf("%s' serve does not bound the port, so a nonsense port reaches the server instead of being refused", served.path)
		}
	}
}
