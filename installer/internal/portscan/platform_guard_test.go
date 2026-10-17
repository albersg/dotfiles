package portscan

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// platformGuardTimeout bounds one cross-platform type check. The go tool may
// have to compile the standard library for that GOOS the first time, so the
// bound is generous; a command that hangs is a failure, not something to wait
// on forever.
const platformGuardTimeout = 5 * time.Minute

// TestThePackageTypeChecksForEveryPlatformItClaims is the guard for the defect
// this file was added with: the package built and its tests passed on the host
// running them, while a macOS CI runner could not compile it at all. A build
// constraint mistake -- a reader hidden behind a _linux.go suffix, a fact
// reader that only existed on one platform, an untagged test naming a
// Linux-only symbol -- is invisible to a host-only build, so this test builds
// the package the way every platform it claims support would.
//
// `go vet` is used rather than `go build` because vet type-checks the test
// files as well: the untagged tests have to compile on the platforms this
// package ships on, and a test that names a Linux-only helper breaks the
// macOS build just as surely as the reader did.
//
// The check runs in a child process with GOOS set, which is the only way to
// ask the toolchain about another platform. It is skipped, with the reason
// stated, when the go tool is not on PATH: the test cannot answer then, and a
// green result it did not measure would be worse than a named skip.
func TestThePackageTypeChecksForEveryPlatformItClaims(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("skipping the cross-platform build guard: the go tool is not on PATH (%v)", err)
	}

	for _, goos := range []string{"linux", "darwin", "windows"} {
		goos := goos
		t.Run(goos, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), platformGuardTimeout)
			defer cancel()

			cmd := exec.CommandContext(ctx, goTool, "vet", ".")
			// The child gets this process's environment with GOOS replaced, so a
			// GOOS the caller exported cannot shadow the platform under test.
			cmd.Env = environmentFor(goos)

			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("the %s type check did not finish within %s", goos, platformGuardTimeout)
			}
			if err != nil {
				t.Errorf("the package does not type-check for %s (%v):\n%s", goos, err, strings.TrimSpace(string(output)))
			}
		})
	}
}

// environmentFor returns the current environment with GOOS set to goos and any
// inherited GOOS removed. Duplicate entries would leave the child's answer up
// to whichever one its runtime reads first, which is not a fact this guard can
// rely on.
func environmentFor(goos string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GOOS=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "GOOS="+goos)
}
