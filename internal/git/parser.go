// Package git provides utilities for reading git repository state.
package git

import (
	"os/exec"
	"regexp"
	"strings"
)

// CommitInfo holds parsed information about the latest commit.
type CommitInfo struct {
	Hash    string
	Subject string
	Body    string
	Branch  string
	Stories []string // deduplicated story IDs found in branch + commit
}

// storyPatterns are the regexes tried in order when extracting story IDs.
// Each captures the story ID in group 1.
var storyPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\[([A-Z]{2,10}-\d{1,6})\]`),           // [PROJ-123]
	regexp.MustCompile(`\(([A-Z]{2,10}-\d{1,6})\)`),           // (PROJ-123)
	regexp.MustCompile(`^([A-Z]{2,10}-\d{1,6}):`),             // PROJ-123: at start
	regexp.MustCompile(`(?i)closes?\s+([A-Z]{2,10}-\d{1,6})`), // closes PROJ-123
	regexp.MustCompile(`(?i)refs?\s+([A-Z]{2,10}-\d{1,6})`),  // refs PROJ-123
	regexp.MustCompile(`([A-Z]{2,10}-\d{1,6})`),               // bare PROJ-123 (last resort)
	regexp.MustCompile(`#(\d+)`),                               // #42 (GitHub-style numeric)
}

// ParseLatestCommit returns CommitInfo for HEAD in the current git repo.
func ParseLatestCommit(customPattern string) (*CommitInfo, error) {
	out, err := gitOutput("log", "-1", "--pretty=format:%H%n%s%n%b")
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(out, "\n", 3)
	hash := strings.TrimSpace(parts[0])
	subject := ""
	if len(parts) > 1 {
		subject = strings.TrimSpace(parts[1])
	}
	body := ""
	if len(parts) > 2 {
		body = strings.TrimSpace(parts[2])
	}

	branch, err := gitOutput("branch", "--show-current")
	if err != nil {
		branch = "" // detached HEAD – not fatal
	}
	branch = strings.TrimSpace(branch)

	var patterns []*regexp.Regexp
	if customPattern != "" {
		p, err := regexp.Compile(customPattern)
		if err == nil {
			patterns = append([]*regexp.Regexp{p}, storyPatterns...)
		}
	}
	if len(patterns) == 0 {
		patterns = storyPatterns
	}

	return &CommitInfo{
		Hash:    hash,
		Subject: subject,
		Body:    body,
		Branch:  branch,
		Stories: extractStories(patterns, branch, subject, body),
	}, nil
}

// extractStories finds all story IDs in text, deduplicating while preserving order.
func extractStories(patterns []*regexp.Regexp, texts ...string) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, text := range texts {
		for _, p := range patterns {
			matches := p.FindAllStringSubmatch(text, -1)
			for _, m := range matches {
				if len(m) < 2 {
					continue
				}
				id := m[1]
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}
	return ids
}

// gitOutput runs a git command and returns its trimmed stdout.
func gitOutput(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
