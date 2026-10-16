package sshcfg

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ============================================================================
// KEYS: WHAT EXISTS, WITHOUT EVER READING A PRIVATE KEY
// ============================================================================
//
// This file lists key pairs and known hosts, and generates new keys. One rule
// shapes every line of it: a private key file is never opened, parsed, copied,
// hashed or logged. What this file may do with a private path is
//
//   - stat it, for existence and mode;
//   - ask ssh-keygen for its fingerprint (`ssh-keygen -l`) or for proof that it
//     is not encrypted (`ssh-keygen -y`);
//   - hand a passphrase to ssh-keygen over the standard-input pipe, never as an
//     argument and never in the environment.
//
// Everything else -- the public half, the fingerprint of a `.pub`, the bit
// length of an RSA modulus -- is computed from public material by this package.

const (
	// keyMaxPublicFileSize bounds a public half this package will read. A `.pub`
	// is a few hundred bytes, so anything past this bound is reported as a
	// warning instead of being loaded into memory.
	keyMaxPublicFileSize = 64 * 1024

	// keyDefaultType, keyDefaultRounds and the per-type bit defaults are what
	// ssh-keygen would choose when the caller states nothing.
	keyDefaultType      = "ed25519"
	keyDefaultRounds    = 100
	keyDefaultRSABits   = 4096
	keyDefaultECDSABits = 521

	// keyAskpassNonexistent is the path SSH_ASKPASS is pointed at. It does not
	// exist on purpose: a child process that would otherwise look for a graphical
	// prompter finds nothing and must read the passphrase from the pipe.
	keyAskpassNonexistent = "dotfiles-sshcfg-askpass-does-not-exist"
)

// keyNonKeyNames are the client's own files in ~/.ssh. OpenSSH offers no way to
// tell a private key from `config` without reading bytes, which this program
// never does, so the files that are known by name not to be keys are named here
// and left out of the inventory.
var keyNonKeyNames = map[string]bool{
	"config":           true,
	"ssh_config":       true,
	"known_hosts":      true,
	"known_hosts.old":  true,
	"authorized_keys":  true,
	"authorized_keys2": true,
	"environment":      true,
	"rc":               true,
	"agent.env":        true,
}

// Key is one key pair as far as this package may look. Path is the private side
// and may not exist; every field that describes the key is public material.
type Key struct {
	// Path is the private-side path; it may or may not exist.
	Path string
	// PubPath is empty when no public half was found.
	PubPath string
	// Type is a wire type such as "ssh-ed25519" or "ssh-rsa", empty when the
	// type could not be learned without reading the private key.
	Type string
	// Bits is 0 when the type has no bit count.
	Bits int
	// Fingerprint is "SHA256:<base64 without padding>" of the key blob.
	Fingerprint string
	// Comment is the key's own comment.
	Comment string
	// Public is the complete public line, public material only.
	Public string
	// PublicKnown reports whether a public half was found and parsed.
	PublicKnown bool
	// Mode is the private half's mode, 0 when there is no private half.
	Mode os.FileMode
	// PermissionsOK reports whether the private half is private enough for ssh.
	PermissionsOK bool
	// Warnings is what could not be learned, in plain prose.
	Warnings []string
	// Encrypted is "", "yes" or "unknown". It is never "no" unless a
	// `ssh-keygen -y` run without a passphrase proved it.
	Encrypted string
}

// ListKeys inventories ~/.ssh one level deep.
//
// It is deliberately not recursive: keys live directly under ~/.ssh, and a walk
// would list backups and unrelated material. An unreadable entry is reported as
// a Key that carries the warning; only a directory that cannot be read at all is
// an error, because then the inventory would be a lie.
func (m *Manager) ListKeys(ctx context.Context) ([]Key, error) {
	entries, err := os.ReadDir(m.Paths.SSHDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read the ssh directory %s: %w", m.Paths.SSHDir, err)
	}
	present := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			present[entry.Name()] = true
		}
	}

	keys := make([]Key, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(m.Paths.SSHDir, name)
		if strings.HasSuffix(name, ".pub") {
			keys = append(keys, m.listPublicHalf(path))
			continue
		}
		if present[name+".pub"] {
			// Already reported through its public half.
			continue
		}
		if keyNonKeyNames[strings.ToLower(name)] {
			// The client's own bookkeeping, not a key.
			continue
		}
		keys = append(keys, m.listPrivateOnly(ctx, path))
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i].Path < keys[j].Path })
	return keys, nil
}

