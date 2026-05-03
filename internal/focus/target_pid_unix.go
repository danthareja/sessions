//go:build !windows

package focus

import "golang.org/x/sys/unix"

func pidAlive(pid int) bool {
	if pid <= 0 {
		return true
	}
	err := unix.Kill(pid, 0)
	return err == nil || err == unix.EPERM
}
