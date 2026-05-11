package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/laguilar-io/devbrowser/internal/wsl"
)

// Find returns the path to Chrome/Chromium, using override if set.
func Find(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	// Fallback: PATH
	for _, name := range []string{"google-chrome-stable", "google-chrome", "chromium-browser", "chromium"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("Chrome or Chromium not found — set browser_path in ~/.devbrowser/config.toml")
}

// Launch starts Chrome with an isolated profile pointing at url.
// Returns the running *exec.Cmd so the caller can Wait() on it.
func Launch(binary, profileDir, url string) (*exec.Cmd, error) {
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return nil, err
	}
	// Suppress the "First Run" welcome screen
	firstRun := filepath.Join(profileDir, "First Run")
	if _, err := os.Stat(firstRun); os.IsNotExist(err) {
		_ = os.WriteFile(firstRun, []byte{}, 0644)
	}

	// In WSL, chrome.exe is a Windows process and expects a Windows-style path
	// for --user-data-dir. Convert the Linux path to a UNC path via wslpath.
	userDataDir := profileDir
	if wsl.IsWSL() && strings.HasSuffix(strings.ToLower(binary), ".exe") {
		userDataDir = wsl.ToWindowsPath(profileDir)
	}

	cmd := exec.Command(binary,
		"--user-data-dir="+userDataDir,
		"--disable-extensions",
		"--auto-open-devtools-for-tabs",
		"--no-first-run",
		url,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}
	return cmd, nil
}
