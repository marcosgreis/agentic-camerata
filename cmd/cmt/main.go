package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"

	"github.com/agentic-camerata/cmt/internal/cli"
	"github.com/agentic-camerata/cmt/internal/db"
)

var version = "dev"

func main() {
	var c cli.CLI

	ctx := kong.Parse(&c,
		kong.Name("cmt"),
		kong.Description("Agentic Camerata - Orchestrate Claude AI coding sessions"),
		kong.UsageOnError(),
		kong.Vars{
			"version": version,
		},
	)

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
