//go:build unix && !darwin

package diskclean

import "syscall"

// setStatDevice sets the device of a fake stat on the unix platforms whose
// Stat_t keeps it as a uint64. The darwin half lives beside this file, because
// darwin's Stat_t.Dev is an int32 and this assignment does not compile there --
// which is what the macOS job of the pull request's first CI run reported
// (measure_mount_unix_test.go:23), a build failure that no run on Linux could
// see.
func setStatDevice(stat *syscall.Stat_t, device uint64) { stat.Dev = device }
