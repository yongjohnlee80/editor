package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/subcommands"
	"github.com/yongjohnlee80/editor"
	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/term"
)

// OpenName is the verb. It is also the command main falls back to, so
// `editor FILE` and `editor open FILE` do the same thing.
const OpenName = "open"

// CmdOpen runs the editor.
type CmdOpen struct {
	Base
}

func (c *CmdOpen) Name() string { return OpenName }

func (c *CmdOpen) Synopsis() string {
	return "Open FILE in the editor (the default when no verb is given)."
}

func (c *CmdOpen) Usage() string {
	return `open [-config PATH] [FILE]:
  Open FILE, or an unnamed buffer when FILE is omitted. A path that does not
  exist yet is a NEW file: it opens empty and ":w" creates it.

  Since open is the default verb, "editor FILE" is the same as
  "editor open FILE".

`
}

func (c *CmdOpen) Execute(_ context.Context, f *flag.FlagSet, _ ...any) subcommands.ExitStatus {
	if f.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "editor: open takes at most one FILE")
		return subcommands.ExitUsageError
	}
	if err := c.run(f.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, "editor:", err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

func (c *CmdOpen) run(file string) error {
	if err := c.LoadConfig(); err != nil {
		return err
	}

	// SIGINT/SIGTERM end the app the same way ":q" does, so a terminal is
	// never left in raw mode by a signal.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	backend, err := term.Open()
	if err != nil {
		return err
	}
	app, err := editor.New(c.Config, file, stop)
	if err != nil {
		return err
	}
	if err := tui.NewApp(app, tui.WithBackend(backend)).Run(ctx); err != nil &&
		!errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
