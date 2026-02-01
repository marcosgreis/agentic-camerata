package plans

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const PlansDir = "thoughts/shared/plans"
const DefaultDir = "thoughts/shared"

// SelectMarkdownFile opens fzf to let the user select a markdown file from the given directory (recursive)
func SelectMarkdownFile(dir string) (string, error) {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("directory not found: %s", dir)
	}

	// Find all .md files recursively
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk directory: %w", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no .md files found in %s", dir)
	}

	// Sort files by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		fi, _ := os.Stat(files[i])
		fj, _ := os.Stat(files[j])
		if fi == nil || fj == nil {
			return files[i] > files[j]
		}
		return fi.ModTime().After(fj.ModTime())
	})

	// Check if fzf is available
	if _, err := exec.LookPath("fzf"); err != nil {
		return "", fmt.Errorf("fzf is required but not installed. Install with: brew install fzf")
	}

	// Prepare display names (relative path from dir)
	displayNames := make([]string, len(files))
	for i, f := range files {
		rel, err := filepath.Rel(dir, f)
		if err != nil {
			displayNames[i] = filepath.Base(f)
		} else {
			displayNames[i] = rel
		}
	}

	// Run fzf
	cmd := exec.Command("fzf",
		"--header", fmt.Sprintf("Select file from %s:", dir),
		"--header-first",
	)

	cmd.Stdin = strings.NewReader(strings.Join(displayNames, "\n"))
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return "", fmt.Errorf("no file selected")
		}
		return "", fmt.Errorf("fzf failed: %w", err)
	}

	selected := strings.TrimSpace(string(output))
	if selected == "" {
		return "", fmt.Errorf("no file selected")
	}

	return filepath.Join(dir, selected), nil
}

// SelectPlanFile opens fzf to let the user select a plan file
// This is kept for backwards compatibility with the implement command
func SelectPlanFile() (string, error) {
	return SelectMarkdownFile(PlansDir)
}
