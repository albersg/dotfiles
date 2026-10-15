package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// WINDOWS TERMINAL: THE RENDERER ON THE OTHER SIDE OF THE WSL BOUNDARY
// ============================================================================
//
// Under WSL the installer writes the Nerd Font into ~/.local/share/fonts and the
// palette into the Linux terminals' configurations, but the program that draws
// the session is Windows Terminal, on the Windows side, which reads neither. The
// install then reports success and the user reads a success that is not one.
//
// This file is the WSL step's answer to that. It says, unconditionally, where
// the font and the palette landed and that Windows Terminal is the renderer, and
// it offers to write the two settings Windows Terminal needs -- the font face
// and the colour scheme -- into the settings.json reachable from the mount. The
// write only happens after an explicit choice, it backs the file up first, and
// it refuses rather than rewrite a file it cannot edit safely.

const (
	// windowsTerminalFontFace is the name the installer registers the archive's
	// Meslo faces under, and the fallback face it writes to Windows Terminal when
	// the machine's own build does not resolve. It derives from mesloArchiveFamily,
	// the one family the pinned archive and the macOS cask declare and the four
	// shipped terminal configurations name, so the Windows registration cannot
	// name a family the faces do not. A face named there but not resolvable is the
	// silent fallback this defect is about.
	windowsTerminalFontFace = mesloArchiveFamily

	// windowsTerminalFontFacePreferred is the Meslo build the machine already
	// carries under a name of its own. When the Windows font store resolves it,
	// it is the face the writer names, because it is the family the person chose
	// for the machine. The installer never redistributes it; it only names it.
	windowsTerminalFontFacePreferred = "MesloLGS NF Herdr"

	// windowsTerminalSchemeName is the name the dotfiles palette is registered
	// under in Windows Terminal's "schemes" array and selected by
	// profiles.defaults.colorScheme.
	windowsTerminalSchemeName = "dotfiles"
)

// windowsTerminalSetting is one thing this installer owns in Windows Terminal's
// settings.json. The list below is the single source of truth the notice and the
// offer both read, so the screen cannot name a setting the writer does not set
// and the labels can only change in one place. The pattern is wslconfig.go's
// managed-entries list: the set is declared once and every route reads it.
type windowsTerminalSetting struct {
	// ID is a stable name, so a test can address one setting without depending
	// on the order the list holds them in.
	ID string
	// Label is the phrase the notice and the offer show a person.
	Label string
}

// windowsTerminalSettings is the single list of keys this installer writes.
var windowsTerminalSettings = []windowsTerminalSetting{
	{ID: "font-face", Label: "the font face " + windowsTerminalFontFace},
	{ID: "colorScheme", Label: "the colour scheme " + windowsTerminalSchemeName},
}

// windowsTerminalRendererLines is the truth the WSL step reports before it
// reports anything as done: where the font and the palette actually landed, that
// Windows Terminal draws the screen, and the two settings it has to be told. It
// is built from windowsTerminalSettings, so a new setting reaches the screen and
// the writer together.
func windowsTerminalRendererLines() []string {
	labels := make([]string, 0, len(windowsTerminalSettings))
	for _, setting := range windowsTerminalSettings {
		labels = append(labels, setting.Label)
	}
	return []string{
		"The Nerd Font is installed under ~/.local/share/fonts and the dotfiles palette is written into the Linux terminal configurations, but this screen is drawn by Windows Terminal on the Windows side, which reads neither of them.",
		"To make the prompt render within Windows Terminal, set " + strings.Join(labels, " and ") + " in its settings.json.",
		"Windows Terminal reads its font list once, when it starts, so reopen it after this install to see the icons and the palette.",
	}
}

// windowsTerminalPackages are the package directory prefixes Windows Terminal
// installs under. The Store build and the Preview build use different names, so
// both are searched rather than assuming one.
var windowsTerminalPackages = []string{
	"Microsoft.WindowsTerminal_",
	"Microsoft.WindowsTerminalPreview_",
}

// windowsTerminalSettingsCandidates returns every settings.json path that could
// exist under a Windows profile directory, for both package names. The profile
// directory is a parameter, not a lookup of its own, so a test injects a
// temporary mount and the code never reads a real /mnt/c.
func windowsTerminalSettingsCandidates(profileDir string) []string {
	var candidates []string
	for _, pkg := range windowsTerminalPackages {
		pattern := filepath.Join(profileDir, "AppData", "Local", "Packages", pkg+"*", "LocalState", "settings.json")
		if matches, err := filepath.Glob(pattern); err == nil {
			candidates = append(candidates, matches...)
		}
	}
	return candidates
}

// windowsTerminalSettingsPath resolves the one settings.json to edit. No
// candidate is a named refusal rather than an empty file, and two candidates are
// refused with both named because the running package cannot be told apart: an
// arbitrary one is never chosen.
func windowsTerminalSettingsPath(profileDir string) (string, error) {
	var found []string
	for _, candidate := range windowsTerminalSettingsCandidates(profileDir) {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			found = append(found, candidate)
		}
	}

	switch len(found) {
	case 0:
		return "", fmt.Errorf("no Windows Terminal settings.json found under %s",
			filepath.Join(profileDir, "AppData", "Local", "Packages"))
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("more than one Windows Terminal settings.json found (%s); the running package cannot be determined, so none was changed",
			strings.Join(found, ", "))
	}
}

// ============================================================================
// THE COLOUR SCHEME: THE PALETTE IN WINDOWS TERMINAL'S OWN KEYS
// ============================================================================

// windowsTerminalSchemeColor is one colour of a scheme, in Windows Terminal's
// own key name.
type windowsTerminalSchemeColor struct {
	Key   string
	Value string
}

// windowsTerminalScheme is the scheme this installer defines, built from a theme
// definition. It is a value, not a file, so the mapping can be checked without
// writing one.
type windowsTerminalScheme struct {
	Name   string
	Colors []windowsTerminalSchemeColor
}

// windowsTerminalSchemeFields maps each canonical palette role to Windows
// Terminal's scheme key. The two text-side roles a theme also carries
// (cursor_text and selection_text) have no scheme key Windows Terminal reads, so
// they are deliberately absent: writing one under an invented name would be a
// key Windows Terminal ignores.
var windowsTerminalSchemeFields = []struct{ role, key string }{
	{"base", "background"},
	{"text", "foreground"},
	{"cursor", "cursorColor"},
	{"selection", "selectionBackground"},
	{"black", "black"},
	{"red", "red"},
	{"green", "green"},
	{"yellow", "yellow"},
	{"blue", "blue"},
	{"magenta", "purple"},
	{"cyan", "cyan"},
	{"white", "white"},
	{"bright_black", "brightBlack"},
	{"bright_red", "brightRed"},
	{"bright_green", "brightGreen"},
	{"bright_yellow", "brightYellow"},
	{"bright_blue", "brightBlue"},
	{"bright_magenta", "brightPurple"},
	{"bright_cyan", "brightCyan"},
	{"bright_white", "brightWhite"},
}

