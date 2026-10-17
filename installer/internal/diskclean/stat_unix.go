//go:build unix

package diskclean

import (
	"os"
	"syscall"
)

// fileID identifies a file on one filesystem. Device and inode together are
// what make two paths the same file: a hardlink is two names for one (device,
// inode), and a directory loop is a walk arriving at an inode it has seen.
type fileID struct {
	device uint64
	inode  uint64
}

// valid reports whether the identity could be read. A platform or filesystem
// that answers nothing gets no dedupe rather than a wrong one.
func (id fileID) valid() bool { return id.device != 0 || id.inode != 0 }

// fileIDOf reads the identity of a file from its stat data.
func fileIDOf(info os.FileInfo) fileID {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileID{}
	}
	return fileID{device: uint64(stat.Dev), inode: uint64(stat.Ino)}
}

// deviceOf reads the filesystem a path lives on, which is how a walk knows it
// has reached a mount point rather than a directory of the store it measures.
func deviceOf(info os.FileInfo) uint64 {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return uint64(stat.Dev)
}

// physicalSize is what the filesystem allocated for a file: its block count
// times the block size. That is the space freeing the file returns, and on a
// compressed or sparse file it is smaller than the logical size.
func physicalSize(info os.FileInfo) int64 {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return info.Size()
	}
	if stat.Blocks <= 0 {
		return 0
	}
	return stat.Blocks * 512
}
