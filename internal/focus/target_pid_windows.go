//go:build windows

package focus

func pidAlive(pid int) bool {
	return true
}
