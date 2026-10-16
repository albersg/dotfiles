package system

import (
	"strconv"
	"strings"
)

// MeminfoBytes reads one field of /proc/meminfo in bytes, e.g. "MemTotal", "MemAvailable" or
// "SwapFree". Every field of that file is written in kibibytes, and this is the one place that
// conversion lives: a second copy of it is how one reader comes to report a different number than
// another for the same machine.
//
// It reports false for a key the file does not carry, so a caller can leave a row out rather than
// print a zero it did not measure.
func MeminfoBytes(data []byte, key string) (uint64, bool) {
	kilobytes, ok := meminfoKilobytes(data, key)
	if !ok {
		return 0, false
	}
	return kilobytes * 1024, true
}

// MeminfoUsed reads the instance's used and total memory from /proc/meminfo.
// MemAvailable is preferred because MemFree excludes reclaimable cache and would read as "full" on a
// healthy desktop; MemFree is the fallback. Used is total minus that value, clamped so a nonsense
// pair cannot underflow. It reports false when the file carries no total or neither of the two
// availability fields, so a caller can leave the reading out rather than print a zero.
func MeminfoUsed(data []byte) (used, total uint64, ok bool) {
	memTotal, haveTotal := MeminfoBytes(data, "MemTotal")
	if !haveTotal || memTotal == 0 {
		return 0, 0, false
	}
	memAvail, haveAvail := MeminfoBytes(data, "MemAvailable")
	memFree, haveFree := MeminfoBytes(data, "MemFree")

	switch {
	case haveAvail:
		used = memTotal - min(memAvail, memTotal)
	case haveFree:
		used = memTotal - min(memFree, memTotal)
	default:
		return 0, 0, false
	}
	return used, memTotal, true
}

// meminfoKilobytes reads one field as the number the file writes, without the unit conversion, so a
// caller that wants the raw kibibytes does not have to divide a value that was just multiplied.
func meminfoKilobytes(data []byte, key string) (uint64, bool) {
	for _, line := range strings.Split(string(data), "\n") {
		name, rest, found := strings.Cut(line, ":")
		if !found || name != key {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return 0, false
		}
		value, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0, false
		}
		return value, true
	}
	return 0, false
}
