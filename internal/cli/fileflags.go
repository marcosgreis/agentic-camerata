package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/agentic-camerata/cmt/internal/plans"
)

// FileFlags provides file selection flags for commands
type FileFlags struct {
	Files    []string `short:"f" help:"File path to prepend to prompt (repeatable)" optional:""`
	Dirs     []string `short:"d" help:"Directory to open fzf file selector on (repeatable)" optional:""`
	Thoughts int      `short:"t" help:"Open fzf on thoughts/shared/ directory (repeatable)" type:"counter" optional:""`
}

// ResolveFiles processes all file flags and returns the list of file paths
// Files are processed in order: -f files, then -d directories, then -t defaults
func (f *FileFlags) ResolveFiles() ([]string, error) {
	var resolved []string

	// Process -f flags (direct file paths)
	for _, file := range f.Files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", file)
		}
		resolved = append(resolved, file)
	}

	// Process -d flags (fzf on custom directories)
	for _, dir := range f.Dirs {
		path, err := plans.SelectMarkdownFile(dir)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, path)
	}

	// Process -t flags (fzf on thoughts/shared/ directory)
	for i := 0; i < f.Thoughts; i++ {
		path, err := plans.SelectMarkdownFile(plans.DefaultDir)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, path)
	}

	return resolved, nil
}

// PrependFilesToTask prepends file paths to the task description
func PrependFilesToTask(files []string, task string) string {
	if len(files) == 0 {
		return task
	}

	parts := append(files, task)
	return strings.Join(parts, " ")
}
