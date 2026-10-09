// Package diskutil reports filesystem capacity. It exists so the HTTP layer and
// the build pipeline agree on one implementation: a build that starts without
// room fails halfway through, with an error nobody can act on.
package diskutil

import "syscall"

// FreeBytes returns the space available to an unprivileged process on the
// filesystem holding path. Non-root processes cannot use the root-reserved
// blocks, so Bavail (not Bfree) is the right number.
func FreeBytes(path string) (int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return int64(stat.Bavail) * int64(stat.Bsize), nil
}
