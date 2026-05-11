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

// ToWindowsLocalPath converts a WSL /mnt/<drive>/... path to a Windows
// drive-letter path (e.g. /mnt/c/Users/foo → C:\Users\foo).
// Returns "" if the path is not under a /mnt/<letter>/ mount.
// Prefer this over ToWindowsPath to avoid UNC network share paths that
// Chrome refuses to use as --user-data-dir.
func ToWindowsLocalPath(path string) string {
	if len(path) >= 7 && path[:5] == "/mnt/" && path[6] == '/' {
		drive := strings.ToUpper(string(path[5]))
		rest := strings.ReplaceAll(path[6:], "/", `\`)
		return drive + ":" + rest
	}
	return ""
}

// WindowsProfilesBaseDir returns the Windows user home as a WSL mount path
// (e.g. C:\Users\foo → /mnt/c/Users/foo), or "" on failure.
// Use this to store Chrome profiles on the Windows filesystem so Chrome.exe
// gets a native drive-letter path instead of a UNC \\wsl.localhost\ share.
func WindowsProfilesBaseDir() string {
	// Try env var first (only set when WSLENV forwards USERPROFILE)
	if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
		return winPathToWSL(userProfile)
	}
	// Fall back to cmd.exe — always available in WSL2
	out, err := exec.Command("cmd.exe", "/c", "echo %USERPROFILE%").Output()
	if err != nil {
		return ""
	}
	userProfile := strings.TrimSpace(string(out))
	return winPathToWSL(userProfile)
}

// winPathToWSL converts a Windows drive-letter path to a WSL mount path.
// e.g. C:\Users\foo → /mnt/c/Users/foo
func winPathToWSL(winPath string) string {
	if len(winPath) >= 2 && winPath[1] == ':' {
		drive := strings.ToLower(string(winPath[0]))
		rest := strings.ReplaceAll(winPath[2:], `\`, "/")
		return "/mnt/" + drive + rest
	}
	return ""
}