// windowsTerminalSchemeFor builds the scheme from a theme definition. A role the
// theme does not define is a refusal, never an invented colour: a scheme with a
// missing key would render half the prompt in another theme's colour.
func windowsTerminalSchemeFor(def themeDefinition) (windowsTerminalScheme, error) {
	scheme := windowsTerminalScheme{Name: windowsTerminalSchemeName}
	for _, field := range windowsTerminalSchemeFields {
		value := def.Palette[field.role]
		if value == "" {
			return windowsTerminalScheme{}, fmt.Errorf("the %q theme defines no %q colour, so Windows Terminal's scheme would be incomplete", def.ID, field.role)
		}
		scheme.Colors = append(scheme.Colors, windowsTerminalSchemeColor{Key: field.key, Value: value})
	}
	return scheme, nil
}

// windowsTerminalSchemeJSON is the object Windows Terminal reads from its
// "schemes" array. Field order follows Windows Terminal's own documentation, and
// the tags are the key names it requires.
type windowsTerminalSchemeJSON struct {
	Name                string `json:"name"`
	Background          string `json:"background"`
	Foreground          string `json:"foreground"`
	CursorColor         string `json:"cursorColor"`
	SelectionBackground string `json:"selectionBackground"`
	Black               string `json:"black"`
	Red                 string `json:"red"`
	Green               string `json:"green"`
	Yellow              string `json:"yellow"`
	Blue                string `json:"blue"`
	Purple              string `json:"purple"`
	Cyan                string `json:"cyan"`
	White               string `json:"white"`
	BrightBlack         string `json:"brightBlack"`
	BrightRed           string `json:"brightRed"`
	BrightGreen         string `json:"brightGreen"`
	BrightYellow        string `json:"brightYellow"`
	BrightBlue          string `json:"brightBlue"`
	BrightPurple        string `json:"brightPurple"`
	BrightCyan          string `json:"brightCyan"`
	BrightWhite         string `json:"brightWhite"`
}

// windowsTerminalSchemeObject turns the scheme into the struct Windows Terminal
// serialises, so the colour keys and the JSON field names stay in one mapping.
func windowsTerminalSchemeObject(scheme windowsTerminalScheme) windowsTerminalSchemeJSON {
	obj := windowsTerminalSchemeJSON{Name: scheme.Name}
	for _, color := range scheme.Colors {
		switch color.Key {
		case "background":
			obj.Background = color.Value
		case "foreground":
			obj.Foreground = color.Value
		case "cursorColor":
			obj.CursorColor = color.Value
		case "selectionBackground":
			obj.SelectionBackground = color.Value
		case "black":
			obj.Black = color.Value
		case "red":
			obj.Red = color.Value
		case "green":
			obj.Green = color.Value
		case "yellow":
			obj.Yellow = color.Value
		case "blue":
			obj.Blue = color.Value
		case "purple":
			obj.Purple = color.Value
		case "cyan":
			obj.Cyan = color.Value
		case "white":
			obj.White = color.Value
		case "brightBlack":
			obj.BrightBlack = color.Value
		case "brightRed":
			obj.BrightRed = color.Value
		case "brightGreen":
			obj.BrightGreen = color.Value
		case "brightYellow":
			obj.BrightYellow = color.Value
		case "brightBlue":
			obj.BrightBlue = color.Value
		case "brightPurple":
			obj.BrightPurple = color.Value
		case "brightCyan":
			obj.BrightCyan = color.Value
		case "brightWhite":
			obj.BrightWhite = color.Value
		}
	}
	return obj
}

// ============================================================================
// THE JSONC EDITOR: EXACT BYTE SPANS, SO COMMENTS AND OTHER KEYS SURVIVE
// ============================================================================
//
// settings.json is JSONC: it carries // and /* */ comments that Windows Terminal
// writes itself and a user's own keys this installer must not touch. The editor
// below therefore never re-serialises the document. It parses it into a tree
// that records the byte span of every value, and every edit replaces or inserts
// bytes at a computed offset -- so a comment, an unrelated key or a differently
// indented line survives unless it is inside the one key being replaced.

// jsoncKind is the shape of one parsed JSONC value.
type jsoncKind int

const (
	jsoncObject jsoncKind = iota
	jsoncArray
	jsoncString
	jsoncNumber
	jsoncBool
	jsoncNull
)

// jsoncMember is one key and its value inside a JSONC object. keyStart is the
// byte the key string begins at, kept so an inserted sibling can be indented
// like the members already there.
type jsoncMember struct {
	key      string
	keyStart int
	value    *jsoncValue
}

// jsoncValue is one parsed JSONC value together with the byte span it occupies
// in the source. The span is what lets an edit replace exactly one value and
// leave every other byte untouched.
type jsoncValue struct {
	kind     jsoncKind
	start    int
	end      int
	text     string
	members  []jsoncMember
	elements []*jsoncValue
}

// findMember returns the member named key, or nil when the value is not an
// object or does not carry that key.
func (v *jsoncValue) findMember(key string) *jsoncMember {
	if v == nil || v.kind != jsoncObject {
		return nil
	}
	for i := range v.members {
		if v.members[i].key == key {
			return &v.members[i]
		}
	}
	return nil
}

// jsoncParser is a recursive-descent reader for the JSON-with-comments shape
// Windows Terminal writes. It records source offsets, so a value can be replaced
// without re-serialising the document.
type jsoncParser struct {
	src []byte
	pos int
}

// parseJSONC reads a whole document and rejects trailing text, so a truncated or
// concatenated file is an error rather than a file the editor half-understands.
func parseJSONC(src []byte) (*jsoncValue, error) {
	p := &jsoncParser{src: src}
	if err := p.skipTrivia(); err != nil {
		return nil, err
	}
	value, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	if err := p.skipTrivia(); err != nil {
		return nil, err
	}
	if p.pos != len(p.src) {
		return nil, fmt.Errorf("unexpected text after the top-level value at byte %d", p.pos)
	}
	return value, nil
}

// jsoncRootObject parses a document and requires its top level to be an object.
func jsoncRootObject(src []byte) (*jsoncValue, error) {
	root, err := parseJSONC(src)
	if err != nil {
		return nil, fmt.Errorf("the file is not valid JSONC: %w", err)
	}
	if root.kind != jsoncObject {
		return nil, fmt.Errorf("the file's top level is not an object")
	}
	return root, nil
}

