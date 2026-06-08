package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// VenueCmd is the parent command for venue management
type VenueCmd struct {
	Add    VenueAddCmd    `cmd:"" help:"Pin a directory as a venue"`
	List   VenueListCmd   `cmd:"" help:"List pinned venues"`
	Remove VenueRemoveCmd `cmd:"" help:"Remove a pinned venue"`
}

// VenueAddCmd pins a directory as a venue
type VenueAddCmd struct {
	Folder string `arg:"" help:"Directory to pin as a venue"`
}

func (c *VenueAddCmd) Run(cli *CLI) error {
	dir, err := filepath.Abs(c.Folder)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("directory not found: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", dir)
	}

	if err := cli.Database().AddVenue(dir); err != nil {
		return err
	}

	fmt.Printf("Pinned venue: %s\n", dir)
	return nil
}

// VenueListCmd lists pinned venues
type VenueListCmd struct{}

func (c *VenueListCmd) Run(cli *CLI) error {
	venues, err := cli.Database().ListVenues()
	if err != nil {
		return err
	}

	if len(venues) == 0 {
		fmt.Println("No pinned venues")
		return nil
	}

	for _, v := range venues {
		fmt.Println(v.Directory)
	}
	return nil
}

// VenueRemoveCmd removes a pinned venue
type VenueRemoveCmd struct {
	Folder string `arg:"" help:"Directory to unpin"`
}

func (c *VenueRemoveCmd) Run(cli *CLI) error {
	dir, err := filepath.Abs(c.Folder)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	if err := cli.Database().RemoveVenue(dir); err != nil {
		return err
	}

	fmt.Printf("Removed venue: %s\n", dir)
	return nil
}
