package sshcfg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// THE KEYS, THE KNOWN HOSTS AND GENERATION
// ============================================================================
//
// The first promise of this file is the one that cannot be checked by reading
// the code that keeps it: a private key is never opened. The test below makes the
// private half unreadable and then asserts the inventory still describes it,
// which is only possible because every fact came from the public half, from
// ssh-keygen, or from os.Stat.
//
// The second promise is that a fact this package cannot prove is reported as
// unknown rather than as a comfortable default: "not encrypted" is never
// inferred from a missing passphrase prompt alone.

func keysTestEnv(t *testing.T) *testManager {
	t.Helper()
	e := testManagerSetup(t)
	e.mkdir(t, e.manager.Paths.SSHDir, 0o700)
	return e
}

// keysTestPublicLine is a real public key line, taken from an ed25519 key.
const keysTestPublicLine = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJdF+rRY8hTLrbwrnaiaQqv+g/TzcttecSI7iAn8hAjQ testuser@host"

func TestListKeysDescribesAPairWithoutOpeningThePrivateHalf(t *testing.T) {
	e := keysTestEnv(t)
	priv := filepath.Join(e.manager.Paths.SSHDir, "id_ed25519")
	e.write(t, priv, 0o600, "-----BEGIN OPENSSH PRIVATE KEY-----\nsecret material\n")
	e.write(t, priv+".pub", 0o644, keysTestPublicLine+"\n")

	// The private half is made unreadable. A listing that opened it would fail
	// here, and the mode is also what the inventory is supposed to report.
	if err := os.Chmod(priv, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	keys, err := e.manager.ListKeys(context.Background())
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("listed %d keys, want 1: %+v", len(keys), keys)
	}
	key := keys[0]
	if key.Path != priv {
		t.Errorf("Path = %q, want the private-side path %q", key.Path, priv)
	}
	if key.PubPath != priv+".pub" {
		t.Errorf("PubPath = %q", key.PubPath)
	}
	if !key.PublicKnown || key.Public != keysTestPublicLine {
		t.Errorf("the public half was not carried: %+v", key)
	}
	if key.Type != "ssh-ed25519" || key.Comment != "testuser@host" {
		t.Errorf("type or comment came from nowhere: %+v", key)
	}
	if key.Mode != 0o000 {
		t.Errorf("Mode = %v, want the mode the private half really has", key.Mode)
	}
	// The rule is about what *other* users can read, so a key nobody but the owner
	// could read is reported as fine even when the owner cannot read it either: the
	// mode is a fact of its own, and the inventory states it separately.
	if !key.PermissionsOK {
		t.Error("a private key with no group or other bits was reported as too open")
	}
	if strings.Contains(key.Public, "secret material") || strings.Contains(strings.Join(key.Warnings, " "), "secret material") {
		t.Error("the private half's bytes reached the inventory")
	}
}

func TestListKeysReportsPrivatePermissions(t *testing.T) {
	cases := []struct {
		name string
		mode os.FileMode
		want bool
	}{
		{"a private key", 0o600, true},
		{"a key its group can read", 0o640, false},
		{"a key everyone can read", 0o644, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := keysTestEnv(t)
			priv := filepath.Join(e.manager.Paths.SSHDir, "id_test")
			e.write(t, priv, tt.mode, "-----BEGIN OPENSSH PRIVATE KEY-----\n")
			e.write(t, priv+".pub", 0o644, keysTestPublicLine+"\n")

			keys, err := e.manager.ListKeys(context.Background())
			if err != nil {
				t.Fatalf("ListKeys: %v", err)
			}
			if len(keys) != 1 {
				t.Fatalf("listed %d keys, want 1", len(keys))
			}
			if keys[0].PermissionsOK != tt.want {
				t.Errorf("PermissionsOK = %v for mode %v, want %v", keys[0].PermissionsOK, tt.mode, tt.want)
			}
		})
	}
}

