//go:build linux

package wsl

import (
	"os"
	"os/exec"
	"strings"
)

// IsWSL returns true when running inside Windows Subsystem for Linux.
func IsWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, _ := os.ReadFile("/proc/version")
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

// ToWindowsPath converts a Linux/WSL path to a Windows UNC path using wslpath.
// Example: /root/.devbrowser/profiles/x → \\wsl.localhost\Ubuntu\root\.devbrowser\profiles\x
// Returns the original path unchanged if wslpath is unavailable or fails.
func ToWindowsPath(path string) string {
	out, err := exec.Command("wslpath", "-w", path).Output()
	if err != nil {
		return path
	}
	return strings.TrimSpace(string(out))
}