// jsoncDefaults locates the profiles.defaults object the owned settings live in.
func jsoncDefaults(src []byte) (*jsoncValue, error) {
	root, err := jsoncRootObject(src)
	if err != nil {
		return nil, err
	}
	profiles := root.findMember("profiles")
	if profiles == nil || profiles.value.kind != jsoncObject {
		return nil, fmt.Errorf("there is no profiles object")
	}
	defaults := profiles.value.findMember("defaults")
	if defaults == nil || defaults.value.kind != jsoncObject {
		return nil, fmt.Errorf("there is no profiles.defaults object")
	}
	return defaults.value, nil
}

// skipTrivia advances past whitespace and comments.
func (p *jsoncParser) skipTrivia() error {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			p.pos++
		case c == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '/':
			p.pos += 2
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		case c == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '*':
			end := bytes.Index(p.src[p.pos+2:], []byte("*/"))
			if end < 0 {
				return fmt.Errorf("unterminated block comment at byte %d", p.pos)
			}
			p.pos += 2 + end + 2
		default:
			return nil
		}
	}
	return nil
}

// parseValue reads one value.
func (p *jsoncParser) parseValue() (*jsoncValue, error) {
	if err := p.skipTrivia(); err != nil {
		return nil, err
	}
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("unexpected end of file")
	}
	start := p.pos
	switch c := p.src[p.pos]; {
	case c == '{':
		return p.parseObject(start)
	case c == '[':
		return p.parseArray(start)
	case c == '"':
		text, err := p.parseString()
		if err != nil {
			return nil, err
		}
		return &jsoncValue{kind: jsoncString, start: start, end: p.pos, text: text}, nil
	default:
		return p.parseLiteral(start)
	}
}

// parseObject reads an object and records each member's key span.
func (p *jsoncParser) parseObject(start int) (*jsoncValue, error) {
	p.pos++ // '{'
	value := &jsoncValue{kind: jsoncObject, start: start}

	if err := p.skipTrivia(); err != nil {
		return nil, err
	}
	if p.pos < len(p.src) && p.src[p.pos] == '}' {
		p.pos++
		value.end = p.pos
		return value, nil
	}

	for {
		if err := p.skipTrivia(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.src) || p.src[p.pos] != '"' {
			return nil, fmt.Errorf("expected a member name at byte %d", p.pos)
		}
		keyStart := p.pos
		key, err := p.parseString()
		if err != nil {
			return nil, err
		}
		if err := p.skipTrivia(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.src) || p.src[p.pos] != ':' {
			return nil, fmt.Errorf("expected ':' after %q at byte %d", key, p.pos)
		}
		p.pos++
		member, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		value.members = append(value.members, jsoncMember{key: key, keyStart: keyStart, value: member})

		if err := p.skipTrivia(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("unterminated object at byte %d", start)
		}
		switch p.src[p.pos] {
		case ',':
			p.pos++
			if err := p.skipTrivia(); err != nil {
				return nil, err
			}
			if p.pos < len(p.src) && p.src[p.pos] == '}' {
				p.pos++
				value.end = p.pos
				return value, nil
			}
		case '}':
			p.pos++
			value.end = p.pos
			return value, nil
		default:
			return nil, fmt.Errorf("expected ',' or '}' at byte %d", p.pos)
		}
	}
}

// parseArray reads an array, accepting a trailing comma.
func (p *jsoncParser) parseArray(start int) (*jsoncValue, error) {
	p.pos++ // '['
	value := &jsoncValue{kind: jsoncArray, start: start}

	if err := p.skipTrivia(); err != nil {
		return nil, err
	}
	if p.pos < len(p.src) && p.src[p.pos] == ']' {
		p.pos++
		value.end = p.pos
		return value, nil
	}

	for {
		element, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		value.elements = append(value.elements, element)

		if err := p.skipTrivia(); err != nil {
			return nil, err
		}
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("unterminated array at byte %d", start)
		}
		switch p.src[p.pos] {
		case ',':
			p.pos++
			if err := p.skipTrivia(); err != nil {
				return nil, err
			}
			if p.pos < len(p.src) && p.src[p.pos] == ']' {
				p.pos++
				value.end = p.pos
				return value, nil
			}
		case ']':
			p.pos++
			value.end = p.pos
			return value, nil
		default:
			return nil, fmt.Errorf("expected ',' or ']' at byte %d", p.pos)
		}
	}
}

// parseString reads a quoted string and decodes its escapes.
func (p *jsoncParser) parseString() (string, error) {
	p.pos++ // opening quote
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch c {
		case '"':
			p.pos++
			return b.String(), nil
		case '\\':
			p.pos++
			if p.pos >= len(p.src) {
				return "", fmt.Errorf("unterminated escape at byte %d", p.pos)
			}
			switch e := p.src[p.pos]; e {
			case '"', '\\', '/':
				b.WriteByte(e)
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'u':
				if p.pos+4 >= len(p.src) {
					return "", fmt.Errorf("truncated \\u escape at byte %d", p.pos)
				}
				n, err := strconv.ParseUint(string(p.src[p.pos+1:p.pos+5]), 16, 32)
				if err != nil {
					return "", fmt.Errorf("invalid \\u escape at byte %d", p.pos)
				}
				b.WriteRune(rune(n))
				p.pos += 4
			default:
				return "", fmt.Errorf("invalid escape \\%c at byte %d", e, p.pos)
			}
			p.pos++
		default:
			b.WriteByte(c)
			p.pos++
		}
	}
	return "", fmt.Errorf("unterminated string at byte %d", p.pos)
}

// parseLiteral reads a number, boolean or null up to its delimiter.
func (p *jsoncParser) parseLiteral(start int) (*jsoncValue, error) {
	for p.pos < len(p.src) {
		switch c := p.src[p.pos]; c {
		case ',', ']', '}', ' ', '\t', '\n', '\r', '/':
			goto done
		}
		p.pos++
	}
done:
	if p.pos == start {
		return nil, fmt.Errorf("expected a value at byte %d", start)
	}
	text := string(p.src[start:p.pos])
	kind := jsoncNumber
	switch text {
	case "true", "false":
		kind = jsoncBool
	case "null":
		kind = jsoncNull
	default:
		if _, err := strconv.ParseFloat(text, 64); err != nil {
			return nil, fmt.Errorf("unrecognised value %q at byte %d", text, start)
		}
	}
	return &jsoncValue{kind: kind, start: start, end: p.pos, text: text}, nil
}

