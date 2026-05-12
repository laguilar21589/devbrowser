//go:build linux

package browser

import (
	"syscall"
	"time"
)

// WaitForClose blocks until the process exits (Linux/Windows behavior:
// closing the window kills the process).
func WaitForClose(pid int) {
	for {
		time.Sleep(500 * time.Millisecond)
		if err := syscall.Kill(pid, 0); err != nil {
			return
		}
	}
}
