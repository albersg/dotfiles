//go:build !unix

package diskclean

import "errors"

// freeSpaceOn reports the bytes available on the filesystem that holds path.
// On a platform whose free-space query this package does not implement, the
// answer is an explicit error: the result says it could not measure the
// release rather than printing a zero as if it had.
func freeSpaceOn(string) (int64, error) {
	return 0, errors.New("this platform does not expose the free space of a filesystem")
}