// jsoncLineIndent is the whitespace between the start of the line offset sits on
// and offset itself, or the empty string when anything else precedes it. An
// inserted member is indented to match the members already there, and a compact
// one-line object simply gets no indentation rather than a wrong guess.
func jsoncLineIndent(src []byte, offset int) string {
	if offset < 0 || offset > len(src) {
		return ""
	}
	lineStart := bytes.LastIndexByte(src[:offset], '\n') + 1
	for _, c := range src[lineStart:offset] {
		if c != ' ' && c != '\t' {
			return ""
		}
	}
	return string(src[lineStart:offset])
}

// jsoncSkipTriviaAt advances from pos past whitespace and comments, for reading
// the source between two parsed spans.
func jsoncSkipTriviaAt(src []byte, pos int) int {
	for pos < len(src) {
		c := src[pos]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			pos++
		case c == '/' && pos+1 < len(src) && src[pos+1] == '/':
			pos += 2
			for pos < len(src) && src[pos] != '\n' {
				pos++
			}
		case c == '/' && pos+1 < len(src) && src[pos+1] == '*':
			end := bytes.Index(src[pos+2:], []byte("*/"))
			if end < 0 {
				return len(src)
			}
			pos += 2 + end + 2
		default:
			return pos
		}
	}
	return pos
}

// jsoncInsertion builds the bytes inserted immediately before the closing token
// of an object or array, matching the indentation of the elements already there
// and adding the comma only when the container does not already end in one.
func jsoncInsertion(src []byte, container *jsoncValue, close int, entry string) string {
	indent := jsoncLineIndent(src, close)
	childIndent := indent + "    "
	switch container.kind {
	case jsoncObject:
		if len(container.members) > 0 {
			childIndent = jsoncLineIndent(src, container.members[0].keyStart)
		}
	case jsoncArray:
		if len(container.elements) > 0 {
			childIndent = jsoncLineIndent(src, container.elements[0].start)
		}
	}

	prefix := ""
	if jsoncHasElement(container) && !jsoncHasTrailingComma(src, container, close) {
		prefix = ","
	}
	return prefix + "\n" + childIndent + entry + "\n" + indent
}

// jsoncHasElement reports whether the container has any parsed member or
// element.
func jsoncHasElement(container *jsoncValue) bool {
	switch container.kind {
	case jsoncObject:
		return len(container.members) > 0
	case jsoncArray:
		return len(container.elements) > 0
	default:
		return false
	}
}

// jsoncHasTrailingComma reports whether the last member or element is already
// followed by a comma, so an insertion does not double it.
func jsoncHasTrailingComma(src []byte, container *jsoncValue, close int) bool {
	var lastEnd int
	switch container.kind {
	case jsoncObject:
		if len(container.members) == 0 {
			return false
		}
		lastEnd = container.members[len(container.members)-1].value.end
	case jsoncArray:
		if len(container.elements) == 0 {
			return false
		}
		lastEnd = container.elements[len(container.elements)-1].end
	default:
		return false
	}
	pos := jsoncSkipTriviaAt(src, lastEnd)
	return pos < close && src[pos] == ','
}

// spliceJSONC replaces the bytes in [start,end) with insert.
func spliceJSONC(src []byte, start, end int, insert string) []byte {
	out := make([]byte, 0, len(src)-(end-start)+len(insert))
	out = append(out, src[:start]...)
	out = append(out, insert...)
	out = append(out, src[end:]...)
	return out
}

// jsoncInsertMember inserts key: value before an object's closing brace.
func jsoncInsertMember(src []byte, obj *jsoncValue, key, value string) ([]byte, error) {
	close := obj.end - 1
	if close < 0 || close >= len(src) || src[close] != '}' {
		return nil, fmt.Errorf("the object does not end with a closing brace")
	}
	encodedKey, err := json.Marshal(key)
	if err != nil {
		return nil, err
	}
	entry := string(encodedKey) + ": " + value
	return spliceJSONC(src, close, close, jsoncInsertion(src, obj, close, entry)), nil
}

// jsoncInsertArrayElement inserts value before an array's closing bracket.
func jsoncInsertArrayElement(src []byte, arr *jsoncValue, value string) ([]byte, error) {
	close := arr.end - 1
	if close < 0 || close >= len(src) || src[close] != ']' {
		return nil, fmt.Errorf("the array does not end with a closing bracket")
	}
	return spliceJSONC(src, close, close, jsoncInsertion(src, arr, close, value)), nil
}

// jsoncReplaceString replaces a string value with a new one, re-encoding it so
// the result is valid JSONC whatever it contains.
func jsoncReplaceString(src []byte, value *jsoncValue, replacement string) ([]byte, error) {
	encoded, err := json.Marshal(replacement)
	if err != nil {
		return nil, err
	}
	return spliceJSONC(src, value.start, value.end, string(encoded)), nil
}

