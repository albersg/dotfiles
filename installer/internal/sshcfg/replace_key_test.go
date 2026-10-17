package sshcfg

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateKeyWithReplaceRemovesTheOldPairFirst covers the one switch that is
// allowed to destroy something. ssh-keygen refuses to overwrite a key that is
// still there -- it asks "already exists. Overwrite (y/n)?" on a terminal this
// interface owns -- so a request that says the pair may be replaced has to
// remove that pair first. The stub mimics the real tool's refusal whenever the
// private file is still present, which is what makes the switch's behavior
// observable without running ssh-keygen.
func TestGenerateKeyWithReplaceRemovesTheOldPairFirst(t *testing.T) {
	e := keysTestEnv(t)
	priv := filepath.Join(e.manager.Paths.SSHDir, "id_replace")
	e.write(t, priv, 0o600, "OLD")
	e.write(t, priv+".pub", 0o644, "OLD")
	e.runner.handler = func(cmd Command) (Result, bool) {
		for i, a := range cmd.Args {
			if a == "-f" && i+1 < len(cmd.Args) {
				if _, err := os.Stat(cmd.Args[i+1]); err == nil {
					return failResult(1, cmd.Args[i+1]+" already exists.\nOverwrite (y/n)? "), true
				}
				os.WriteFile(cmd.Args[i+1], []byte("NEW"), 0o600)
				os.WriteFile(cmd.Args[i+1]+".pub", []byte("ssh-ed25519 AAAA new\n"), 0o644)
			}
		}
		return okResult(""), true
	}
	if _, err := e.manager.GenerateKey(context.Background(), KeygenRequest{Path: "~/.ssh/id_replace", Type: "ed25519", AllowReplace: true}); err != nil {
		t.Fatalf("GenerateKey with AllowReplace: %v", err)
	}
}
