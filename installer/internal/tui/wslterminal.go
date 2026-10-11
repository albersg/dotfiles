package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	// windowsTerminalFontFace is the face Windows Terminal must select for the
	// Nerd Font glyphs the prompt draws. It is the same face the Linux terminals
	// are configured with.
	windowsTerminalFontFace = "IosevkaTerm NF"

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
// profiles.defaults.font.face and profiles.defaults.colorScheme and registers
// the scheme, and it touches nothing else. The first half of the function
// re-parses after every structural insert because the byte offsets shift; that
// keeps each edit anchored to the tree it was read from.
func windowsTerminalSettingsContent(src []byte, scheme windowsTerminalScheme) ([]byte, error) {
	if _, err := jsoncRootObject(src); err != nil {
		return nil, err
	}
	if _, err := jsoncDefaults(src); err != nil {
		return nil, err
	}

	updated, err := jsoncEnsureFontFace(src)
	if err != nil {
		return nil, err
	}
	updated, err = jsoncSetDefaultsColorScheme(updated, scheme.Name)
	if err != nil {
		return nil, err
	}
	return jsoncEnsureScheme(updated, scheme)
}

// jsoncEnsureFontFace sets profiles.defaults.font.face, creating the font object
// first when the defaults object carries none.
func jsoncEnsureFontFace(src []byte) ([]byte, error) {
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
	return jsoncSetString(src, font.value, "face", windowsTerminalFontFace)
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
func readWindowsTerminalOffer(profileDir string, def themeDefinition) windowsTerminalOffer {
	offer := windowsTerminalOffer{Face: windowsTerminalFontFace}

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
	updated, err := windowsTerminalSettingsContent(original, scheme)
	if err != nil {
		offer.Reason = fmt.Sprintf("%s was not written: %v", path, err)
		return offer
	}

	offer.Available = true
	offer.Original = original
	offer.updated = updated
	return offer
}

// describeWindowsTerminalOffer is what the offer shows before the user chooses:
// the file, the two settings, and what happens to the previous file. It is built
// from windowsTerminalSettings, so it cannot name a setting the writer does not
// set.
func describeWindowsTerminalOffer(offer windowsTerminalOffer) []string {
	if !offer.Available {
		return []string{offer.Reason}
	}
	labels := make([]string, 0, len(windowsTerminalSettings))
	for _, setting := range windowsTerminalSettings {
		labels = append(labels, setting.Label)
	}
	return []string{
		fmt.Sprintf("Windows Terminal's settings.json is reachable at %s.", offer.Path),
		fmt.Sprintf("Choosing to configure it would set %s there, registering the %s scheme in \"schemes\".", strings.Join(labels, " and "), offer.Scheme),
		"The previous file is copied beside it as .bak-dotfiles-<stamp> first, and every other key and comment is kept.",
	}
}

// applyWindowsTerminalOffer is the only path that writes. chosen is the explicit
// yes: without it nothing is written and no backup is made. With it, the previous
// bytes are copied beside the file before the write, and a backup that cannot be
// made aborts the write rather than leaving the user without one.
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

	labels := make([]string, 0, len(windowsTerminalSettings))
	for _, setting := range windowsTerminalSettings {
		labels = append(labels, setting.Label)
	}
	return fmt.Sprintf("Windows Terminal configured: %s were written to %s; the previous file is at %s.",
		strings.Join(labels, " and "), offer.Path, backup), true, nil
}
