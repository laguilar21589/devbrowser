//go:build windows

package browser

import "golang.org/x/sys/windows"

// WaitForClose blocks until the process exits.
// On Windows we use WaitForSingleObject so we can wait on any process handle,
// not just child processes.
func WaitForClose(pid int) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	_, _ = windows.WaitForSingleObject(h, windows.INFINITE)
}