// listPublicHalf describes a `.pub` file and stats the private side beside it.
// The private bytes are never opened: only `os.Stat` touches that path.
func (m *Manager) listPublicHalf(pubPath string) Key {
	key := Key{Path: strings.TrimSuffix(pubPath, ".pub"), PubPath: pubPath}
	if info, err := os.Stat(key.Path); err == nil {
		key.Mode = info.Mode().Perm()
		key.PermissionsOK = keyPrivatePermsOK(key.Mode)
		key.Encrypted = "unknown"
	} else {
		// A public half whose private half is absent is still a real,
		// shareable key: report it and say why the mode is unknown.
		key.PermissionsOK = true
		key.Warnings = append(key.Warnings, fmt.Sprintf("the private half %s is absent; only the public half was found", key.Path))
	}

	data, oversized, err := keyReadSmall(pubPath)
	switch {
	case err != nil:
		key.Warnings = append(key.Warnings, fmt.Sprintf("the public half could not be read: %v", err))
		return key
	case oversized:
		key.Warnings = append(key.Warnings, fmt.Sprintf("the public half %s is larger than %d bytes and was not loaded", pubPath, keyMaxPublicFileSize))
		return key
	}

	line := firstLine(string(data))
	if line == "" {
		key.Warnings = append(key.Warnings, "the public half is empty")
		return key
	}
	keyType, _, comment, blob, parseErr := keyParsePublicLine(line)
	if parseErr != nil {
		key.Warnings = append(key.Warnings, fmt.Sprintf("the public half does not parse: %v", parseErr))
		return key
	}
	key.Type = keyType
	key.Comment = comment
	key.Public = line
	key.PublicKnown = true
	key.Fingerprint = keyFingerprint(blob)
	key.Bits = keyBits(keyType, blob)
	return key
}

// listPrivateOnly describes a private key that has no `.pub` beside it. The only
// source of type, bits, comment and fingerprint is `ssh-keygen -l -f <path>`;
// when that fails the key is still reported, with the tool's own line as the
// warning and nothing invented from the file's bytes.
func (m *Manager) listPrivateOnly(ctx context.Context, path string) Key {
	key := Key{Path: path}
	if info, err := os.Stat(path); err == nil {
		key.Mode = info.Mode().Perm()
		key.PermissionsOK = keyPrivatePermsOK(key.Mode)
	}

	res := m.keygen(ctx, "", 0, "-l", "-f", path)
	if !res.OK() {
		key.Encrypted = "unknown"
		message := keyMessage(res)
		if keyMentionsPassphrase(message) {
			// ssh-keygen recognised a key and asked for a passphrase it was
			// never given: that is the one reliable signal that it is encrypted.
			key.Encrypted = "yes"
		}
		key.Warnings = append(key.Warnings, message)
		return key
	}

	bits, fingerprint, comment, humanType := keyParseFingerprintLine(res.Output())
	key.Bits = bits
	key.Fingerprint = fingerprint
	key.Comment = comment
	key.Type = keyWireType(humanType, bits)

	// Prove the key is unencrypted before saying so. `ssh-keygen -y` needs no
	// passphrase for a key that has none; for one that does it fails rather than
	// prompting, because the command is detached from any terminal.
	proof := m.keygen(ctx, "", 0, "-y", "-f", path)
	if proof.OK() && strings.TrimSpace(proof.Output()) != "" {
		key.Encrypted = "no"
	} else {
		key.Encrypted = "unknown"
	}
	return key
}

