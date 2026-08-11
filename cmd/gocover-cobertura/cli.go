package main

import (
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kong"
	cobertura "github.com/blackbirdworks/gocover-cobertura"
)

// CLI represents the command line configuration for gocover-cobertura.
type CLI struct {
	Input  string `default:"-" help:"Input coverage file path (default '-' for stdin)"        short:"i"`
	Output string `default:"-" help:"Output Cobertura XML file path (default '-' for stdout)" short:"o"`
}

// Run executes the CLI logic.
func (c *CLI) Run() error {
	var in io.Reader
	if c.Input == "-" || c.Input == "" {
		in = os.Stdin
	} else {
		f, err := os.Open(c.Input)
		if err != nil {
			return fmt.Errorf("failed to open input file %q: %w", c.Input, err)
		}
		defer f.Close()
		in = f
	}

	var out io.Writer
	if c.Output == "-" || c.Output == "" {
		out = os.Stdout
	} else {
		f, err := os.Create(c.Output)
		if err != nil {
			return fmt.Errorf("failed to create output file %q: %w", c.Output, err)
		}
		defer f.Close()
		out = f
	}

	if err := cobertura.Convert(in, out); err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}

	return nil
}

// RunCLI initializes Kong and executes the CLI application.
func RunCLI() {
	var cli CLI
	kctx := kong.Parse(&cli,
		kong.Name("gocover-cobertura"),
		kong.Description("Convert Go cover profile to Cobertura XML format"),
	)
	kctx.FatalIfErrorf(cli.Run())
}
