//go:build linux

package browser

import (
	"os"

	"github.com/laguilar-io/devbrowser/internal/wsl"
)

var candidates []string

func init() {
	native := []string{
		"/usr/bin/google-chrome-stable",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium-browser",
		"/usr/bin/chromium",
		"/snap/bin/chromium",
	}

	if !wsl.IsWSL() {
		candidates = native
		return
	}

	// In WSL, Chrome is a Windows application. Look for chrome.exe in the
	// standard Windows installation paths (accessible via /mnt/c/...).
	windowsPaths := []string{
		`/mnt/c/Program Files/Google/Chrome/Application/chrome.exe`,
		`/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe`,
	}

	// Also check the current Windows user's AppData (LOCALAPPDATA is usually
	// forwarded into WSL when WSLENV includes it, or set by default in WSL2).
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		// LOCALAPPDATA is a Windows path like C:\Users\name\AppData\Local.
		// Convert to a WSL path the easy way: swap the drive letter prefix.
		// wslpath would be more robust but we avoid an exec here.
		windowsPaths = append(windowsPaths,
			localAppDataToWSL(localAppData)+`/Google/Chrome/Application/chrome.exe`,
		)
	}

	// Prefer Windows Chrome first in WSL; fall back to native Linux Chrome
	// (e.g. installed via WSLg on Windows 11).
	candidates = append(windowsPaths, native...)
}

// localAppDataToWSL does a best-effort conversion of a Windows LOCALAPPDATA
// path (e.g. "C:\Users\foo\AppData\Local") to a WSL mount path
// ("/mnt/c/Users/foo/AppData/Local"). It handles drive letters A-Z.
func localAppDataToWSL(winPath string) string {
	if len(winPath) >= 2 && winPath[1] == ':' {
		drive := string(winPath[0] + 32) // upper → lower
		rest := winPath[2:]
		// Replace backslashes with forward slashes
		result := "/mnt/" + drive
		for _, c := range rest {
			if c == '\\' {
				result += "/"
			} else {
				result += string(c)
			}
		}
		return result
	}
	return winPath
}
