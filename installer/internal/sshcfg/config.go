package sshcfg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ============================================================================
// READING THE USER'S CLIENT CONFIGURATION
// ============================================================================
//
// Everything in this file reads. LoadConfig turns the files the client would
// really read -- the user's own file, the system file, and every file an
// Include pulls in -- into a structure the interface can show, and Resolve asks
// the client itself what it would do.
//
// Two facts shape the whole file. The first is that a client configuration is
// not a list of records: an Include is a textual insertion, so a directive
// pulled in inside a `Host x` block belongs to that same stanza, and a `Match`
// block is read in the middle of the file. The second is that whatever this
// package does not understand must still survive an edit, so every directive is
// carried verbatim, with the file and line it came from, and nothing is ever
// re-ordered or normalised. An entry this reading cannot be sure about is
// reported as read-only with the reason instead of being guessed at.

// Option is one directive of a stanza, exactly as it was written.
type Option struct {
	Key   string
	Value string
	File  string
	Line  int
}

// Host is one `Host` stanza. Patterns is every pattern on the Host line, in
// order and verbatim; Alias is the first of them, or empty when the line had
// none. Options is every directive of the stanza, in file order.
//
// ReadOnly is empty when the entry may be edited safely. When it is not empty
// it is a sentence for a person, and the interface shows it verbatim as the
// reason the entry cannot be changed.
type Host struct {
	Alias    string
	Patterns []string
	File     string
	Line     int
	Managed  bool
	Options  []Option
	ReadOnly string
}

// ConfigFile is one file the client would read. Depth is 0 for the two
// top-level files (the user's own and the system one) and one more for each
// level of Include below them. Missing says the file did not exist, which is
// not an error: a fresh machine has no user configuration at all.
type ConfigFile struct {
	Path    string
	Depth   int
	Missing bool
	Err     string
}

// Config is what LoadConfig found. Failures holds one sentence per Include
// pattern that matched nothing, per file that could not be read and per shape
// that was too ambiguous to guess about; none of them stops the reading.
type Config struct {
	Files       []ConfigFile
	Hosts       []Host
	Failures    []string
	MatchBlocks int
}

// The kinds of block a client configuration is made of. Directives before the
// first Host or Match line are global; the resolver still has to see them,
// because `User` written at the top of the file applies to every host.
type blockKind int

const (
	blockGlobal blockKind = iota
	blockHost
	blockMatch
)

// matchRule is one criterion of a `Match` line. Evaluable is false for the
// criteria this package deliberately does not evaluate: guessing whether
// `exec` matches would run a command, so it is reported instead.
type matchRule struct {
	Keyword   string
	Value     string
	Evaluable bool
}

// configBlock is one run of directives with the context that decides who they
// apply to. The parser keeps every block in file order, with included files
// inlined where their Include appeared, because that order is the precedence
// the client uses.
type configBlock struct {
	kind            blockKind
	patterns        []string
	criteria        []matchRule
	options         []Option
	file            string
	line            int
	hostIndex       int
	managed         bool
	hasInclude      bool
	followedByMatch bool
	nonPlain        bool
}

// configModel is the parser's own result. Config is the public projection of
// it; the resolver needs the blocks and the Match criteria, which Config does
// not carry.
type configModel struct {
	files       []ConfigFile
	hosts       []Host
	failures    []string
	matchBlocks int
	blocks      []configBlock
}

// LoadConfig reads the user configuration first and the system one after it,
// honouring Include inline, and returns the stanzas in the order the client
// would read them.
func (m *Manager) LoadConfig(ctx context.Context) Config {
	model := m.parseModel()
	return Config{
		Files:       model.files,
		Hosts:       model.hosts,
		Failures:    model.failures,
		MatchBlocks: model.matchBlocks,
	}
}

// parseModel is the shared reading behind LoadConfig and Resolve.
func (m *Manager) parseModel() *configModel {
	p := &configParser{m: m, model: &configModel{blocks: []configBlock{{kind: blockGlobal, hostIndex: -1}}}, open: map[string]bool{}}
	p.addTop(m.Paths.UserConfig, m.Paths.UserConfigDir)
	system := m.Paths.SystemConfig
	if system != "" && system != m.Paths.UserConfig {
		p.addTop(system, filepath.Dir(system))
	}
	p.finish()
	return p.model
}

