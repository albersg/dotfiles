package tui

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestEveryHintObeysTheKeyGrammar is the guard under the key grammar fixed in
// odd/tasks/key-grammar.md. It reads every installerHint literal in the package
// and fails when a hint offers a key for a verb the grammar does not allow:
//
//   - Enter opens or chooses. It is not a start, an exit, a stop, an apply, a
//     see-the-fix or a way back. A text field commits with it, because a
//     field's letters are its text.
//   - Space toggles a row that is a switch, and nothing else.
//   - A command does not run on a move key: j and k never, and h and l on a
//     screen that moves horizontally.
//   - One screen does not offer the same letter for two different commands.
//
// The hints that break the grammar today are listed in keyGrammarLegacy with
// the unit of the document that fixes them, so the pass is honest about what it
// has not reached yet. The list only shrinks: a unit that fixes a screen
// removes its entry in the same commit, and an entry that stops breaking the
// grammar fails here until it is removed. A violation that is not on the list
// fails the moment it is written; nothing new may be added to it.
//
// The Vim Trainer's hints are read past: docs/vim-trainer-spec.md fixes their
// keys, and they are the lesson, so a literal assigned to a trainerHint
// variable is not judged here.
func TestEveryHintObeysTheKeyGrammar(t *testing.T) {
	sites, screens, files := readHintSites(t)
	if len(sites) < 35 {
		t.Fatalf("the walk read %d hints in %d files, too few to be reading the package: this guard would pass by looking at nothing", len(sites), files)
	}

	breaches := map[string]string{}
	for _, site := range sites {
		if reason := grammarBreach(site); reason != "" {
			breaches[hintID(site.keys, site.verb)] = reason
		}
	}

	for _, screen := range screens {
		if letters := screenLetterCollisions(screen); len(letters) > 0 {
			t.Errorf("%s: the screen offers %s for more than one command: %v", screen.pos, strings.Join(letters, ", "), screen.keys)
		}
		if letters := screenHorizontalMoveCollisions(screen); len(letters) > 0 {
			t.Errorf("%s: the screen moves horizontally and also runs a command on %s: %v", screen.pos, strings.Join(letters, ", "), screen.keys)
		}
	}

	for id, reason := range breaches {
		if _, known := keyGrammarLegacy[id]; !known {
			t.Errorf("a hint breaks the key grammar and is not in keyGrammarLegacy: %s (%s)", id, reason)
		}
	}
	for id, unit := range keyGrammarLegacy {
		if _, still := breaches[id]; !still {
			t.Errorf("keyGrammarLegacy lists %s for %s and it obeys the grammar now: remove the entry so the list keeps shrinking", id, unit)
		}
	}

	t.Logf("read %d hints in %d files; %d still break the grammar (%d legacy entries)", len(sites), files, len(breaches), len(keyGrammarLegacy))
}

// keyGrammarLegacy is the shrink-only list of hints that still break the
// grammar, keyed by the hint's identity (hintID) and valued with the unit of
// odd/tasks/key-grammar.md that fixes its screen. Each unit removes its entries
// in the commit that fixes them, and the guard fails if an entry outlives the
// violation it names.
var keyGrammarLegacy = map[string]string{
	// The install review starts the run with Enter and the welcome screen starts
	// the wizard with it. U3 fixed the review; U9 fixes the welcome, and the entry
	// goes when both have.
	`"[Enter]" "start"`: "U9",
	// The port manager's stop confirmation names Enter as the gentle stop and
	// runs the forced one on k, which is the move-down key.
	`"[t/Enter]" "stop gently"`: "U5",
	`"[k]" "force stop"`:        "U5",
	// The update centre's confirmation applies the plan with Enter.
	`"[Enter]" "apply the plan"`: "U5",
	// The screens the pass reaches last: the completion screen exits with Enter,
	// the reference and topic screens return to the menu with Enter/Space/Esc/q,
	// the installing screen toggles details with Space, and the screens that quit
	// spend Space and q on it.
	`"[Enter]" "exit"`:                       "U9",
	`"[Enter/Space/Esc/q]" "return to menu"`: "U9",
	`"[Space d]" "details"`:                  "U9",
	`"[Space q]" "quit"`:                     "U9",
}

// hintSite is one installerHint literal the guard read, with the variable it is
// assigned to when it has one and the position it was written at.
type hintSite struct {
	name string
	keys string
	verb string
	pos  token.Position
}

