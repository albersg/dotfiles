//go:build unix

package diskclean

import "syscall"

// freeSpaceOn reports the bytes available to an unprivileged process on the
// filesystem that holds path. It is the number the result screen uses to say
// what a run released: the filesystem's own answer, not an estimate this
// package derived from the sizes it removed.
func freeSpaceOn(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	// Bavail, not Bfree: the reserved blocks are not space the user gets back.
	// The multiplication is done in 128 bits' worth of care by converting the
	// block count first, because a large filesystem's block count times its
	// block size can exceed what an int64 holds in intermediate arithmetic on a
	// 32-bit build.
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}