// configParser walks one or more files and builds a configModel. cur is the
// index of the block directives currently belong to, so an Include inside a
// `Host` block keeps contributing to that stanza.
type configParser struct {
	m         *Manager
	model     *configModel
	open      map[string]bool
	inManaged bool
	cur       int
}

// addTop reads one of the two top-level files. A missing top-level file is
// recorded as Missing and is not a failure: a machine that has never had the
// client configured has no user file, and that is what a fresh install sees.
// Reading a top-level file starts a fresh block context, because a Host block
// never spans two files.
func (p *configParser) addTop(path, base string) {
	p.cur = 0
	p.inManaged = false
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			p.model.files = append(p.model.files, ConfigFile{Path: path, Missing: true})
			return
		}
		p.model.files = append(p.model.files, ConfigFile{Path: path, Err: err.Error()})
		p.model.failures = append(p.model.failures, fmt.Sprintf("%s: cannot inspect it: %v", path, err))
		return
	}
	p.read(path, 0, base)
}

// read loads one file and parses it inline. The include stack catches a file
// that includes itself, directly or through another file: following it would
// never end, so it is recorded and skipped rather than looped on.
func (p *configParser) read(path string, depth int, base string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if p.open[abs] {
		p.model.failures = append(p.model.failures, fmt.Sprintf("%s: Include of %s was skipped because that file is already being read, so following it would loop", path, path))
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		p.model.files = append(p.model.files, ConfigFile{Path: path, Depth: depth, Err: err.Error()})
		p.model.failures = append(p.model.failures, fmt.Sprintf("%s: cannot read it: %v", path, err))
		return
	}
	p.model.files = append(p.model.files, ConfigFile{Path: path, Depth: depth})
	p.open[abs] = true
	defer delete(p.open, abs)
	p.parseContent(data, path, depth, base)
}

// parseContent reads a file line by line. Line numbers are 1-based, matching
// what an editor shows and what the interface prints.
func (p *configParser) parseContent(data []byte, path string, depth int, base string) {
	for i, raw := range splitFileLines(string(data)) {
		p.parseLine(raw, path, i+1, depth, base)
	}
}

// parseLine handles one line. Comments and blank lines are kept out of the
// model but two comment lines are structural: the markers of the block this
// tool owns. Everything between them is managed.
func (p *configParser) parseLine(line, path string, lineNo, depth int, base string) {
	line = strings.TrimRight(line, "\r")
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}
	if strings.HasPrefix(trimmed, "#") {
		switch trimmed {
		case ManagedBlockStart:
			p.inManaged = true
		case ManagedBlockEnd:
			p.inManaged = false
		}
		return
	}

	key, value := splitDirective(trimmed)
	if key == "" {
		return
	}

	switch strings.ToLower(key) {
	case "host":
		patterns := splitTokens(value)
		host := Host{
			Patterns: patterns,
			File:     path,
			Line:     lineNo,
			Managed:  p.inManaged,
		}
		if len(patterns) > 0 {
			host.Alias = patterns[0]
		}
		index := len(p.model.hosts)
		p.model.hosts = append(p.model.hosts, host)
		p.model.blocks = append(p.model.blocks, configBlock{
			kind:      blockHost,
			patterns:  patterns,
			file:      path,
			line:      lineNo,
			hostIndex: index,
			managed:   p.inManaged,
		})
		p.cur = len(p.model.blocks) - 1

	case "match":
		p.model.matchBlocks++
		// A Match line ends the Host block it follows. The directives after it
		// belong to the match, not to the stanza, and the stanza is ambiguous
		// enough that it is reported read-only instead of being guessed at.
		if p.cur >= 0 && p.model.blocks[p.cur].kind == blockHost {
			p.model.blocks[p.cur].followedByMatch = true
		}
		p.model.blocks = append(p.model.blocks, configBlock{
			kind:      blockMatch,
			criteria:  parseMatchRules(value),
			file:      path,
			line:      lineNo,
			hostIndex: -1,
			managed:   p.inManaged,
		})
		p.cur = len(p.model.blocks) - 1

	case "include":
		if p.cur >= 0 && p.model.blocks[p.cur].kind == blockHost {
			p.model.blocks[p.cur].hasInclude = true
		}
		for _, pattern := range splitTokens(value) {
			p.include(pattern, path, lineNo, depth, base)
		}

	default:
		option := Option{Key: key, Value: value, File: path, Line: lineNo}
		if optionValueIsAmbiguous(value) {
			if p.cur >= 0 && p.model.blocks[p.cur].kind == blockHost {
				p.model.blocks[p.cur].nonPlain = true
			}
		}
		if p.cur >= 0 {
			p.model.blocks[p.cur].options = append(p.model.blocks[p.cur].options, option)
		}
	}
}