func TestListKeysIgnoresTheClientsOwnFiles(t *testing.T) {
	e := keysTestEnv(t)
	for _, name := range []string{"config", "known_hosts", "known_hosts.old", "authorized_keys", "agent.env"} {
		e.write(t, filepath.Join(e.manager.Paths.SSHDir, name), 0o600, "not a key\n")
	}
	e.write(t, filepath.Join(e.manager.Paths.SSHDir, "id_ed25519.pub"), 0o644, keysTestPublicLine+"\n")

	keys, err := e.manager.ListKeys(context.Background())
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("listed %+v, want only the key pair", keys)
	}
	if !strings.HasSuffix(keys[0].PubPath, "id_ed25519.pub") {
		t.Errorf("the listed entry is %q, want the key", keys[0].PubPath)
	}
}

func TestListKeysAsksSshKeygenAboutAPrivateKeyWithNoPublicHalf(t *testing.T) {
	e := keysTestEnv(t)
	priv := filepath.Join(e.manager.Paths.SSHDir, "id_rsa")
	e.write(t, priv, 0o600, "-----BEGIN OPENSSH PRIVATE KEY-----\n")
	e.runner.handler = func(cmd Command) (Result, bool) {
		if cmd.Name != e.manager.Caps.Keygen {
			return Result{}, false
		}
		switch {
		case equalStrings(cmd.Args, []string{"-l", "-f", priv}):
			return okResult("4096 SHA256:ccccccccccccccccccccccccccccccccccccccccccc testuser@host (RSA)\n"), true
		case equalStrings(cmd.Args, []string{"-y", "-f", priv}):
			return okResult("ssh-rsa AAAAB3NzaC1yc2E testuser@host\n"), true
		}
		return Result{}, false
	}

	keys, err := e.manager.ListKeys(context.Background())
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("listed %+v, want one key", keys)
	}
	key := keys[0]
	if key.Bits != 4096 || key.Type != "ssh-rsa" || key.Comment != "testuser@host" {
		t.Errorf("the key's facts did not come from ssh-keygen: %+v", key)
	}
	// "not encrypted" is only ever claimed after `ssh-keygen -y` accepted the key
	// with nothing on the pipe.
	if key.Encrypted != "no" {
		t.Errorf("Encrypted = %q, want %q for a key ssh-keygen opened without a passphrase", key.Encrypted, "no")
	}
	if key.PublicKnown {
		t.Error("a key with no public half claims a public half")
	}
}

func TestListKeysSaysUnknownRatherThanAssuming(t *testing.T) {
	t.Run("an encrypted key it cannot open", func(t *testing.T) {
		e := keysTestEnv(t)
		priv := filepath.Join(e.manager.Paths.SSHDir, "id_enc")
		e.write(t, priv, 0o600, "-----BEGIN OPENSSH PRIVATE KEY-----\n")
		e.runner.handler = func(cmd Command) (Result, bool) {
			return failResult(1, "Enter passphrase for "+priv+": \n"), true
		}

		keys, err := e.manager.ListKeys(context.Background())
		if err != nil {
			t.Fatalf("ListKeys: %v", err)
		}
		if len(keys) != 1 {
			t.Fatalf("listed %+v, want one key", keys)
		}
		if keys[0].Encrypted != "yes" {
			t.Errorf("Encrypted = %q, want %q: ssh-keygen asked for a passphrase", keys[0].Encrypted, "yes")
		}
		if len(keys[0].Warnings) == 0 {
			t.Error("a key whose type could not be learned carries no warning")
		}
	})

	t.Run("a file that is not a key at all", func(t *testing.T) {
		e := keysTestEnv(t)
		priv := filepath.Join(e.manager.Paths.SSHDir, "notes.txt")
		e.write(t, priv, 0o600, "just a text file\n")
		e.runner.handler = func(cmd Command) (Result, bool) {
			return failResult(1, priv+": is not a key file\n"), true
		}

		keys, err := e.manager.ListKeys(context.Background())
		if err != nil {
			t.Fatalf("ListKeys: %v", err)
		}
		if len(keys) != 1 {
			t.Fatalf("listed %+v, want the file reported rather than dropped", keys)
		}
		if keys[0].Encrypted != "unknown" {
			t.Errorf("Encrypted = %q, want %q for a file that is not a key", keys[0].Encrypted, "unknown")
		}
	})
}

