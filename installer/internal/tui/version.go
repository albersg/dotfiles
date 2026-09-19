package tui

import "strings"

// Version is the build the installer was made from. Release builds set it at
// link time with -ldflags "-X main.Version=<version>" and then hand that value
// to this variable; a build straight from the source tree leaves it at the
// "dev" default.
var Version = "dev"

// VersionLabel renders Version for display. A value that is empty or equal to
// "dev" (compared case-insensitively) becomes "dev build": that names what a
// source build actually is. The default deliberately gets no "v" prefix,
// because "vdev" reads like a typo rather than information.
//
// Release tags already carry the "v" prefix (for example "v0.1.0"), so a value
// that already starts with "v" is returned unchanged instead of producing the
// nonsense "vv0.1.0". Every other value is prefixed with "v" so "0.1.0" reads
// as "v0.1.0".
func VersionLabel() string {
	if Version == "" || strings.EqualFold(Version, "dev") {
		return "dev build"
	}
	if strings.HasPrefix(Version, "v") {
		return Version
	}
	return "v" + Version
}
