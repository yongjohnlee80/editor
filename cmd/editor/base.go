package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/yongjohnlee80/editor"
)

// Base holds what every command shares: where the config came from, and the
// loaded config itself. Following lm/cmd/cli-v2, the shared flags live here and
// each command embeds it.
type Base struct {
	ConfigPath string
	Config     editor.Config
}

// SetFlags adds the flags common to every command.
//
// The DEFAULT is the field's current value, not "". subcommands calls SetFlags
// after main has run, so a hard-coded "" default silently overwrote a
// ConfigPath that main had already seeded from the top-level -config — the bare
// `editor -config X FILE` form parsed its flag, stored it, and then had it
// erased before Execute. Seeding the default instead keeps both forms working,
// and an explicit verb-level -config still wins because it is parsed after.
func (cmd *Base) SetFlags(flags *flag.FlagSet) {
	flags.StringVar(&cmd.ConfigPath, "config", cmd.ConfigPath, "path to editor.toml (default: search)")
}

// LoadConfig resolves and reads the config. A missing file is not an error.
func (cmd *Base) LoadConfig() error {
	cfg, err := editor.LoadFile(cmd.resolveConfig())
	if err != nil {
		return err
	}
	cmd.Config = cfg
	return nil
}

// resolveConfig picks the first candidate that is set.
//
// An EXPLICIT -config is returned without checking that it exists, so a path
// the user named reports its own read error rather than being silently skipped
// for the next candidate — a typo there should fail, not fall back.
func (cmd *Base) resolveConfig() string {
	if cmd.ConfigPath != "" {
		return cmd.ConfigPath
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