// keyReadSmall reads at most keyMaxPublicFileSize bytes and reports when the
// file is larger, so a runaway `.pub` cannot be loaded whole.
func keyReadSmall(path string) (data []byte, oversized bool, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	data, err = io.ReadAll(io.LimitReader(file, keyMaxPublicFileSize+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > keyMaxPublicFileSize {
		return nil, true, nil
	}
	return data, false, nil
}

// ============================================================================
// PUBLIC KEY MATERIAL, COMPUTED HERE
// ============================================================================

// keyParsePublicLine splits `<type> <base64> [comment]` and decodes the blob.
// The comment keeps everything after the blob, spaces included.
func keyParsePublicLine(line string) (keyType, blobBase64, comment string, blob []byte, err error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", "", "", nil, errors.New("the line has no key type and blob")
	}
	keyType = fields[0]
	blobBase64 = fields[1]
	blob, err = base64.StdEncoding.DecodeString(blobBase64)
	if err != nil {
		return keyType, blobBase64, keyCommentAfter(line, blobBase64), nil, fmt.Errorf("the blob is not valid base64: %w", err)
	}
	return keyType, blobBase64, keyCommentAfter(line, blobBase64), blob, nil
}

// keyCommentAfter returns what follows the blob token on the line.
func keyCommentAfter(line, token string) string {
	at := strings.Index(line, token)
	if at < 0 {
		return ""
	}
	return strings.TrimSpace(line[at+len(token):])
}

// keyFingerprint is the OpenSSH fingerprint of a decoded key blob: "SHA256:"
// followed by the standard base64 of the SHA-256 digest, without padding. It is
// the same string `ssh-keygen -l` prints, produced here so a public half can be
// described without running the tool.
func keyFingerprint(blob []byte) string {
	sum := sha256.Sum256(blob)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// keyBits reads the key size: the RSA modulus' bit length from the blob, and the
// fixed size of every other supported type.
func keyBits(keyType string, blob []byte) int {
	switch keyType {
	case "ssh-rsa", "rsa-sha2-256", "rsa-sha2-512":
		// The RSA blob is `string "ssh-rsa"`, then two mpints: the public
		// exponent and the modulus. The modulus' bit length is the key size.
		_, offset, ok := keySSHString(blob, 0)
		if !ok {
			return 0
		}
		_, offset, ok = keySSHString(blob, offset)
		if !ok {
			return 0
		}
		modulus, _, ok := keySSHString(blob, offset)
		if !ok {
			return 0
		}
		return keyMPIntBits(modulus)
	case "ssh-ed25519", "sk-ssh-ed25519@openssh.com":
		return 256
	case "ecdsa-sha2-nistp256", "sk-ecdsa-sha2-nistp256@openssh.com":
		return 256
	case "ecdsa-sha2-nistp384":
		return 384
	case "ecdsa-sha2-nistp521":
		return 521
	}
	return 0
}

// keySSHString reads one length-prefixed field from an SSH wire-format blob.
func keySSHString(blob []byte, offset int) ([]byte, int, bool) {
	if offset < 0 || offset+4 > len(blob) {
		return nil, offset, false
	}
	length := int(binary.BigEndian.Uint32(blob[offset:]))
	offset += 4
	if length < 0 || offset+length > len(blob) {
		return nil, offset, false
	}
	return blob[offset : offset+length], offset + length, true
}

// keyMPIntBits is the bit length of a signed big-endian integer, ignoring the
// leading zero byte an mpint carries to keep its sign.
func keyMPIntBits(value []byte) int {
	for len(value) > 0 && value[0] == 0 {
		value = value[1:]
	}
	if len(value) == 0 {
		return 0
	}
	bits := (len(value) - 1) * 8
	for top := value[0]; top > 0; top >>= 1 {
		bits++
	}
	return bits
}

// keyParseFingerprintLine reads the `ssh-keygen -l` and `ssh-add -l` shape:
// `<bits> SHA256:<fingerprint> [comment] (<TYPE>)`.
func keyParseFingerprintLine(line string) (bits int, fingerprint, comment, humanType string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, "", "", ""
	}
	if open := strings.LastIndex(line, "("); open >= 0 && strings.HasSuffix(line, ")") {
		candidate := strings.TrimSpace(line[open+1 : len(line)-1])
		if candidate != "" && !strings.ContainsAny(candidate, " \t") {
			humanType = candidate
			line = strings.TrimSpace(line[:open])
		}
	}
	fields := strings.SplitN(line, " ", 3)
	if len(fields) >= 1 {
		bits, _ = strconv.Atoi(strings.TrimSpace(fields[0]))
	}
	if len(fields) >= 2 {
		fingerprint = fields[1]
	}
	if len(fields) >= 3 {
		comment = strings.TrimSpace(fields[2])
	}
	return bits, fingerprint, comment, humanType
}

// keyWireType turns the type `ssh-keygen -l` prints in parentheses ("ED25519",
// "RSA", "ECDSA", ...) into the wire name a public line uses. The bit count
// settles which ECDSA curve the parenthesised name does not spell out. A value
// that is already a wire name is passed through unchanged.
func keyWireType(humanType string, bits int) string {
	human := strings.ToLower(strings.TrimSpace(humanType))
	switch human {
	case "":
		return ""
	case "rsa":
		return "ssh-rsa"
	case "dsa":
		return "ssh-dss"
	case "ed25519":
		return "ssh-ed25519"
	case "ed25519-sk":
		return "sk-ssh-ed25519@openssh.com"
	case "ecdsa":
		switch bits {
		case 384:
			return "ecdsa-sha2-nistp384"
		case 521:
			return "ecdsa-sha2-nistp521"
		default:
			return "ecdsa-sha2-nistp256"
		}
	case "ecdsa-sk":
		return "sk-ecdsa-sha2-nistp256@openssh.com"
	}
	if strings.HasPrefix(human, "ssh-") || strings.HasPrefix(human, "ecdsa-") || strings.Contains(human, "@") {
		return humanType
	}
	return humanType
}

// ============================================================================
// PERMISSIONS
// ============================================================================

// keyPrivatePermsOK encodes the rule ssh itself enforces on a private key: no
// group or other access at all, neither read nor write nor execute. The check is
// the same mask for both, so 0600 and 0400 pass and 0644 does not.
func keyPrivatePermsOK(mode os.FileMode) bool { return mode.Perm()&0o077 == 0 }

// keyDirPermsOK encodes the rule ssh enforces on ~/.ssh. It is narrower than the
// key's rule on purpose: a directory must not be writable by group or other, but
// it may be readable and executable by them (the usual 0755 home directory).
func keyDirPermsOK(mode os.FileMode) bool { return mode.Perm()&0o022 == 0 }

// ============================================================================
// KNOWN HOSTS
// ============================================================================

// KnownHost is one line of known_hosts.
type KnownHost struct {
	// Hosts is the spelled-out host list; empty when the entry is hashed.
	Hosts []string
	// Hashed reports an entry whose host is stored as a keyed hash.
	Hashed bool
	// Markers is "@cert-authority" and "@revoked", in the order written.
	Markers []string
	// Type is the host key's wire type.
	Type string
	// Fingerprint is "SHA256:<base64 without padding>" of the host key blob.
	Fingerprint string
	// File and Line locate the entry.
	File string
	Line int
	// Warnings is what could not be parsed, in plain prose.
	Warnings []string
}

// ListKnownHosts parses the user's known_hosts. A missing file is not an error:
// it means the client has not recorded any host yet. A malformed line is
// reported as a KnownHost with a warning, never dropped and never fatal.
//
// known_hosts.old is deliberately never read. It is a backup of a previous
// revision, so listing it would claim a trust the client is not using.
func (m *Manager) ListKnownHosts(ctx context.Context) ([]KnownHost, error) {
	lines, err := readKnownHostsFile(m.Paths.KnownHosts)
	if err != nil {
		return nil, err
	}
	return m.parseKnownHostLines(lines), nil
}

// LookupKnownHost asks ssh-keygen which entry covers a host. `-F` is the only
// reliable way to match a hashed entry, because reversing the hash is exactly
// what the hash is there to prevent. A host with no entry returns an empty list
// and no error.
func (m *Manager) LookupKnownHost(ctx context.Context, host string) ([]KnownHost, error) {
	if host == "" || m.Caps.Keygen == "" {
		return nil, nil
	}
	res := m.keygen(ctx, "", 0, "-F", host, "-f", m.Paths.KnownHosts)
	if !res.OK() {
		return nil, nil
	}
	fileLines, _ := readKnownHostsFile(m.Paths.KnownHosts)
	var found []KnownHost
	for _, line := range strings.Split(res.Stdout, "\n") {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		_, _, _, blob, _, ok := knownHostSplit(trimmed)
		if !ok {
			continue
		}
		lineNumber := knownHostLineNumber(fileLines, blob)
		found = append(found, m.parseKnownHostLine(trimmed, m.Paths.KnownHosts, lineNumber))
	}
	return found, nil
}

// parseKnownHostLines reports every entry line. Blank lines and comments are not
// entries and are skipped; anything else is reported, valid or not.
func (m *Manager) parseKnownHostLines(lines []string) []KnownHost {
	hosts := make([]KnownHost, 0, len(lines))
	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		hosts = append(hosts, m.parseKnownHostLine(trimmed, m.Paths.KnownHosts, i+1))
	}
	return hosts
}