// jsoncSetString sets a string member, replacing the value in place when the key
// exists and inserting the member when it does not. A key present with a
// different shape is refused rather than overwritten.
func jsoncSetString(src []byte, obj *jsoncValue, key, value string) ([]byte, error) {
	if member := obj.findMember(key); member != nil {
		if member.value.kind != jsoncString {
			return nil, fmt.Errorf("%q is not a string", key)
		}
		return jsoncReplaceString(src, member.value, value)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return jsoncInsertMember(src, obj, key, string(encoded))
}

// windowsTerminalSettingsContent is the whole edit: it sets
// profiles.defaults.font.face to the resolved face and
// profiles.defaults.colorScheme and registers the scheme, and it touches nothing
// else. The face is a parameter rather than the constant, because the writer
// names the family the Windows font store resolved, which is not known until
// install time. The first half of the function re-parses after every structural
// insert because the byte offsets shift; that keeps each edit anchored to the
// tree it was read from.
func windowsTerminalSettingsContent(src []byte, scheme windowsTerminalScheme, face string) ([]byte, error) {
	if _, err := jsoncRootObject(src); err != nil {
		return nil, err
	}
	if _, err := jsoncDefaults(src); err != nil {
		return nil, err
	}

	updated, err := jsoncEnsureFontFace(src, face)
	if err != nil {
		return nil, err
	}
	updated, err = jsoncSetDefaultsColorScheme(updated, scheme.Name)
	if err != nil {
		return nil, err
	}
	return jsoncEnsureScheme(updated, scheme)
}

// jsoncEnsureFontFace sets profiles.defaults.font.face to face, creating the
// font object first when the defaults object carries none. face is the family
// the Windows font store resolved, so the editor never names a face by hand.
func jsoncEnsureFontFace(src []byte, face string) ([]byte, error) {
	defaults, err := jsoncDefaults(src)
	if err != nil {
		return nil, err
	}

	font := defaults.findMember("font")
	if font == nil {
		updated, err := jsoncInsertMember(src, defaults, "font", "{}")
		if err != nil {
			return nil, err
		}
		src = updated
		defaults, err = jsoncDefaults(src)
		if err != nil {
			return nil, err
		}
		font = defaults.findMember("font")
	}
	if font.value.kind != jsoncObject {
		return nil, fmt.Errorf("profiles.defaults.font is not an object")
	}
	return jsoncSetString(src, font.value, "face", face)
}

// jsoncSetDefaultsColorScheme sets profiles.defaults.colorScheme.
func jsoncSetDefaultsColorScheme(src []byte, name string) ([]byte, error) {
	defaults, err := jsoncDefaults(src)
	if err != nil {
		return nil, err
	}
	return jsoncSetString(src, defaults, "colorScheme", name)
}

// jsoncEnsureScheme adds the schemes array when the file has none, then upserts
// the owned scheme into it.
func jsoncEnsureScheme(src []byte, scheme windowsTerminalScheme) ([]byte, error) {
	root, err := jsoncRootObject(src)
	if err != nil {
		return nil, err
	}
	member := root.findMember("schemes")
	if member == nil {
		updated, err := jsoncInsertMember(src, root, "schemes", "[]")
		if err != nil {
			return nil, err
		}
		src = updated
		root, err = jsoncRootObject(src)
		if err != nil {
			return nil, err
		}
		member = root.findMember("schemes")
	}
	if member.value.kind != jsoncArray {
		return nil, fmt.Errorf("schemes is not an array")
	}
	return jsoncUpsertScheme(src, scheme)
}

// jsoncUpsertScheme updates the owned scheme's colours in place when it exists,
// or appends a fresh object when it does not. Other schemes are never touched.
func jsoncUpsertScheme(src []byte, scheme windowsTerminalScheme) ([]byte, error) {
	root, err := jsoncRootObject(src)
	if err != nil {
		return nil, err
	}
	if existing := findJSONCScheme(root.findMember("schemes").value, scheme.Name); existing != nil {
		current := src
		for _, color := range scheme.Colors {
			updated, err := jsoncSetSchemeColor(current, scheme.Name, color)
			if err != nil {
				return nil, err
			}
			current = updated
		}
		return current, nil
	}

	encoded, err := json.Marshal(windowsTerminalSchemeObject(scheme))
	if err != nil {
		return nil, err
	}
	root, err = jsoncRootObject(src)
	if err != nil {
		return nil, err
	}
	return jsoncInsertArrayElement(src, root.findMember("schemes").value, string(encoded))
}

// jsoncSetSchemeColor sets one colour on the owned scheme, re-locating it after
// the previous edit moved its offsets.
func jsoncSetSchemeColor(src []byte, name string, color windowsTerminalSchemeColor) ([]byte, error) {
	root, err := jsoncRootObject(src)
	if err != nil {
		return nil, err
	}
	target := findJSONCScheme(root.findMember("schemes").value, name)
	if target == nil {
		return nil, fmt.Errorf("the %q scheme disappeared during the edit", name)
	}
	return jsoncSetString(src, target, color.Key, color.Value)
}

// findJSONCScheme returns the scheme object an array holds under a name.
func findJSONCScheme(arr *jsoncValue, name string) *jsoncValue {
	if arr == nil || arr.kind != jsoncArray {
		return nil
	}
	for _, element := range arr.elements {
		if element.kind != jsoncObject {
			continue
		}
		member := element.findMember("name")
		if member != nil && member.value.kind == jsoncString && member.value.text == name {
			return element
		}
	}
	return nil
}

// ============================================================================
// THE FONT: PER-USER REGISTRATION AND THE VERIFICATION BEFORE THE FACE
// ============================================================================
//
// Windows Terminal can only use a family the Windows font store resolves. A
// font under ~/.local/share/fonts lives in the distribution's ext4 filesystem,
// which a Windows program cannot read, so the face named in settings.json would
// fall back silently -- exactly the defect issue #261 describes. The step
// therefore copies a handful of the faces the Linux step already extracted into
// the per-user Windows font store and registers them under HKCU, which needs no
// administrator, and then asks the Windows font store whether the family
// resolves. The face is written only when that answer is yes; when the answer
// cannot be had, the face is not written and the step says why.

const (
	// windowsFontStoreRelPath is %LOCALAPPDATA%\Microsoft\Windows\Fonts,
	// the per-user font store, relative to the Windows profile directory.
	windowsFontStoreRelPath = "AppData/Local/Microsoft/Windows/Fonts"

	// windowsFontFamilyFilePrefix is the file name prefix the Meslo Nerd Fonts
	// archive gives every face of the family the installer installs. It is
	// "MesloLGSNerdFont-", not the family name: the patcher names the file after
	// the font ("MesloLGS") and the family after the release ("MesloLGS Nerd Font").
	windowsFontFamilyFilePrefix = "MesloLGSNerdFont-"

	// windowsFontPresent and windowsFontAbsent are the two answers the
	// verification query prints. Anything else is "no answer" and refuses the
	// face rather than guessing.
	windowsFontPresent = "font=present"
	windowsFontAbsent  = "font=absent"
)

// windowsFontFaceCandidates is the preference order the writer resolves the face
// against: the machine's own Meslo build first, the family the installer's
// archive provides second. The order is the rule, and it is stated once.
var windowsFontFaceCandidates = []string{
	windowsTerminalFontFacePreferred,
	windowsTerminalFontFace,
}

// windowsFontFace is one file copied to the Windows store and the style label
// Windows uses in the registry value name. Only four faces travel: the whole
// family is 81 files and about a gigabyte, and Windows Terminal needs a
// regular, a bold, an italic and a bold italic, not the set.
var windowsFontFaces = []windowsFontFace{
	{File: windowsFontFamilyFilePrefix + "Regular.ttf", Label: "Regular"},
	{File: windowsFontFamilyFilePrefix + "Bold.ttf", Label: "Bold"},
	{File: windowsFontFamilyFilePrefix + "Italic.ttf", Label: "Italic"},
	{File: windowsFontFamilyFilePrefix + "BoldItalic.ttf", Label: "Bold Italic"},
}

type windowsFontFace struct {
	File  string
	Label string
}

// windowsFontQuery runs one Windows-side PowerShell program. It is a variable so
// a test replaces the interop boundary; production uses system.WindowsQuery,
// which is bounded by WindowsQueryTimeout with WaitDelay.
var windowsFontQuery = system.WindowsQuery

// windowsFontQueryTimeout is the deadline each Windows-side font query runs
// under. It is the shared WindowsQueryTimeout; it is a variable so a test can
// shorten it and prove a hanging query is bounded rather than blocking.
var windowsFontQueryTimeout = system.WindowsQueryTimeout

// runWindowsFontQuery bounds one font query with the shared timeout. The caller
// gets a reported failure on a timeout, never an installer that stops
// responding (constraint 5).
func runWindowsFontQuery(script string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), windowsFontQueryTimeout)
	defer cancel()
	return windowsFontQuery(ctx, script)
}

