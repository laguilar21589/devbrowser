//go:build !linux

package wsl

func IsWSL() bool        { return false }
func ToWindowsPath(p string) string { return p }
