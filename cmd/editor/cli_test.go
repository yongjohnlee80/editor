package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The bug this pins: subcommands calls SetFlags AFTER main has seeded
// ConfigPath from the top-level -config, so a hard-coded "" default erased it
// and `editor -config X FILE` silently ignored X. The default must be the
// field's current value.
func TestBase_SetFlagsKeepsASeededConfigPath(t *testing.T) {
	t.Parallel()
	cmd := &Base{ConfigPath: "/seeded/editor.toml"}
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	cmd.SetFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if cmd.ConfigPath != "/seeded/editor.toml" {
		t.Errorf("ConfigPath = %q, want the seeded value to survive SetFlags+Parse", cmd.ConfigPath)
	}
}

// An explicit verb-level -config still WINS over a seeded one, because it is
// parsed after. Paired with the test above so neither can pass alone.
func TestBase_ExplicitFlagBeatsASeededConfigPath(t *testing.T) {
	t.Parallel()
	cmd := &Base{ConfigPath: "/seeded/editor.toml"}
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	cmd.SetFlags(fs)
	if err := fs.Parse([]string{"-config", "/explicit/editor.toml"}); err != nil {
		t.Fatal(err)
	}
	if cmd.ConfigPath != "/explicit/editor.toml" {
		t.Errorf("ConfigPath = %q, want the explicit flag to win", cmd.ConfigPath)
	}
}

func TestResolveConfig_Precedence(t *testing.T) {
	// Not parallel: it sets an env var and the working directory.
	dir := t.TempDir()
	t.Chdir(dir)

	explicit := filepath.Join(dir, "explicit.toml")
	env := filepath.Join(dir, "env.toml")

	// An EXPLICIT path wins, and is returned WITHOUT an existence check — a
	// path the user named must report its own read error rather than being
	// skipped for the next candidate.
	t.Setenv("EDITOR_CONFIG", env)
	cmd := &Base{ConfigPath: explicit}
	if got := cmd.resolveConfig(); got != explicit {
		t.Errorf("resolveConfig = %q, want the explicit %q", got, explicit)
	}

	// Then the environment.
	cmd = &Base{}
	if got := cmd.resolveConfig(); got != env {
		t.Errorf("resolveConfig = %q, want the env %q", got, env)
	}

	// Then ./editor.toml, but only if it EXISTS.
	t.Setenv("EDITOR_CONFIG", "")
	if err := os.WriteFile("editor.toml", []byte("[editor]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := cmd.resolveConfig(); got != "editor.toml" {
		t.Errorf("resolveConfig = %q, want the local editor.toml", got)
	}

	// With none of those, it falls back under the user config dir — never to
	// the empty string, which would make LoadFile open the current directory.
	if err := os.Remove("editor.toml"); err != nil {
		t.Fatal(err)
	}
	got := cmd.resolveConfig()
	if got == "" {
		t.Error("resolveConfig returned an empty path")
	}
	if !strings.HasSuffix(got, filepath.Join("editor", "editor.toml")) && got != "editor.toml" {
		t.Errorf("resolveConfig = %q, want a path under the user config dir", got)
	}
}

// isVerb must recognise every REGISTERED command, because anything it does not
// recognise is treated as a filename. It is derived from the commander rather
// than a hard-coded list precisely so a new verb cannot be missed.
func TestIsVerb(t *testing.T) {
	registerCommands(&CmdOpen{})
	for _, name := range []string{OpenName, VersionName, "help", "flags"} {
		if !isVerb(name) {
			t.Errorf("isVerb(%q) = false, want true — it would be opened as a file", name)
		}
	}
	for _, name := range []string{"notes.md", "open.md", "versions", "README"} {
		if isVerb(name) {
			t.Errorf("isVerb(%q) = true, want false — a filename must not be taken for a verb", name)
		}
	}
}
