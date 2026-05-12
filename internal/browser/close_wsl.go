//go:build linux

package browser

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// WaitForCloseWSL blocks until no chrome.exe with the given Windows profile
// directory is running. Used in WSL where PID tracking is unreliable: the WSL
// interop stub exits when Chrome's initial launcher exits, not when the user
// closes the browser window.
func WaitForCloseWSL(windowsProfileDir string) {
	// Escape backslashes for PowerShell LIKE clause (\\ = literal \)
	escaped := strings.ReplaceAll(windowsProfileDir, `\`, `\\`)
	script := fmt.Sprintf(
		`(Get-CimInstance Win32_Process -Filter "Name='chrome.exe'" | Where-Object { $_.CommandLine -like '*%s*' } | Measure-Object).Count`,
		escaped,
	)

	poll := func() int {
		out, err := exec.Command("powershell.exe",
			"-NoProfile", "-NonInteractive", "-Command", script,
		).Output()
		if err != nil {
			return 0
		}
		// PowerShell may emit blank lines before the number
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				n, _ := strconv.Atoi(line)
				return n
			}
		}
		return 0
	}

	// Wait for Chrome to appear (up to 10s)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if poll() > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Wait for Chrome to disappear
	for {
		time.Sleep(2 * time.Second)
		if poll() == 0 {
			return
		}
	}
}