// parseKnownHostLine reads one entry. A line that does not parse is returned
// with a warning rather than an error, so one bad line cannot hide the rest.
func (m *Manager) parseKnownHostLine(line, file string, lineNumber int) KnownHost {
	kh := KnownHost{File: file, Line: lineNumber}
	markers, host, keyType, blobString, _, ok := knownHostSplit(line)
	kh.Markers = markers
	if !ok {
		kh.Warnings = append(kh.Warnings, "this line is not a known_hosts entry")
		return kh
	}
	if strings.HasPrefix(host, "|") {
		// A hashed host is reported as hashed and never reversed.
		kh.Hashed = true
	} else {
		kh.Hosts = strings.Split(host, ",")
	}
	kh.Type = keyType
	blob, err := base64.StdEncoding.DecodeString(blobString)
	if err != nil {
		kh.Warnings = append(kh.Warnings, "the host key blob is not valid base64")
		return kh
	}
	kh.Fingerprint = keyFingerprint(blob)
	return kh
}

// knownHostSplit breaks a known_hosts line into markers, the host field, the key
// type, the blob and the comment. It understands the `[host]:port` form because
// it is just another host field, and it never tries to interpret a hashed host.
func knownHostSplit(line string) (markers []string, host, keyType, blob, comment string, ok bool) {
	fields := strings.Fields(line)
	at := 0
	for at < len(fields) && strings.HasPrefix(fields[at], "@") {
		markers = append(markers, fields[at])
		at++
	}
	if at+3 > len(fields) {
		return markers, "", "", "", "", false
	}
	host = fields[at]
	keyType = fields[at+1]
	blob = fields[at+2]
	comment = strings.Join(fields[at+3:], " ")
	return markers, host, keyType, blob, comment, true
}