// psSingleQuote renders s as a PowerShell single-quoted literal, so a value can
// never escape the string it is embedded in.
func psSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// windowsFontRegisterScript is the whole registration: one HKCU value per copied
// face, named the way Windows names a per-user font and pointing at the absolute
// path a per-user face is resolved by. It writes nothing outside the current
// user's registry hive.
//
// Two things here are load-bearing, and both were wrong once -- measured on a
// machine rather than argued about:
//
//   - The key is created only when it is missing. It is not this installer's key:
//     it holds every per-user font on the machine. PowerShell's New-Item -Force
//     recreates a key that already exists and discards the values it held, so an
//     unguarded New-Item over this path uninstalls every per-user font the
//     installer did not put there. The run that was measured destroyed 36
//     registrations -- the whole CaskaydiaCove Nerd Font family -- and left the
//     .ttf files in the store with nothing to resolve them by.
//   - The value is the absolute path, built on the Windows side out of
//     %LOCALAPPDATA% with Join-Path. A bare file name is resolved against
//     %WINDIR%\Fonts, where these faces are not, so a value naming one registers
//     a face Windows cannot follow -- and the verification that gates the write
//     then refuses, correctly, on every run.
func windowsFontRegisterScript(faces []windowsFontFace) string {
	var b strings.Builder
	b.WriteString("$key = 'HKCU:\\Software\\Microsoft\\Windows NT\\CurrentVersion\\Fonts'\n")
	// Only when it is missing: New-Item -Force on a key that already exists
	// discards every value it holds.
	b.WriteString("if (-not (Test-Path $key)) { New-Item -Path $key -Force | Out-Null }\n")
	// The store is %LOCALAPPDATA% on Windows, which only the Windows side knows,
	// so the program resolves it there rather than being handed a path some other
	// layer guessed.
	b.WriteString("$store = Join-Path $env:LOCALAPPDATA 'Microsoft\\Windows\\Fonts'\n")
	for _, face := range faces {
		value := windowsTerminalFontFace + " " + face.Label + " (TrueType)"
		fmt.Fprintf(&b, "New-ItemProperty -Path $key -Name %s -Value (Join-Path $store %s) -PropertyType String -Force | Out-Null\n",
			psSingleQuote(value), psSingleQuote(face.File))
	}
	return b.String()
}

// windowsFontFaceQueryScript asks the Windows font store, not the registry,
// whether a family resolves. System.Drawing.Text.InstalledFontCollection is the
// list the renderer resolves a face against, so an answer of present is the same
// fact Windows Terminal would see. It is the one question every face check asks,
// so a name cannot be changed in one check and left behind in another.
func windowsFontFaceQueryScript(face string) string {
	return "Add-Type -AssemblyName System.Drawing; " +
		"$names = (New-Object System.Drawing.Text.InstalledFontCollection).Families | ForEach-Object { $_.Name }; " +
		"if ($names -contains " + psSingleQuote(face) + ") { " +
		psSingleQuote(windowsFontPresent) + " } else { " + psSingleQuote(windowsFontAbsent) + " }"
}

// windowsFontVerifyScript asks whether the family the installer registers
// resolves.
func windowsFontVerifyScript() string {
	return windowsFontFaceQueryScript(windowsTerminalFontFace)
}

// installedWindowsFontFace resolves the face the writer should name against the
// Windows font store, preferring the machine's own build. It copies and
// registers nothing: asking is read-only, so the interactive offer can name the
// same face the write would use. A store that answers nothing, or errors, for a
// candidate is treated as that candidate not resolving.
func installedWindowsFontFace() (string, bool) {
	for _, face := range windowsFontFaceCandidates {
		output, err := runWindowsFontQuery(windowsFontFaceQueryScript(face))
		if err != nil {
			continue
		}
		if present, known := parseWindowsFontPresence(output); known && present {
			return face, true
		}
	}
	return "", false
}

// parseWindowsFontPresence reads the verification answer. A BOM a redirected
// Windows PowerShell console can prepend is stripped first, and any output that
// is not one of the two answers is "no answer", which the caller treats as a
// refusal rather than as an absent font.
func parseWindowsFontPresence(output []byte) (present bool, known bool) {
	for _, line := range strings.Split(strings.TrimPrefix(string(output), "\ufeff"), "\n") {
		switch strings.TrimSpace(line) {
		case windowsFontPresent:
			return true, true
		case windowsFontAbsent:
			return false, true
		}
	}
	return false, false
}

// installAndVerifyWindowsFont copies the handful of faces the Linux step
// extracted into the Windows per-user font store, registers them under HKCU, and
// confirms the family resolves there. It is the gate constraint 1 requires: a
// non-nil error means the family could not be confirmed, and the caller must not
// write font.face.
func installAndVerifyWindowsFont(profileDir string) error {
	fontDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "fonts")
	dest := filepath.Join(profileDir, filepath.FromSlash(windowsFontStoreRelPath))

	var copied []windowsFontFace
	for _, face := range windowsFontFaces {
		data, err := os.ReadFile(filepath.Join(fontDir, face.File))
		if err != nil {
			continue
		}
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return fmt.Errorf("creating the Windows font store at %s: %w", dest, err)
		}
		if err := os.WriteFile(filepath.Join(dest, face.File), data, 0o644); err != nil {
			return fmt.Errorf("copying %s into the Windows font store: %w", face.File, err)
		}
		copied = append(copied, face)
	}
	if len(copied) == 0 {
		return fmt.Errorf("no %s*.ttf faces were found in %s, so the Linux font step did not run or did not extract the family", windowsFontFamilyFilePrefix, fontDir)
	}

	if _, err := runWindowsFontQuery(windowsFontRegisterScript(copied)); err != nil {
		return fmt.Errorf("registering the family under HKCU failed: %w", err)
	}

	output, err := runWindowsFontQuery(windowsFontVerifyScript())
	if err != nil {
		return fmt.Errorf("the family %q could not be verified in the Windows font store: %w", windowsTerminalFontFace, err)
	}
	present, known := parseWindowsFontPresence(output)
	if !known {
		return fmt.Errorf("the Windows font store gave no answer for %q, so the face was not written", windowsTerminalFontFace)
	}
	if !present {
		return fmt.Errorf("%q is not present in the Windows font store after registering it, so the face was not written", windowsTerminalFontFace)
	}
	return nil
}

