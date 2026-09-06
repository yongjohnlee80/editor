// Command editor is a small modal text editor on golib/tui.
//
//	editor [-config PATH] [FILE]
//
// With no FILE it opens an unnamed buffer; ":w NAME" names it. Configuration is
// read from -config, else $EDITOR_CONFIG, else ./editor.toml, else
// $XDG_CONFIG_HOME/editor/editor.toml. A missing config is not an error.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/yongjohnlee80/editor"
	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/term"
)

func main() {
	cfgPath := flag.String("config", "", "path to editor.toml (default: search)")
	flag.Parse()

	if err := run(*cfgPath, flag.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, "editor:", err)
		os.Exit(1)
	}
}

func run(cfgPath, file string) error {
	cfg, err := editor.LoadFile(resolveConfig(cfgPath))
	if err != nil {
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

	app, err := editor.New(cfg, file, stop)
	if err != nil {
		return err
	}
	if err := tui.NewApp(app, tui.WithBackend(backend)).Run(ctx); err != nil &&
		!errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// resolveConfig picks the first candidate that is set. It does NOT check for
// existence: a path the user named explicitly should report its own read error
// rather than being silently skipped for the next candidate.
func resolveConfig(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if p := os.Getenv("EDITOR_CONFIG"); p != "" {
		return p
	}
	if _, err := os.Stat("editor.toml"); err == nil {
		return "editor.toml"
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "editor.toml"
	}
	return filepath.Join(dir, "editor", "editor.toml")
}
