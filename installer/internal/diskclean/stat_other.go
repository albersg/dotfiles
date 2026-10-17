//go:build !unix

package diskclean

import "os"

// fileID identifies a file on one filesystem. On a platform whose stat data
// does not expose a device and an inode pair, the identity is empty and the
// walk counts every file it meets: a shared file may then be counted twice,
// which is the honest failure mode for a platform this package cannot promise
// dedupe on.
type fileID struct {
	device uint64
	inode  uint64
}

// valid reports whether the identity could be read.
func (id fileID) valid() bool { return false }

// fileIDOf returns the empty identity: nothing to dedupe by.
func fileIDOf(os.FileInfo) fileID { return fileID{} }

// deviceOf returns no filesystem identity, so a walk never declares a mount
// point it cannot prove.
func deviceOf(os.FileInfo) uint64 { return 0 }

// physicalSize falls back to the logical size where the allocation is not
// exposed.
func physicalSize(info os.FileInfo) int64 { return info.Size() }