// include resolves one Include pattern the way ssh_config(5) says. A relative
// pattern is resolved against `~/.ssh` for the user's file and `/etc/ssh` for
// the system one -- and an included file does not change that base, so a
// relative Include inside an included file still lands where the client would
// look for it. A pattern that matches nothing, or a file that cannot be read,
// is recorded and never fatal.
func (p *configParser) include(pattern, fromFile string, fromLine, depth int, base string) {
	if pattern == "" {
		return
	}
	expanded := expandTilde(pattern, p.m.Paths.Home)
	if !filepath.IsAbs(expanded) {
		expanded = filepath.Join(base, expanded)
	}
	matches, err := filepath.Glob(expanded)
	if err != nil {
		p.model.failures = append(p.model.failures, fmt.Sprintf("%s:%d: Include pattern %q is not valid: %v", fromFile, fromLine, pattern, err))
		return
	}
	if len(matches) == 0 {
		p.model.failures = append(p.model.failures, fmt.Sprintf("%s:%d: Include %q matched no file", fromFile, fromLine, pattern))
		return
	}
	for _, match := range matches {
		p.read(match, depth+1, base)
	}
}

// finish copies each host block's directives onto its Host and decides which
// entries may be edited. Deciding here rather than while parsing keeps the
// reason for every entry in one place.
func (p *configParser) finish() {
	for i := range p.model.blocks {
		block := &p.model.blocks[i]
		if block.kind != blockHost || block.hostIndex < 0 {
			continue
		}
		host := &p.model.hosts[block.hostIndex]
		host.Options = block.options
		host.ReadOnly = hostReadOnlyReason(host, block)
	}
}

// hostReadOnlyReason is the sentence the interface shows when an entry cannot
// be edited safely, or empty when it can. The checks run from the shape of the
// Host line outward: a pattern that covers more than one host is already
// enough, and a file this tool cannot write is the last word.
func hostReadOnlyReason(host *Host, block *configBlock) string {
	switch {
	case len(host.Patterns) == 0:
		return "the Host line has no pattern, so this tool has no alias to write back"
	case len(host.Patterns) > 1:
		return "the Host line lists more than one pattern, so editing it would change which hosts this entry covers"
	}
	for _, pattern := range host.Patterns {
		switch {
		case strings.HasPrefix(pattern, "!"):
			return "the Host pattern starts with a negation, so it is a rule about other hosts rather than one entry"
		case strings.ContainsAny(pattern, "*?"):
			return "the Host pattern contains a wildcard, so it matches more than this one entry"
		}
	}
	switch {
	case block.hasInclude:
		return "an Include line appears inside this entry, so its options also live in other files"
	case block.followedByMatch:
		return "a Match line follows this entry, so this tool cannot tell which options belong to it"
	case block.nonPlain:
		return "a line inside this entry is not a plain Key Value directive, so this tool will not rewrite it"
	case !fileWritable(host.File):
		return "the file this entry lives in is not writable by this tool"
	}
	return ""
}

// fileWritable reports whether the tool may write a configuration file. A file
// that does not exist yet is writable: it is the file a create would make.
func fileWritable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return errors.Is(err, os.ErrNotExist)
	}
	return info.Mode().Perm()&0o200 != 0
}

// ============================================================================
// THE EFFECTIVE CONFIGURATION
// ============================================================================

// Source says where one effective value came from. FromClient is always false
// in practice: when the client answers, it does not report provenance, so a
// Source is only filled when this package's own reading found the value.
type Source struct {
	Key        string
	Value      string
	File       string
	Line       int
	FromClient bool
}