// readKnownHostsFile returns the file's lines. A missing file is an empty list
// and no error: the client simply has not recorded a host yet.
func readKnownHostsFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	return strings.Split(text, "\n"), nil
}

// knownHostLineNumber locates an entry in the file by its blob, which is the one
// part of the line that is unique and stable.
func knownHostLineNumber(lines []string, blob string) int {
	if blob == "" {
		return 0
	}
	for i, line := range lines {
		if strings.Contains(line, blob) {
			return i + 1
		}
	}
	return 0
}

// ============================================================================
// GENERATION
// ============================================================================

// KeygenRequest is one generation request. Passphrase is a secret and is never
// logged, never an argument and never an environment value.
type KeygenRequest struct {
	// Path is where the pair is written; a leading "~" is expanded by the
	// manager.
	Path string
	// Type is the key type; empty means "ed25519".
	Type string
	// Bits is the key size; 0 means the recommended default for the type.
	Bits int
	// Comment is the key's comment.
	Comment string
	// Passphrase is the passphrase for the new private key.
	Passphrase string
	// Rounds is the number of KDF rounds (ssh-keygen -a); 0 means the default.
	Rounds int
	// AllowReplace allows writing over an existing pair. When false, an existing
	// file is refused and nothing is written.
	AllowReplace bool
}

// GenerateResult is the outcome of one generation.
type GenerateResult struct {
	// Key is the generated pair, described from its public half.
	Key Key
	// Args is the argument list actually used, safe to log: it never carries the
	// passphrase.
	Args []string
	// PassphraseConfirmed reports that the passphrase was proven to open the new
	// key.
	PassphraseConfirmed bool
	// Warnings is what could not be proven, in plain prose.
	Warnings []string
}

