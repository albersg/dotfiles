//go:build !unix

package diskclean

import (
	"testing"
)

// simulateMountAt cannot simulate a filesystem boundary on a platform whose
// stat data carries no device, so the guard it feeds is skipped there rather
// than weakened.
func simulateMountAt(t *testing.T, _ *Probe, _, _ string) {
	t.Helper()
	t.Skip("this platform cannot report a device, so a mount point cannot be simulated")
}