func TestListKnownHostsParsesWhatTheClientWrote(t *testing.T) {
	e := keysTestEnv(t)
	const entries = "# a comment\n" +
		"github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIcccc\n" +
		"|1|aGFzaGVk|aGFzaGVkc2FsdA= ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIdddd\n" +
		"@cert-authority *.example.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIeeee\n" +
		"not-a-host-key\n"
	e.write(t, e.manager.Paths.KnownHosts, 0o600, entries)
	// known_hosts.old is a previous revision: reading it would claim a trust the
	// client is not using.
	e.write(t, e.manager.Paths.KnownHosts+".old", 0o600, "an-old-host ssh-rsa AAAA\n")

	hosts, err := e.manager.ListKnownHosts(context.Background())
	if err != nil {
		t.Fatalf("ListKnownHosts: %v", err)
	}
	if len(hosts) != 4 {
		t.Fatalf("parsed %d entries, want 4 (comments are not entries): %+v", len(hosts), hosts)
	}
	if !equalStrings(hosts[0].Hosts, []string{"github.com"}) || hosts[0].Type != "ssh-ed25519" {
		t.Errorf("first entry = %+v", hosts[0])
	}
	if !hosts[1].Hashed {
		t.Error("a hashed entry was reported as a spelled-out host")
	}
	if len(hosts[1].Hosts) != 0 {
		t.Errorf("a hashed entry carries host names it cannot know: %+v", hosts[1].Hosts)
	}
	if len(hosts[2].Markers) != 1 || hosts[2].Markers[0] != "@cert-authority" {
		t.Errorf("the certificate authority marker was lost: %+v", hosts[2])
	}
	// A malformed line is reported with a warning rather than dropped: a line
	// nobody could read is exactly the kind a user needs to know about.
	if len(hosts[3].Warnings) == 0 {
		t.Errorf("the unparsable line was reported without a warning: %+v", hosts[3])
	}
	for _, host := range hosts {
		if strings.Contains(strings.Join(host.Hosts, " "), "an-old-host") {
			t.Error("known_hosts.old was read")
		}
	}
	if hosts[0].Line != 2 || hosts[0].File != e.manager.Paths.KnownHosts {
		t.Errorf("the entry's location is %s:%d, want the file line", hosts[0].File, hosts[0].Line)
	}
}