// GenerateKey creates a key pair with ssh-keygen.
//
// The passphrase travels on the child's standard input only. A comment in
// keygen explains why the child is detached and why it is told there is no
// graphical prompter, so it cannot silently accept an empty passphrase.
func (m *Manager) GenerateKey(ctx context.Context, req KeygenRequest) (GenerateResult, error) {
	path, err := m.keygenPath(req.Path)
	if err != nil {
		return GenerateResult{}, err
	}
	if m.Caps.Keygen == "" {
		return GenerateResult{}, errors.New("ssh-keygen was not found, so no key can be generated")
	}

	keyType := req.Type
	if keyType == "" {
		keyType = keyDefaultType
	}
	if len(m.Caps.KeygenSupports) > 0 && !keyTypeSupported(m.Caps.KeygenSupports, keyType) {
		return GenerateResult{}, fmt.Errorf("this ssh-keygen cannot generate %q; it supports: %s", keyType, strings.Join(m.Caps.KeygenSupports, ", "))
	}

	rounds := req.Rounds
	if rounds <= 0 {
		rounds = keyDefaultRounds
	}
	bits := req.Bits
	if bits <= 0 {
		switch keyType {
		case "rsa":
			bits = keyDefaultRSABits
		case "ecdsa":
			bits = keyDefaultECDSABits
		case "sk-ecdsa-sha2-nistp256@openssh.com":
			bits = 256
		}
	}

	if req.AllowReplace {
		// The switch is the user saying this pair may be destroyed, and it has to be
		// honoured here: ssh-keygen refuses to overwrite a key that is still present and
		// asks "already exists. Overwrite (y/n)?" on a terminal this interface owns, so a
		// request that reached it with the old pair in place failed with a message about
		// a prompt nobody saw. Only the two files this request names are removed, and a
		// removal that fails stops the generation: the alternative is leaving the user
		// with an error they cannot act on.
		for _, existing := range []string{path, path + ".pub"} {
			if err := os.Remove(existing); err != nil && !errors.Is(err, os.ErrNotExist) {
				return GenerateResult{}, fmt.Errorf("the existing key %s could not be removed, so nothing was generated: %w", existing, err)
			}
		}
	} else {
		for _, existing := range []string{path, path + ".pub"} {
			if _, statErr := os.Stat(existing); statErr == nil {
				return GenerateResult{}, fmt.Errorf("the key %s already exists; nothing was written. Remove it or allow replacement", existing)
			}
		}
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if _, statErr := os.Stat(dir); errors.Is(statErr, os.ErrNotExist) {
			if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
				return GenerateResult{}, fmt.Errorf("cannot create %s: %w", dir, mkErr)
			}
		}
	}

	args := []string{"-q", "-t", keyType}
	if bits > 0 {
		args = append(args, "-b", strconv.Itoa(bits))
	}
	args = append(args, "-a", strconv.Itoa(rounds), "-f", path, "-C", req.Comment)

	if res := m.keygen(ctx, req.Passphrase, 2, args...); !res.OK() {
		return GenerateResult{Args: args}, fmt.Errorf("ssh-keygen did not create the key: %s", keyMessage(res))
	}

	// ssh-keygen honours umask, so the modes are set explicitly. A private key
	// ssh would refuse to use is a security bug, not a cosmetic one.
	if chmodErr := os.Chmod(path, 0o600); chmodErr != nil {
		return GenerateResult{Args: args}, fmt.Errorf("cannot set the private key permissions: %w", chmodErr)
	}
	if chmodErr := os.Chmod(path+".pub", 0o644); chmodErr != nil {
		return GenerateResult{Args: args}, fmt.Errorf("cannot set the public key permissions: %w", chmodErr)
	}

	result := GenerateResult{Key: m.listPublicHalf(path + ".pub"), Args: args}
	if req.Passphrase != "" {
		// Prove the passphrase was applied instead of assuming it. The proof is
		// another read through the pipe; the passphrase is never an argument.
		proof := m.keygen(ctx, req.Passphrase, 1, "-y", "-f", path)
		if !proof.OK() {
			message := keyMessage(proof)
			warning := "the passphrase may not have been applied: " + message
			result.Key.Warnings = append(result.Key.Warnings, warning)
			result.Warnings = append(result.Warnings, warning)
			return result, fmt.Errorf("the passphrase could not be confirmed: %s", message)
		}
		result.PassphraseConfirmed = true
		result.Key.Encrypted = "yes"
		return result, nil
	}

	// Without a passphrase, say "no" only after `ssh-keygen -y` accepted the key
	// with nothing on the pipe.
	proof := m.keygen(ctx, "", 0, "-y", "-f", path)
	if proof.OK() && strings.TrimSpace(proof.Output()) != "" {
		result.Key.Encrypted = "no"
	} else {
		result.Key.Encrypted = "unknown"
	}
	return result, nil
}

