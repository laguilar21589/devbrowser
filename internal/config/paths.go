package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/laguilar-io/devbrowser/internal/wsl"
)

func BaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".devbrowser")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func ProfilesDir() (string, error) {
	// In WSL, store profiles on the Windows filesystem so Chrome.exe receives a
	// native drive-letter path (C:\...) instead of a UNC \\wsl.localhost\ share,
	// which Chrome refuses to use as --user-data-dir.
	if wsl.IsWSL() {
		if winBase := wsl.WindowsProfilesBaseDir(); winBase != "" {
			dir := filepath.Join(winBase, ".devbrowser", "profiles")
			if err := os.MkdirAll(dir, 0755); err == nil {
				return dir, nil
			}
		}
	}

	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "profiles")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func ConfigFile() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "config.toml"), nil
}

func StateFile() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "state.json"), nil
}

func StateLockFile() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "state.lock"), nil
}

// ExpandHome replaces a leading ~ with the user's home directory.
func ExpandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