// hintScreen is one []installerHint{...} literal: the footer of one screen,
// which is what a letter collision is judged against.
type hintScreen struct {
	keys []string
	pos  token.Position
}

// hintID is the identity a hint is listed by: its keys and its verb, which are
// the two things the grammar judges and the two things a unit changes when it
// moves a screen onto the grammar.
func hintID(keys, verb string) string {
	return fmt.Sprintf("%q %q", keys, verb)
}

// grammarBreach names why a hint breaks the grammar, or returns "" when it
// obeys it.
func grammarBreach(site hintSite) string {
	if strings.Contains(site.keys, "Enter") && !enterVerbAllowed(site.verb) {
		return fmt.Sprintf("Enter is offered for %q, and Enter opens or chooses (a text field commits)", site.verb)
	}
	if strings.Contains(site.keys, "Space") && !spaceVerbAllowed(site.verb) {
		return fmt.Sprintf("Space is offered for %q, and Space only toggles a switch", site.verb)
	}
	if !moveVerb(site.verb) {
		for _, letter := range []string{"j", "k"} {
			if hasSingleKey(site.keys, letter) {
				return fmt.Sprintf("the command runs on %q, which is a move key", letter)
			}
		}
	}
	return ""
}

// enterVerbAllowed reports whether Enter may carry the verb. It opens a screen,
// it chooses a value, and it commits a text field; "select" is the spelling the
// pass is replacing, and the guard reads it as the choose it means.
func enterVerbAllowed(verb string) bool {
	switch verb {
	case "select", "validate", "keep filter":
		return true
	}
	return strings.HasPrefix(verb, "open") || strings.HasPrefix(verb, "choose")
}

// spaceVerbAllowed reports whether Space may carry the verb: Space toggles a
// row that is a switch, whether the row turns a setting off or on or picks an
// item up and puts it down.
func spaceVerbAllowed(verb string) bool {
	switch verb {
	case "turn it off or on", "select or drop":
		return true
	}
	return false
}

// moveVerb reports whether the hint is one of the movement hints, which are the
// only hints a move key may carry.
func moveVerb(verb string) bool {
	switch verb {
	case "up", "down", "move":
		return true
	}
	return false
}

// hasSingleKey reports whether keys offers one letter as a key of its own.
func hasSingleKey(keys, letter string) bool {
	for _, key := range hintKeyTokens(keys) {
		if key == letter {
			return true
		}
	}
	return false
}

// hintKeyTokens splits a hint's keys into the keys themselves: the brackets, the
// slashes and the arrows are separators, and what is left are the words a
// reader presses.
func hintKeyTokens(keys string) []string {
	separators := strings.NewReplacer(
		"[", " ", "]", " ", "/", " ",
		"↑", " ", "↓", " ", "←", " ", "→", " ",
	)
	return strings.Fields(separators.Replace(keys))
}

// screenLetterCollisions returns the letters one screen offers for more than one
// command. The letters are letters only: Space, Enter, Esc and the named keys
// are words, and a collision between those is the other rules' business.
func screenLetterCollisions(screen hintScreen) []string {
	counts := map[string]int{}
	for _, keys := range screen.keys {
		for _, key := range hintKeyTokens(keys) {
			if len(key) == 1 && key[0] >= 'a' && key[0] <= 'z' {
				counts[key]++
			}
		}
	}
	var collisions []string
	for key, count := range counts {
		if count > 1 {
			collisions = append(collisions, key)
		}
	}
	sort.Strings(collisions)
	return collisions
}

// screenHorizontalMoveCollisions returns the h and l commands on a screen that
// also moves horizontally, where those two letters are the move keys. A screen
// moves horizontally when one of its hints offers an arrow: the arrows are the
// pair the grammar fixes for adjusting a value, and every screen that adjusts
// one shows them.
func screenHorizontalMoveCollisions(screen hintScreen) []string {
	movesHorizontally := false
	for _, keys := range screen.keys {
		if strings.ContainsAny(keys, "←→") {
			movesHorizontally = true
			break
		}
	}
	if !movesHorizontally {
		return nil
	}
	seen := map[string]bool{}
	for _, keys := range screen.keys {
		for _, key := range hintKeyTokens(keys) {
			if key == "h" || key == "l" {
				seen[key] = true
			}
		}
	}
	var collisions []string
	for key := range seen {
		collisions = append(collisions, key)
	}
	sort.Strings(collisions)
	return collisions
}

