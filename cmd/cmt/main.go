package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"

	"github.com/agentic-camerata/cmt/internal/cli"
	"github.com/agentic-camerata/cmt/internal/db"
	"github.com/agentic-camerata/cmt/internal/tmux"
)

var version = "dev"

func main() {
	var c cli.CLI

	ctx := kong.Parse(&c,
		kong.Name("cmt"),
		kong.Description("Claude Management Tool - Orchestrate Claude AI coding sessions"),
		kong.UsageOnError(),
		kong.Vars{
			"version": version,
		},
	)

	// Check tmux requirement
	if err := tmux.RequireTmux(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "cmt requires running inside a tmux session.")
		fmt.Fprintln(os.Stderr, "Start tmux with: tmux new-session -s main")
		os.Exit(1)
	}

	// Open database
	database, err := db.Open(c.DB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	c.SetDatabase(database)

	// Run command
	if err := ctx.Run(&c); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