// windowsTerminalThemeDefinition loads the palette the Windows Terminal scheme
// is built from. The scheme carries the colours of the theme this machine has
// applied -- the name the installer's own record holds -- so the Windows side is
// painted from the palette the shell, the prompt and every other tool on the
// Linux side are actually using. Loading the default theme instead would put the
// two sides of the boundary on different palettes on any machine whose theme is
// not the default.
//
// The scheme's name does not change with the theme: it is the one dotfiles
// scheme and a theme change repaints it, because Windows Terminal resolves it by
// the name profiles.defaults.colorScheme holds.
//
// Three answers end at the default theme, and all three are deliberate. A
// machine with no record has applied nothing, so the default -- the theme a fresh
// install applies -- is its palette. A record this machine cannot read is treated
// the same way. A record naming a theme this repository no longer defines falls
// back too rather than failing: this function's error abandons the whole Windows
// side of the step, font face included, and a stale record must not cost a font
// that has nothing to do with the theme it names.
func windowsTerminalThemeDefinition(repoDir string) (themeDefinition, error) {
	defs, err := loadThemeDefinitions(repoDir)
	if err != nil {
		return themeDefinition{}, err
	}

	id := defaultThemeID
	if rec := readDotfilesThemeRecord(); rec != nil && rec.Theme != "" {
		if _, ok := themeByID(defs, rec.Theme); ok {
			id = rec.Theme
		}
	}

	def, ok := themeByID(defs, id)
	if !ok {
		return themeDefinition{}, fmt.Errorf("the %q theme definition is not in %s/%s", id, repoDir, themesDirName)
	}
	if !def.Complete() {
		return themeDefinition{}, fmt.Errorf("the %q theme is incomplete, so Windows Terminal's scheme would be incomplete", def.ID)
	}
	return def, nil
}

// configureWindowsTerminalRenderer leaves the Windows side of a WSL install in a
// state where the next Windows Terminal start renders the theme. It never fails
// the step: a Windows-side problem is reported and skipped, because the shell is
// useful whether or not the renderer was reached.
//
// The two paths are deliberate and distinct (the maintainer's decision on #261):
// under --non-interactive the flag is already an explicit choice, so the font is
// installed, the family is verified, and the settings are written; outside it the
// offer is shown and nothing is written, because docs/tui-installer.md promises a
// write only after an explicit yes.
func configureWindowsTerminalRenderer(repoDir, profileDir string) {
	def, err := windowsTerminalThemeDefinition(repoDir)
	if err != nil {
		wslLog(wslStepID, "Windows Terminal was not configured: "+err.Error())
		return
	}

	// Resolve the face before the offer is built, so the offer, the write and
	// the settings all name the same family. The name is never hand-written: a
	// face the Windows font store cannot resolve falls back silently, so the
	// store is asked and its answer is what is named. The machine's own Meslo
	// build is preferred; the family the installer's archive provides is the
	// fallback. Asking is read-only and changes nothing.
	face, resolved := installedWindowsFontFace()
	if !resolved {
		face = windowsTerminalFontFace
	}

	offer := readWindowsTerminalOffer(profileDir, def, face)
	if !offer.Available {
		wslLog(wslStepID, "Windows Terminal was not configured: "+offer.Reason)
		return
	}

	if !nonInteractiveMode {
		for _, line := range describeWindowsTerminalOffer(offer) {
			wslLog(wslStepID, line)
		}
		wslLog(wslStepID, "The Nerd Font would also be copied into the Windows per-user font store and registered under HKCU once you choose to configure it.")
		wslLog(wslStepID, "Windows Terminal was left unchanged: the font face and the colour scheme are written only after an explicit yes.")
		return
	}

	// Nothing resolved, so the installer's own family has to be registered
	// before its face can be written. installAndVerifyWindowsFont is the gate
	// that confirms it resolves; a family that does not verify is not written.
	if !resolved {
		if err := installAndVerifyWindowsFont(profileDir); err != nil {
			wslLog(wslStepID, "Windows Terminal's font face was not written: "+err.Error())
			return
		}
	}

	notice, wrote, err := applyWindowsTerminalOffer(offer, true, wslStepID)
	if err != nil {
		wslLog(wslStepID, "Windows Terminal was not configured: "+err.Error())
		return
	}
	if wrote {
		wslLog(wslStepID, "✓ "+notice)
	}
	wslLog(wslStepID, "Windows Terminal reads its font list when it starts; reopen it to see the icons and the palette.")
}

// ============================================================================
// THE OFFER: WHAT WOULD CHANGE, AND THE WRITE ONLY AFTER A CHOICE
// ============================================================================

// windowsTerminalOffer is the read-only result of looking at the reachable
// settings.json: whether it can be edited safely, why not when it cannot, where
// it is, and the bytes a write would produce. updated is unexported on purpose,
// so the only way to reach the file is applyWindowsTerminalOffer.
type windowsTerminalOffer struct {
	Available bool
	Reason    string
	Path      string
	Face      string
	Scheme    string
	Original  []byte
	updated   []byte
}

// readWindowsTerminalOffer resolves the settings file, reads it, and computes
// the bytes a write would leave behind. It never writes: a refusal carries a
// reason and changes nothing, which is the correct behaviour when the file is
// unparseable, its shape is unexpected, or no writable mount holds it.
func readWindowsTerminalOffer(profileDir string, def themeDefinition, face string) windowsTerminalOffer {
	offer := windowsTerminalOffer{Face: face}

	scheme, err := windowsTerminalSchemeFor(def)
	if err != nil {
		offer.Reason = err.Error()
		return offer
	}
	offer.Scheme = scheme.Name

	path, err := windowsTerminalSettingsPath(profileDir)
	if err != nil {
		offer.Reason = err.Error()
		return offer
	}
	offer.Path = path

	original, err := os.ReadFile(path)
	if err != nil {
		offer.Reason = fmt.Sprintf("reading %s: %v", path, err)
		return offer
	}
	updated, err := windowsTerminalSettingsContent(original, scheme, face)
	if err != nil {
		offer.Reason = fmt.Sprintf("%s was not written: %v", path, err)
		return offer
	}

	offer.Available = true
	offer.Original = original
	offer.updated = updated
	return offer
}

// windowsTerminalUnsetValue is the left side of a change row whose setting the file
// does not carry. It is a fact and not a guess: an available offer proves the file
// was read and parsed, so an absent key is the only thing "unset" can mean.
const windowsTerminalUnsetValue = "(unset)"

// windowsTerminalOfferLabelColumn is the width the offer's labels are padded to, so
// the values line up in a column instead of starting wherever a label happens to end.
const windowsTerminalOfferLabelColumn = 11