// Effective is what the client would really do for one alias. When the client
// answered, FromClient is true and Raw is what it printed. When it could not be
// asked, the values come from this package's own reading and Warnings says so,
// together with any Match criterion that was deliberately not evaluated.
type Effective struct {
	Alias         string
	HostName      string
	User          string
	Port          string
	ProxyJump     string
	IdentityFiles []string
	Options       map[string]string
	Sources       map[string]Source
	FromClient    bool
	Raw           string
	Warnings      []string
}

// resolveOwn is this package's own reading, used as the fallback when the
// client is unavailable.
type resolveOwn struct {
	first       map[string]Source
	identity    []Source
	matched     bool
	unevaluated []string
}

// Resolve asks the client first and falls back to this package's own reading.
//
// The client is the authority on precedence, because the precedence between the
// user file, an included file and the system file is its rule and not this
// package's. The fallback applies the same rule for the subset it understands,
// and names what it did not evaluate rather than guessing.
func (m *Manager) Resolve(ctx context.Context, alias string) Effective {
	effective := Effective{
		Alias:   alias,
		Options: map[string]string{},
		Sources: map[string]Source{},
	}
	model := m.parseModel()
	own := m.resolveOwn(model, alias)

	var res Result
	if m.Run != nil {
		res = m.Run.Run(ctx, Command{Name: m.sshBinary(), Args: []string{"-G", "--", alias}})
	}
	if res.OK() {
		effective.FromClient = true
		effective.Raw = res.Stdout
		applyClientOutput(&effective, res.Stdout)
		// The client answered, so Sources stays empty for its keys: it does
		// not report where a value came from. When this package's own reading
		// agrees with a value, the Source is filled from that reading anyway,
		// which is the only provenance available. That asymmetry -- client
		// values without sources, agreeing values with this package's own --
		// is deliberate, not an oversight.
		for key, source := range own.first {
			if value, ok := effective.Options[key]; ok && value == source.Value {
				effective.Sources[key] = source
			}
		}
		return effective
	}

	effective.Warnings = append(effective.Warnings,
		"the client could not be asked, so these values come from this tool's own reading of the files: "+res.Message())
	effective.HostName = own.first["hostname"].Value
	effective.User = own.first["user"].Value
	effective.Port = own.first["port"].Value
	effective.ProxyJump = own.first["proxyjump"].Value
	for key, source := range own.first {
		effective.Options[key] = source.Value
		effective.Sources[key] = source
	}
	for _, source := range own.identity {
		effective.IdentityFiles = append(effective.IdentityFiles, source.Value)
	}
	if effective.HostName == "" {
		// No HostName anywhere means the client connects to the alias itself,
		// which is what it does for an alias no file mentions.
		effective.HostName = alias
	}
	if len(own.unevaluated) > 0 {
		effective.Warnings = append(effective.Warnings,
			"this tool does not evaluate these Match criteria, so a block using them was ignored: "+strings.Join(own.unevaluated, ", "))
	}
	if !own.matched {
		effective.Warnings = append(effective.Warnings,
			fmt.Sprintf("no configuration mentions %q, so the client connects to the name itself", alias))
	}
	return effective
}

// sshBinary is the client this manager runs. It prefers the path the probe
// found and falls back to the name, so a Manager built by hand still works.
func (m *Manager) sshBinary() string {
	if m.Caps.SSH != "" {
		return m.Caps.SSH
	}
	return "ssh"
}

// applyClientOutput parses `ssh -G` output: one `key value` per line, keys
// lower-cased. IdentityFile is the one key the client prints once per file, so
// it is collected in order instead of overwritten, which is what lets a host
// with several identities keep all of them.
func applyClientOutput(effective *Effective, output string) {
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" {
			continue
		}
		key, value := splitDirective(line)
		if key == "" {
			continue
		}
		lower := strings.ToLower(key)
		if lower == "identityfile" {
			effective.IdentityFiles = append(effective.IdentityFiles, value)
			if _, ok := effective.Options[lower]; !ok {
				effective.Options[lower] = value
			}
			continue
		}
		if _, ok := effective.Options[lower]; !ok {
			effective.Options[lower] = value
		}
		switch lower {
		case "hostname":
			effective.HostName = value
		case "user":
			effective.User = value
		case "port":
			effective.Port = value
		case "proxyjump":
			effective.ProxyJump = value
		}
	}
}

