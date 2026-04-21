package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	sentinelStart = "# --- ishtrak hook start ---"
	sentinelEnd   = "# --- ishtrak hook end ---"
)

// hookScript is the content injected by ishtrak hook install.
const hookScript = `
` + sentinelStart + `
ishtrak git-event --hook=%s 2>/dev/null || true
` + sentinelEnd

// HookPath returns the absolute path for the named git hook in the repo
// at repoRoot.
func HookPath(repoRoot, hookName string) string {
	return filepath.Join(repoRoot, ".git", "hooks", hookName)
}

// RepoRoot returns the root directory of the git repo containing the
// current working directory.
func RepoRoot() (string, error) {
	out, err := gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(out), nil
}

// Install writes (or appends) the ishtrak hook snippet into hookPath.
// If the file already exists it appends; if not it creates a new hook file.
func Install(hookPath, hookName string) error {
	snippet := fmt.Sprintf(hookScript, hookName)

	existing, err := os.ReadFile(hookPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read hook file: %w", err)
	}

	content := string(existing)

	// Already installed – nothing to do.
	if strings.Contains(content, sentinelStart) {
		return nil
	}

	var newContent string
	if content == "" {
		// New file: write shebang + snippet.
		newContent = "#!/usr/bin/env sh\n" + snippet + "\n"
	} else {
		// Append to existing file.
		newContent = strings.TrimRight(content, "\n") + "\n" + snippet + "\n"
	}

	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	if err := os.WriteFile(hookPath, []byte(newContent), 0o755); err != nil {
		return fmt.Errorf("write hook file: %w", err)
	}
	return nil
}

// Uninstall removes the ishtrak sentinel block from hookPath.
// If nothing remains after removal the file is deleted.
func Uninstall(hookPath string) error {
	data, err := os.ReadFile(hookPath)
	if os.IsNotExist(err) {
		return nil // nothing to do
	}
	if err != nil {
		return fmt.Errorf("read hook file: %w", err)
	}

	content := string(data)
	if !strings.Contains(content, sentinelStart) {
		return nil // ishtrak block not present
	}

	// Remove lines between (and including) sentinels.
	var out []string
	inside := false
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == sentinelStart {
			inside = true
			continue
		}
		if strings.TrimSpace(line) == sentinelEnd {
			inside = false
			continue
		}
		if !inside {
			out = append(out, line)
		}
	}

	result := strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"

	// If only the shebang remains, delete the file entirely.
	trimmed := strings.TrimSpace(strings.Replace(result, "#!/usr/bin/env sh", "", 1))
	if trimmed == "" {
		return os.Remove(hookPath)
	}

	return os.WriteFile(hookPath, []byte(result), 0o755)
}
