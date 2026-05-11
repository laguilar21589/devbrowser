package envfiles

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// localEnvFiles are the gitignored env files Next.js (and others) use locally.
// .env is included because many projects use it directly (gitignored via .gitignore).
var localEnvFiles = []string{
	".env",
	".env.local",
	".env.development.local",
	".env.test.local",
	".env.production.local",
}

// CopyResult tracks which files were copied so they can be cleaned up later.
type CopyResult struct {
	Copied []string // absolute paths in the destination that were created
}

// CopyToWorktree copies .env*.local files from repoRoot to worktreeDir,
// skipping any that already exist in the worktree.
func CopyToWorktree(repoRoot, worktreeDir string) (*CopyResult, error) {
	result := &CopyResult{}
	for _, name := range localEnvFiles {
		src := filepath.Join(repoRoot, name)
		dst := filepath.Join(worktreeDir, name)

		// Skip if source doesn't exist
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		// Skip if destination already exists (user managed it manually)
		if _, err := os.Stat(dst); err == nil {
			continue
		}

		if err := copyFile(src, dst); err != nil {
			return result, fmt.Errorf("copying %s: %w", name, err)
		}
		result.Copied = append(result.Copied, dst)
	}
	return result, nil
}

// Cleanup removes only the files that were copied by CopyToWorktree.
func (r *CopyResult) Cleanup() {
	for _, path := range r.Copied {
		_ = os.Remove(path)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