// resolveOwn walks every block in file order -- the user file before the
// system one, includes inlined where they appeared -- and takes the first value
// each key was given, which is the client's own rule. IdentityFile is the
// exception: the client accumulates it, so every matching IdentityFile is kept
// in order.
func (m *Manager) resolveOwn(model *configModel, alias string) *resolveOwn {
	own := &resolveOwn{first: map[string]Source{}}
	for i := range model.blocks {
		block := &model.blocks[i]
		switch block.kind {
		case blockHost:
			if !hostPatternMatches(block.patterns, alias) {
				continue
			}
		case blockMatch:
			applies, unevaluated := matchApplies(block.criteria, alias)
			own.unevaluated = append(own.unevaluated, unevaluated...)
			if !applies {
				continue
			}
		}
		if block.kind != blockGlobal {
			own.matched = true
		}
		for _, option := range block.options {
			key := strings.ToLower(option.Key)
			value := optionResolvedValue(option.Value)
			source := Source{Key: key, Value: value, File: option.File, Line: option.Line}
			if key == "identityfile" {
				own.identity = append(own.identity, source)
				if _, ok := own.first[key]; !ok {
					own.first[key] = source
				}
				continue
			}
			if _, ok := own.first[key]; ok {
				continue
			}
			own.first[key] = source
		}
	}
	return own
}

// matchApplies evaluates a Match line's criteria. Only `all` and `host` are
// evaluated; the rest -- exec would run a command, and the others depend on
// state this reading does not have -- are named as not evaluated and the block
// is skipped rather than guessed at.
func matchApplies(rules []matchRule, alias string) (bool, []string) {
	var unevaluated []string
	applies := true
	for _, rule := range rules {
		if !rule.Evaluable {
			unevaluated = append(unevaluated, rule.Keyword)
			applies = false
			continue
		}
		switch rule.Keyword {
		case "all":
			// always matches
		case "host":
			if !hostPatternMatches(matchHostPatterns(rule.Value), alias) {
				applies = false
			}
		default:
			applies = false
		}
	}
	if len(unevaluated) > 0 {
		applies = false
	}
	return applies, unevaluated
}

// hostPatternMatches applies one Host line's patterns to an alias. A negated
// pattern that matches excludes the host immediately; otherwise any positive
// match counts, because the patterns on one line are alternatives.
func hostPatternMatches(patterns []string, alias string) bool {
	matched := false
	for _, pattern := range patterns {
		negated := strings.HasPrefix(pattern, "!")
		name := strings.TrimPrefix(pattern, "!")
		if !wildcardMatch(name, alias) {
			continue
		}
		if negated {
			return false
		}
		matched = true
	}
	return matched
}