// readHintSites reads the package's sources and returns every installerHint
// literal, plus every []installerHint{...} slice, which is one screen's footer.
func readHintSites(t *testing.T) ([]hintSite, []hintScreen, int) {
	t.Helper()

	fset := token.NewFileSet()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package's sources failed: %v", err)
	}

	type source struct {
		path string
		node *ast.File
	}
	var sources []source
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s failed: %v", path, err)
		}
		sources = append(sources, source{path: path, node: node})
	}

	// The declared hints first, so a screen that lists them by name can be read
	// whole. A literal assigned to a trainerHint variable is left out: the
	// trainer's keys are its lesson.
	declared := map[string]hintSite{}
	declaredNodes := map[*ast.CompositeLit]bool{}
	for _, src := range sources {
		ast.Inspect(src.node, func(node ast.Node) bool {
			spec, ok := node.(*ast.ValueSpec)
			if !ok {
				return true
			}
			for index, name := range spec.Names {
				if index >= len(spec.Values) {
					continue
				}
				lit, ok := spec.Values[index].(*ast.CompositeLit)
				if !ok || !isInstallerHintLit(lit) {
					continue
				}
				// The node is recorded either way, so the second pass reads a
				// trainer hint as the declared literal it is and not as an inline
				// one.
				declaredNodes[lit] = true
				if strings.HasPrefix(name.Name, "trainerHint") {
					continue
				}
				keys, verb, ok := hintLiteralText(lit)
				if !ok {
					continue
				}
				declared[name.Name] = hintSite{name: name.Name, keys: keys, verb: verb, pos: fset.Position(lit.Pos())}
			}
			return true
		})
	}

	var sites []hintSite
	for _, site := range declared {
		sites = append(sites, site)
	}

	var screens []hintScreen
	for _, src := range sources {
		ast.Inspect(src.node, func(node ast.Node) bool {
			lit, ok := node.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if isInstallerHintLit(lit) && !declaredNodes[lit] {
				keys, verb, ok := hintLiteralText(lit)
				if ok {
					sites = append(sites, hintSite{keys: keys, verb: verb, pos: fset.Position(lit.Pos())})
				}
				return true
			}
			if !isHintSliceLit(lit) {
				return true
			}
			var screen hintScreen
			trainer := false
			for _, element := range lit.Elts {
				switch element := element.(type) {
				case *ast.Ident:
					if strings.HasPrefix(element.Name, "trainerHint") {
						trainer = true
						continue
					}
					if site, ok := declared[element.Name]; ok {
						screen.keys = append(screen.keys, site.keys)
					}
				case *ast.CompositeLit:
					if keys, _, ok := hintLiteralText(element); ok {
						screen.keys = append(screen.keys, keys)
					}
				}
			}
			if !trainer && len(screen.keys) > 0 {
				screen.pos = fset.Position(lit.Pos())
				screens = append(screens, screen)
			}
			return true
		})
	}

	return sites, screens, len(sources)
}

// isInstallerHintLit reports whether a composite literal is one installerHint:
// the type is written out, or it is one element of a []installerHint slice the
// caller is already inside.
func isInstallerHintLit(lit *ast.CompositeLit) bool {
	ident, ok := lit.Type.(*ast.Ident)
	return ok && ident.Name == "installerHint"
}

// isHintSliceLit reports whether a composite literal is a []installerHint.
func isHintSliceLit(lit *ast.CompositeLit) bool {
	array, ok := lit.Type.(*ast.ArrayType)
	if !ok {
		return false
	}
	element, ok := array.Elt.(*ast.Ident)
	return ok && element.Name == "installerHint"
}

// hintLiteralText returns the two strings an installerHint literal holds, in
// the order the type declares them. A literal whose fields are not both plain
// strings is not returned: it is computed, and the guard cannot judge it.
func hintLiteralText(lit *ast.CompositeLit) (string, string, bool) {
	if len(lit.Elts) != 2 {
		return "", "", false
	}
	keys, ok := lit.Elts[0].(*ast.BasicLit)
	if !ok || keys.Kind != token.STRING {
		return "", "", false
	}
	verb, ok := lit.Elts[1].(*ast.BasicLit)
	if !ok || verb.Kind != token.STRING {
		return "", "", false
	}
	keysText, err := strconv.Unquote(keys.Value)
	if err != nil {
		return "", "", false
	}
	verbText, err := strconv.Unquote(verb.Value)
	if err != nil {
		return "", "", false
	}
	return keysText, verbText, true
}
