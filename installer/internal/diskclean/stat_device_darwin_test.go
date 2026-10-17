//go:build darwin

package diskclean

import "syscall"

// setStatDevice sets the device of a fake stat on darwin, where Stat_t.Dev is
// an int32 rather than the uint64 every other unix in this package uses. The
// values a simulated mount passes here are small device numbers, so the
// narrowing cannot lose one.
func setStatDevice(stat *syscall.Stat_t, device uint64) { stat.Dev = int32(device) }