// wildcardMatch is ssh's own `*` and `?` matching, case-insensitive. It is
// used both for Host lines and for Match host patterns.
func wildcardMatch(pattern, name string) bool {
	p := []rune(strings.ToLower(pattern))
	s := []rune(strings.ToLower(name))
	pi, si := 0, 0
	star, mark := -1, 0
	for si < len(s) {
		switch {
		case pi < len(p) && (p[pi] == '?' || p[pi] == s[si]):
			pi++
			si++
		case pi < len(p) && p[pi] == '*':
			star = pi
			mark = si
			pi++
		case star >= 0:
			pi = star + 1
			mark++
			si = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

// parseMatchRules reads a Match line into criteria. A value token that is
// itself a criterion keyword starts the next one, so `exec "cmd" host x` is
// read as two.
func parseMatchRules(value string) []matchRule {
	tokens := splitTokens(value)
	var rules []matchRule
	for i := 0; i < len(tokens); {
		keyword := strings.ToLower(tokens[i])
		i++
		switch keyword {
		case "all":
			rules = append(rules, matchRule{Keyword: "all", Evaluable: true})
		case "host":
			var values []string
			for i < len(tokens) && !isMatchKeyword(tokens[i]) {
				values = append(values, tokens[i])
				i++
			}
			rules = append(rules, matchRule{Keyword: "host", Value: strings.Join(values, " "), Evaluable: true})
		default:
			argument := ""
			if i < len(tokens) && !isMatchKeyword(tokens[i]) {
				argument = tokens[i]
				i++
			}
			rules = append(rules, matchRule{Keyword: keyword, Value: argument})
		}
	}
	return rules
}

// matchKeywords is every criterion a Match line may name. The parser needs the
// whole set to know where one criterion's argument ends and the next begins.
var matchKeywords = map[string]bool{
	"all": true, "canonical": true, "exec": true, "final": true,
	"host": true, "localnetwork": true, "localuser": true,
	"originalhost": true, "tagged": true, "user": true,
}

func isMatchKeyword(token string) bool {
	return matchKeywords[strings.ToLower(token)]
}

// matchHostPatterns splits the comma- or space-separated list a Match host
// criterion may carry.
func matchHostPatterns(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}

// ============================================================================
// SOURCE-TEXT HELPERS
// ============================================================================

// splitDirective splits one line into a keyword and its value. Both forms the
// client accepts are handled -- `Key value` and `Key=value` -- and the value is
// returned verbatim so that unknown options survive untouched.
func splitDirective(line string) (string, string) {
	space := strings.IndexAny(line, " \t")
	equals := strings.IndexByte(line, '=')
	if equals >= 0 && (space < 0 || equals < space) {
		return strings.TrimSpace(line[:equals]), strings.TrimSpace(line[equals+1:])
	}
	if space < 0 {
		return line, ""
	}
	return line[:space], strings.TrimSpace(line[space+1:])
}

// splitTokens splits a value into whitespace-separated tokens, keeping a double
// quoted token together and dropping the quotes. It is what makes a path with a
// space survive.
func splitTokens(value string) []string {
	var tokens []string
	var current strings.Builder
	quoted := false
	for _, r := range value {
		switch {
		case r == '"':
			quoted = !quoted
		case unicode.IsSpace(r) && !quoted:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// expandTilde turns `~` and `~/...` into the home directory the client itself
// would use. Anything else is returned unchanged.
func expandTilde(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// optionValueIsAmbiguous reports whether a directive's value cannot be rewritten
// safely. A bare keyword and a trailing `#` comment both mean the line is not a
// plain `Key Value` directive, so the stanza is reported read-only instead of
// having the comment silently dropped by a rewrite.
func optionValueIsAmbiguous(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	return hasUnquotedComment(value)
}

// hasUnquotedComment reports whether a `#` outside quotes makes the rest of the
// value a comment. A `#` inside quotes is part of the value.
func hasUnquotedComment(value string) bool {
	quoted := false
	for _, r := range value {
		switch {
		case r == '"':
			quoted = !quoted
		case r == '#' && !quoted:
			return true
		}
	}
	return false
}

// stripInlineComment removes a trailing `#` comment, leaving a quoted `#`
// alone. It is used when resolving a value, not when carrying it.
func stripInlineComment(value string) string {
	quoted := false
	for i, r := range value {
		switch {
		case r == '"':
			quoted = !quoted
		case r == '#' && !quoted:
			return strings.TrimSpace(value[:i])
		}
	}
	return strings.TrimSpace(value)
}

// optionResolvedValue is the value this package resolves with: the trailing
// comment removed and surrounding quotes undone. The Option itself keeps the
// verbatim text.
func optionResolvedValue(value string) string {
	return unquoteValue(stripInlineComment(value))
}

// unquoteValue removes one pair of surrounding double quotes and the escapes
// the client allows inside them.
func unquoteValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return value
	}
	inner := value[1 : len(value)-1]
	inner = strings.ReplaceAll(inner, `\"`, `"`)
	inner = strings.ReplaceAll(inner, `\\`, `\`)
	return inner
}

// splitFileLines splits a file into lines without losing the shape: a trailing
// newline leaves a final empty element, so joining the result reproduces the
// original bytes exactly. That round trip is what lets every writer touch only
// the lines it means to and leave the rest byte-for-byte alone.
func splitFileLines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

// joinFileLines is the inverse of splitFileLines.
func joinFileLines(lines []string) string {
	return strings.Join(lines, "\n")
}
