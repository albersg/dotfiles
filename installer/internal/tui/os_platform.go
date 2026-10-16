package tui

import (
	"fmt"
	"runtime"
	"strings"
)

// The platform a run works from is answered in two places that have to agree: the
// setup screen's first row, which records one of the answer keys osOptions produces,
// and the --os flag, whose accepted values are read from that same row rather than
// written out a second time. WSL is deliberately absent: it is a fact about the
// machine -- SystemInfo.IsWSL, which schedules the configuration step -- and not a
// platform anybody chooses, so a flag offering it would offer a value no row can
// record.
func SupportedOperatingSystems() []string {
	var platforms []string
	for _, option := range (Model{}).osOptions() {
		key := questionAnswerKey(ScreenOSSelect, option)
		if key == "" {
			continue
		}
		known := false
		for _, seen := range platforms {
			if seen == key {
				known = true
				break
			}
		}
		if !known {
			platforms = append(platforms, key)
		}
	}
	return platforms
}

// ValidateOperatingSystem reports whether --os=<value> is a platform this installer
// works from. An empty value is the machine's own answer and is accepted; anything
// else has to be one of the values the setup screen's row records, and the refusal
// names them, because a flag that quietly fell back to Linux would leave the user
// reading a Linux plan under a Mac name.
func ValidateOperatingSystem(value string) error {
	if value == "" {
		return nil
	}
	for _, supported := range SupportedOperatingSystems() {
		if value == supported {
			return nil
		}
	}
	return fmt.Errorf("--os=%s is not a platform this installer offers: it accepts %s, or no value at all to work from the machine it runs on",
		value, strings.Join(SupportedOperatingSystems(), ", "))
}

// PlatformFor is the one place the platform a run works from is decided: the value
// the caller chose, and the machine only when nothing was chosen. It replaces the
// non-interactive path's imposition of linux or mac, which overwrote a chosen
// platform before anything could read it.
func PlatformFor(choices UserChoices) string {
	if choices.OS != "" {
		return choices.OS
	}
	if runtime.GOOS == "darwin" {
		return "mac"
	}
	return "linux"
}
