package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/agentic-camerata/cmt/internal/catalog"
)

// CatalogCmd is the parent command for catalog management.
type CatalogCmd struct {
	Save CatalogSaveCmd `cmd:"" help:"Copy a .md file into the catalog"`
	List CatalogListCmd `cmd:"" help:"List cataloged files"`
	Rm   CatalogRmCmd   `cmd:"rm" help:"Remove a cataloged file"`
	Show CatalogShowCmd `cmd:"" help:"Print a cataloged file's contents"`
	Pick CatalogPickCmd `cmd:"" help:"Open fzf and print the chosen catalog path"`
}

// CatalogSaveCmd copies a file into the catalog. The source file can be given
// directly as the positional File arg or selected via the file picker flags
// (-f/-d/-t/-c); when a picker is used the positional arg is optional.
type CatalogSaveCmd struct {
	FileFlags
	Force bool   `short:"F" help:"Overwrite if the catalog entry already exists"`
	File  string `arg:"" optional:"" help:"Path to the .md file to catalog (optional when a picker flag is used)"`
	Name  string `arg:"" optional:"" help:"Optional name to store it under"`
}

func (c *CatalogSaveCmd) Run(cli *CLI) error {
	picked, err := c.FileFlags.ResolveFiles()
	if err != nil {
		return err
	}

	var sources []string
	if c.File != "" {
		sources = append(sources, c.File)
	}
	sources = append(sources, picked...)

	if len(sources) == 0 {
		return fmt.Errorf("no file specified: pass a path or use a picker flag (-f/-d/-t/-c)")
	}
	if c.Name != "" && len(sources) > 1 {
		return fmt.Errorf("cannot use a custom name when cataloging multiple files")
	}

	for _, src := range sources {
		dest, err := catalog.Save(src, c.Name, c.Force)
		if err != nil {
			return err
		}
		fmt.Printf("Cataloged: %s\n", dest)
	}
	return nil
}

// CatalogListCmd lists cataloged files with metadata.
type CatalogListCmd struct{}

func (c *CatalogListCmd) Run(cli *CLI) error {
	entries, err := catalog.List()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("Catalog is empty")
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ModTime > entries[j].ModTime })
	for _, e := range entries {
		fmt.Printf("%-40s  %8d  %s\n", e.Name, e.Size,
			time.Unix(0, e.ModTime).Format("2006-01-02 15:04"))
	}
	return nil
}

// CatalogRmCmd removes a cataloged file, always selecting it via the fzf picker.
type CatalogRmCmd struct{}

func (c *CatalogRmCmd) Run(cli *CLI) error {
	p, err := catalog.Select()
	if err != nil {
		return err
	}
	if err := catalog.Remove(p); err != nil {
		return err
	}
	fmt.Printf("Removed: %s\n", filepath.Base(p))
	return nil
}

// CatalogShowCmd prints a cataloged file's contents, always selecting the file
// via the fzf picker.
type CatalogShowCmd struct{}

func (c *CatalogShowCmd) Run(cli *CLI) error {
	p, err := catalog.Select()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("read catalog file: %w", err)
	}
	fmt.Print(string(data))
	return nil
}

// CatalogPickCmd opens fzf and prints the chosen absolute path.
type CatalogPickCmd struct{}

func (c *CatalogPickCmd) Run(cli *CLI) error {
	p, err := catalog.Select()
	if err != nil {
		return err
	}
	fmt.Println(p)
	return nil
}
