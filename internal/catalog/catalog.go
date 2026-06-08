package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentic-camerata/cmt/internal/plans"
)

// EnvDir is the environment variable that overrides the catalog directory.
const EnvDir = "CMT_CATALOG_DIR"

// Entry describes a cataloged file.
type Entry struct {
	Name    string
	Size    int64
	ModTime int64 // unix nano; formatted by callers
}

// Dir returns the absolute catalog directory (CMT_CATALOG_DIR or the default
// ~/.agentic-camerata/catalog), with a leading ~ expanded. It does not create
// the directory.
func Dir() (string, error) {
	dir := os.Getenv(EnvDir)
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		return filepath.Join(home, ".agentic-camerata", "catalog"), nil
	}
	if strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		dir = filepath.Join(home, dir[2:])
	}
	return filepath.Abs(dir)
}

// Save copies srcPath into the catalog. If name is empty, the source basename is
// used; otherwise name (basename only, .md enforced) is used. Errors if the
// destination exists unless force is true.
func Save(srcPath, name string, force bool) (string, error) {
	if !strings.HasSuffix(srcPath, ".md") {
		return "", fmt.Errorf("only .md files can be cataloged: %s", srcPath)
	}
	info, err := os.Stat(srcPath)
	if err != nil {
		return "", fmt.Errorf("source file not found: %s", srcPath)
	}
	if info.IsDir() {
		return "", fmt.Errorf("not a file: %s", srcPath)
	}

	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create catalog directory: %w", err)
	}

	destName := filepath.Base(srcPath)
	if name != "" {
		destName = filepath.Base(name)
		if !strings.HasSuffix(destName, ".md") {
			destName += ".md"
		}
	}
	dest := filepath.Join(dir, destName)

	if _, err := os.Stat(dest); err == nil && !force {
		return "", fmt.Errorf("%q already exists in catalog (use --force to overwrite)", destName)
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read source: %w", err)
	}
	if err := os.WriteFile(dest, data, 0644); err != nil {
		return "", fmt.Errorf("write catalog file: %w", err)
	}
	return dest, nil
}

// List returns the .md entries in the catalog (flat). Missing dir => empty.
func List() ([]Entry, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read catalog directory: %w", err)
	}
	var out []Entry
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		info, err := de.Info()
		if err != nil {
			continue
		}
		out = append(out, Entry{Name: de.Name(), Size: info.Size(), ModTime: info.ModTime().UnixNano()})
	}
	return out, nil
}

// Path resolves a catalog entry name to its absolute path, erroring if absent.
func Path(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	base := filepath.Base(name)
	if !strings.HasSuffix(base, ".md") {
		base += ".md"
	}
	p := filepath.Join(dir, base)
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("%q not found in catalog", name)
	}
	return p, nil
}

// Remove deletes a catalog entry by name.
func Remove(name string) error {
	p, err := Path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		return fmt.Errorf("remove catalog file: %w", err)
	}
	return nil
}

// Select opens fzf on the catalog directory and returns the chosen absolute path.
func Select() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return plans.SelectMarkdownFile(dir)
}