func TestListKnownHostsOnAMissingFileIsNotAnError(t *testing.T) {
	e := keysTestEnv(t)
	hosts, err := e.manager.ListKnownHosts(context.Background())
	if err != nil {
		t.Fatalf("ListKnownHosts on a missing file: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("parsed %+v from a file that does not exist", hosts)
	}
}

func TestListKeysOnAnUnreadableDirectoryIsAnError(t *testing.T) {
	e := testManagerSetup(t)
	// The directory is never created: an inventory of a directory that was not
	// read must be an error rather than an empty list, which would read as "you
	// have no keys".
	if _, err := e.manager.ListKeys(context.Background()); err == nil {
		t.Fatal("ListKeys reported an empty inventory for a directory it could not read")
	}
}

func TestLookupKnownHostAsksSshKeygen(t *testing.T) {
	e := keysTestEnv(t)
	e.write(t, e.manager.Paths.KnownHosts, 0o600, "")
	e.runner.handler = func(cmd Command) (Result, bool) {
		if equalStrings(cmd.Args, []string{"-F", "github.com", "-f", e.manager.Paths.KnownHosts}) {
			return okResult("# Host github.com found: line 1\n|1|aGFzaGVk|aGFzaGVkc2FsdA= ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIcccc\n"), true
		}
		return Result{}, false
	}

	hosts, err := e.manager.LookupKnownHost(context.Background(), "github.com")
	if err != nil {
		t.Fatalf("LookupKnownHost: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("found %d entries, want 1: %+v", len(hosts), hosts)
	}
	if !hosts[0].Hashed {
		t.Error("the lookup did not report the entry as hashed")
	}
}

func TestLookupKnownHostWithoutAMatchIsNotAnError(t *testing.T) {
	e := keysTestEnv(t)
	e.write(t, e.manager.Paths.KnownHosts, 0o600, "")
	e.runner.handler = func(cmd Command) (Result, bool) { return failResult(1, ""), true }

	hosts, err := e.manager.LookupKnownHost(context.Background(), "github.com")
	if err != nil {
		t.Fatalf("LookupKnownHost: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("found %+v for a host with no entry", hosts)
	}
}

func TestGenerateKeyAsksSshKeygenWithTheDefaults(t *testing.T) {
	e := keysTestEnv(t)
	path := filepath.Join(e.manager.Paths.SSHDir, "id_ed25519")
	e.runner.handler = func(cmd Command) (Result, bool) {
		// ssh-keygen would write the files; the test writes them instead so the
		// modes and the public-half read can be exercised without a real key.
		if strings.HasPrefix(cmd.Name, "/usr/bin/ssh-keygen") && len(cmd.Args) > 0 && cmd.Args[0] == "-q" {
			e.write(t, path, 0o600, "-----BEGIN OPENSSH PRIVATE KEY-----\n")
			e.write(t, path+".pub", 0o644, keysTestPublicLine+"\n")
			return okResult(""), true
		}
		if equalStrings(cmd.Args, []string{"-y", "-f", path}) {
			return okResult("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI testuser@host\n"), true
		}
		return Result{}, false
	}

	result, err := e.manager.GenerateKey(context.Background(), KeygenRequest{Path: path, Comment: "testuser@host"})
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	want := []string{"-q", "-t", "ed25519", "-a", "100", "-f", path, "-C", "testuser@host"}
	if !equalStrings(result.Args, want) {
		t.Errorf("args =\n%v\nwant\n%v", result.Args, want)
	}
	if !equalStrings(e.runner.calls[0].Args, want) {
		t.Errorf("the command that ran was %v, want %v", e.runner.calls[0].Args, want)
	}
	if result.Key.Encrypted != "no" {
		t.Errorf("Encrypted = %q, want %q proven by ssh-keygen -y", result.Key.Encrypted, "no")
	}
	if !result.Key.PublicKnown {
		t.Error("the generated pair was not described from its public half")
	}
}

func TestGenerateKeyPutsThePassphraseOnThePipeOnly(t *testing.T) {
	const passphrase = "another correct horse"
	e := keysTestEnv(t)
	path := filepath.Join(e.manager.Paths.SSHDir, "id_work")
	e.runner.handler = func(cmd Command) (Result, bool) {
		if cmd.Args[0] == "-q" {
			e.write(t, path, 0o600, "-----BEGIN OPENSSH PRIVATE KEY-----\n")
			e.write(t, path+".pub", 0o644, keysTestPublicLine+"\n")
			return okResult(""), true
		}
		if cmd.Args[0] == "-y" {
			return okResult("ssh-ed25519 AAAA testuser@host\n"), true
		}
		return Result{}, false
	}

	result, err := e.manager.GenerateKey(context.Background(), KeygenRequest{
		Path: path, Comment: "testuser@host", Passphrase: passphrase,
	})
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	for _, call := range e.runner.calls {
		for _, arg := range call.Args {
			if strings.Contains(arg, passphrase) {
				t.Fatal("the passphrase was passed as an argument")
			}
		}
		for name, value := range call.Env {
			if strings.Contains(value, passphrase) {
				t.Fatalf("the passphrase was passed in the environment (%s)", name)
			}
		}
	}
	generate := e.runner.calls[0]
	if generate.Stdin != passphrase+"\n"+passphrase+"\n" {
		t.Errorf("Stdin = %q, want the passphrase twice for the two prompts", generate.Stdin)
	}
	if !generate.Detach {
		t.Error("the child was not detached, so it could read the terminal")
	}
	if generate.Env["SSH_ASKPASS"] == "" || generate.Env["DISPLAY"] != "" {
		t.Errorf("Env = %v, want the graphical prompter disabled", generate.Env)
	}
	if !result.PassphraseConfirmed {
		t.Error("the passphrase was not confirmed against the new key")
	}
	if result.Key.Encrypted != "yes" {
		t.Errorf("Encrypted = %q, want %q", result.Key.Encrypted, "yes")
	}
	// The key's mode is set explicitly: ssh-keygen honours the umask.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("the private key is mode %v, want 0600", info.Mode().Perm())
	}
	public, err := os.Stat(path + ".pub")
	if err != nil {
		t.Fatalf("stat the public half: %v", err)
	}
	if public.Mode().Perm() != 0o644 {
		t.Errorf("the public key is mode %v, want 0644", public.Mode().Perm())
	}
}

func TestGenerateKeyRefusesToOverwriteAPair(t *testing.T) {
	e := keysTestEnv(t)
	path := filepath.Join(e.manager.Paths.SSHDir, "id_ed25519")
	e.write(t, path, 0o600, "-----BEGIN OPENSSH PRIVATE KEY-----\nthe key that is there\n")

	_, err := e.manager.GenerateKey(context.Background(), KeygenRequest{Path: path})
	if err == nil {
		t.Fatal("GenerateKey overwrote an existing key")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("the refusal does not say the key exists: %v", err)
	}
	if len(e.runner.calls) != 0 {
		t.Errorf("ssh-keygen ran for a refused generation: %v", e.runner.calls)
	}
	requireUnchanged(t, "the key that was there", e.read(t, path), "-----BEGIN OPENSSH PRIVATE KEY-----\nthe key that is there\n")
}

func TestGenerateKeyRefusesATypeSshKeygenCannotMake(t *testing.T) {
	e := keysTestEnv(t)
	path := filepath.Join(e.manager.Paths.SSHDir, "id_dsa")

	_, err := e.manager.GenerateKey(context.Background(), KeygenRequest{Path: path, Type: "ssh-dss"})
	if err == nil {
		t.Fatal("GenerateKey asked for a type this ssh-keygen does not support")
	}
	if !strings.Contains(err.Error(), "ssh-dss") {
		t.Errorf("the refusal does not name the type: %v", err)
	}
	if len(e.runner.calls) != 0 {
		t.Errorf("ssh-keygen ran for a refused type: %v", e.runner.calls)
	}
}

func TestGenerateKeyReportsARefusalFromSshKeygen(t *testing.T) {
	e := keysTestEnv(t)
	path := filepath.Join(e.manager.Paths.SSHDir, "id_ed25519")
	e.runner.handler = func(cmd Command) (Result, bool) {
		return failResult(1, "Saving key \""+path+"\" failed: Permission denied\n"), true
	}

	result, err := e.manager.GenerateKey(context.Background(), KeygenRequest{Path: path, Comment: "c"})
	if err == nil {
		t.Fatal("GenerateKey reported success for a refused write")
	}
	if !strings.Contains(err.Error(), "Permission denied") {
		t.Errorf("the error does not quote ssh-keygen: %v", err)
	}
	if len(result.Args) == 0 {
		t.Error("the result does not carry the argument list that ran")
	}
	// A refused generation leaves nothing behind.
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("a file was left behind: %v", statErr)
	}
}

func TestGenerateKeyWantsAKeygenBeforeItStarts(t *testing.T) {
	e := keysTestEnv(t)
	e.manager.Caps.Keygen = ""
	_, err := e.manager.GenerateKey(context.Background(), KeygenRequest{Path: filepath.Join(e.manager.Paths.SSHDir, "id_ed25519")})
	if err == nil {
		t.Fatal("GenerateKey started without ssh-keygen")
	}
	if !strings.Contains(err.Error(), "ssh-keygen") {
		t.Errorf("the refusal does not name the missing tool: %v", err)
	}
}