// windowsTerminalOfferRow is one change: the setting's key in settings.json, the
// value the file holds today, and the value this installer would write. The two
// values are read against each other instead of being stated in a sentence, which is
// what lets the screen say what a write replaces without spending a row on saying
// that it replaces something.
func windowsTerminalOfferRow(key, current, next string) string {
	return fmt.Sprintf("  %-*s  %s > %s", windowsTerminalOfferLabelColumn, key, current, next)
}

// windowsTerminalOfferFact is one row of the offer that is not a change.
func windowsTerminalOfferFact(label, value string) string {
	return fmt.Sprintf("  %-*s  %s", windowsTerminalOfferLabelColumn, label, value)
}

// describeWindowsTerminalOffer is what the offer shows before the user chooses: the
// file a write would land in, the two settings with the value each holds today beside
// the value a write would set, and what happens to the rest of the file. It is a
// table rather than a paragraph because of where it is drawn: the panel sits in the
// installing body and takes the rows left after the live meters, which at the
// tightest terminal the frame guards measure is eight. The seven sentences it used to
// be needed more than that, and the rows that fell off the bottom first were the ones
// naming the value a write would replace -- the fact this screen exists to state.
//
// Eight content rows is the whole budget, and the path spends three or four of them
// depending on the platform, so the four rows left carry two facts each only where
// they belong together: the backup and what is kept are one row, because a write that
// takes a backup and loses nothing else is a single promise about the rest of the
// file. The keys are the ones windowsTerminalSettingsContent writes, and the face is
// the resolved one rather than the constant, so the screen cannot name a setting or a
// family the writer does not use.
func describeWindowsTerminalOffer(offer windowsTerminalOffer) []string {
	if !offer.Available {
		return []string{offer.Reason}
	}
	present := func(read func([]byte) (string, bool)) string {
		if value, ok := read(offer.Original); ok {
			return value
		}
		return windowsTerminalUnsetValue
	}
	return []string{
		offer.Path,
		windowsTerminalOfferRow("font.face", present(windowsTerminalCurrentFontFace), offer.Face),
		windowsTerminalOfferRow("colorScheme", present(windowsTerminalCurrentColorScheme), offer.Scheme),
		windowsTerminalOfferFact("backup", ".bak-dotfiles-<stamp>, rest kept"),
	}
}

// applyWindowsTerminalOffer is the only path that writes. chosen is the explicit
// yes: without it nothing is written and no backup is made. With it, the previous
// bytes are copied beside the file before the write, and a backup that cannot be
// made aborts the write rather than leaving the user without one. The notice it
// returns is read back from the file, so it reports the face and the colour
// scheme the file actually holds rather than the bytes that were sent.
func applyWindowsTerminalOffer(offer windowsTerminalOffer, chosen bool, stepID string) (notice string, wrote bool, err error) {
	if !offer.Available {
		return offer.Reason, false, nil
	}
	if !chosen {
		return "Windows Terminal was left unchanged: the font face and the colour scheme are only written when you choose to configure it.", false, nil
	}
	if dryRun() {
		return fmt.Sprintf("DRY RUN: %s was not changed.", offer.Path), false, nil
	}

	backup := fmt.Sprintf("%s.bak-dotfiles-%s", offer.Path, time.Now().Format("20060102-150405"))
	if err := os.WriteFile(backup, offer.Original, 0o644); err != nil {
		return "", false, fmt.Errorf("backing up %s to %s: %w", offer.Path, backup, err)
	}
	if err := os.WriteFile(offer.Path, offer.updated, 0o644); err != nil {
		return "", false, fmt.Errorf("writing %s: %w", offer.Path, err)
	}

	// The report is read from the file, and a landing that did not land whole is returned as
	// an error rather than only as words: a caller that sees no error prints a checkmark, and
	// a checkmark over a half-written settings file is the defect this reports.
	notice, whole := windowsTerminalWriteReport(offer.Path, offer.Face, offer.Scheme, backup)
	if !whole {
		return notice, true, fmt.Errorf("%s did not land in %s, so the settings Windows Terminal reads are not the ones this installer wrote",
			strings.Join(windowsTerminalMissingLabels(offer.Path, offer.Face, offer.Scheme), " and "), offer.Path)
	}
	return notice, true, nil
}

// windowsTerminalMissingLabels names the settings the file does not hold as sent, for the
// shortfall message. It asks the file rather than the notice, so the message and the report
// cannot disagree.
func windowsTerminalMissingLabels(path, sentFace, sentScheme string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{"the font face", "the colour scheme"}
	}
	face, faceOK := windowsTerminalCurrentFontFace(data)
	scheme, schemeOK := windowsTerminalCurrentColorScheme(data)
	missing := make([]string, 0, 2)
	if !faceOK || face != sentFace {
		missing = append(missing, "the font face")
	}
	if !schemeOK || scheme != sentScheme {
		missing = append(missing, "the colour scheme")
	}
	return missing
}

// windowsTerminalWriteReport reads the settings file Windows Terminal will read,
// after the write, and reports the font face and the colour scheme it now holds.
// The bytes that were sent are not evidence: a write that did not land would
// otherwise be reported as a success. The two values are checked on their own,
// and a value the file does not hold is named as the value that did not land, so
// the report and the file can only agree.
func windowsTerminalWriteReport(path, sentFace, sentScheme, backup string) (notice string, whole bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("Windows Terminal settings were written to %s, but the file could not be read back to confirm the font face and the colour scheme: %v; the previous file is at %s.",
			path, err, backup), false
	}

	face, faceOK := windowsTerminalCurrentFontFace(data)
	scheme, schemeOK := windowsTerminalCurrentColorScheme(data)
	// A value landed only when the file names it and names the same value that was sent.
	faceLanded := faceOK && face == sentFace
	schemeLanded := schemeOK && scheme == sentScheme
	values := []string{
		windowsTerminalLandedValue("the font face", sentFace, face, faceOK),
		windowsTerminalLandedValue("the colour scheme", sentScheme, scheme, schemeOK),
	}
	return fmt.Sprintf("Windows Terminal now reads from %s: %s; the previous file is at %s.",
		path, strings.Join(values, " and "), backup), faceLanded && schemeLanded
}

// windowsTerminalLandedValue describes one owned setting by what the file holds:
// the value it landed with, or the value that did not land and what the file
// holds instead. It is the read-back half of the write report, so a value the
// file never received cannot be described as if it had.
func windowsTerminalLandedValue(label, sent, found string, ok bool) string {
	switch {
	case !ok:
		return fmt.Sprintf("%s %q did not land (the file names none)", label, sent)
	case found != sent:
		return fmt.Sprintf("%s %q did not land (the file names %q)", label, sent, found)
	default:
		return fmt.Sprintf("%s %q", label, found)
	}
}