// keygenPath expands a leading "~" and rejects an empty path. The caller is
// expected to expand it already; this is the safe fallback.
func (m *Manager) keygenPath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	switch {
	case path == "":
		return "", errors.New("a path is required to generate a key")
	case path == "~":
		return m.Paths.Home, nil
	case strings.HasPrefix(path, "~/"):
		return filepath.Join(m.Paths.Home, path[2:]), nil
	default:
		return path, nil
	}
}

// keygen runs ssh-keygen through the Manager's Runner.
//
// The passphrase reaches the child only as Command.Stdin, repeated once per
// prompt (generation asks twice, `-y` once). Detach removes the controlling
// terminal, so ssh-keygen must read the pipe instead of the terminal the
// interface is drawing on; clearing DISPLAY and pointing SSH_ASKPASS at a
// nonexistent path stop it from trying a graphical prompter and silently taking
// an empty passphrase. The passphrase therefore never appears in an argument
// list or an environment value.
func (m *Manager) keygen(ctx context.Context, passphrase string, confirmations int, args ...string) Result {
	if m.Caps.Keygen == "" {
		return Result{Code: -1, Err: errors.New("ssh-keygen was not found")}
	}
	cmd := Command{Name: m.Caps.Keygen, Args: args, Detach: true}
	if passphrase != "" {
		if confirmations < 1 {
			confirmations = 1
		}
		cmd.Stdin = strings.Repeat(passphrase+"\n", confirmations)
		cmd.Env = keyAskpassEnv()
	}
	run := m.Run
	if run == nil {
		run = ExecRunner{}
	}
	return run.Run(ctx, cmd)
}

// keyAskpassEnv is the environment that keeps a child process from falling back
// to a prompter that could quietly supply an empty passphrase.
func keyAskpassEnv() map[string]string {
	return map[string]string{
		"DISPLAY":     "",
		"SSH_ASKPASS": filepath.Join(os.TempDir(), keyAskpassNonexistent),
	}
}

// keyMessage is the tool's own first line, with a plain fallback when it said
// nothing. It never carries a passphrase: the tools say "incorrect passphrase",
// not the passphrase itself.
func keyMessage(res Result) string {
	if message := res.Message(); message != "" {
		return message
	}
	return "ssh-keygen failed without a message"
}

// keyMentionsPassphrase reports whether a tool's line is about a passphrase.
func keyMentionsPassphrase(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "passphrase") || strings.Contains(lower, "pass phrase") || strings.Contains(lower, "password")
}

// keyTypeSupported reports whether ssh-keygen's advertised list covers the
// requested type. The list from `ssh -Q key` uses wire names while a request may
// use the short form, so both are accepted.
func keyTypeSupported(supports []string, keyType string) bool {
	for _, supported := range supports {
		if supported == keyType {
			return true
		}
	}
	canonical := keyCanonicalType(keyType)
	if canonical == keyType {
		return false
	}
	for _, supported := range supports {
		if supported == canonical {
			return true
		}
	}
	return false
}

// keyCanonicalType maps the short names ssh-keygen accepts to wire names.
func keyCanonicalType(keyType string) string {
	switch keyType {
	case "ed25519":
		return "ssh-ed25519"
	case "rsa":
		return "ssh-rsa"
	case "ecdsa":
		return "ecdsa-sha2-nistp256"
	case "dsa":
		return "ssh-dss"
	case "ed25519-sk":
		return "sk-ssh-ed25519@openssh.com"
	case "ecdsa-sk":
		return "sk-ecdsa-sha2-nistp256@openssh.com"
	default:
		return keyType
	}
}
