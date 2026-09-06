package editor

import (
	"io"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/yongjohnlee80/golib/errs"
)

// Config is the editor's on-disk configuration.
//
// Every field has a working default, so a missing config file is not an error
// and neither is a file that sets only one key: parsing decodes ONTO
// [DefaultConfig], so absent keys keep their default rather than becoming a
// zero value.
type Config struct {
	Editor   EditorConfig   `toml:"editor"`
	Keyboard KeyboardConfig `toml:"keyboard"`
}

// EditorConfig is the [editor] section.
type EditorConfig struct {
	// HorizontalWrap soft-wraps long lines to the panel width.
	//
	// It also HIDES the horizontal scroll indicator, and that is a consequence
	// rather than a second setting: wrapped text has no horizontal extent to
	// scroll, so an indicator would be reporting a dimension that does not
	// exist. Defaults to false, which keeps long lines on one row and scrolls
	// horizontally instead.
	HorizontalWrap bool `toml:"HorizontalWrap"`
}

// KeyboardConfig is the [keyboard] section.
type KeyboardConfig struct {
	// LeaderKey opens the command line, exactly as ":" does. It exists so the
	// command line is reachable without a shifted key; ":" always works too,
	// so a caller cannot lock themselves out by setting this badly.
	//
	// It is a single key. Defaults to " " (spacebar).
	LeaderKey string `toml:"LeaderKey"`
}

// DefaultConfig is the configuration used when no file is found, and the base
// every parsed file is decoded onto.
func DefaultConfig() Config {
	return Config{
		Editor:   EditorConfig{HorizontalWrap: false},
		Keyboard: KeyboardConfig{LeaderKey: " "},
	}
}

// LoadFile reads a TOML config file, decoding it onto [DefaultConfig].
//
// A MISSING FILE IS NOT AN ERROR: it returns the defaults with a nil error, so
// callers do not have to distinguish "no config" from "empty config". Anything
// else that goes wrong IS an error — an unreadable file, malformed TOML, an
// unknown key — because silently ignoring a setting the user wrote is worse
// than refusing to start.
func LoadFile(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), errs.WrapCause(errs.ErrInvalidArgument, err,
			"editor: reading config %s", path)
	}
	defer f.Close()
	cfg, err := Parse(f)
	if err != nil {
		return DefaultConfig(), errs.Wrap(err, "editor: config %s", path)
	}
	return cfg, nil
}

// Parse decodes TOML from r onto [DefaultConfig].
//
// IT IS STRICT ABOUT KEYS IT DOES NOT KNOW. An unrecognised key or section is
// an error rather than a silent skip, so a typo surfaces at startup instead of
// as a setting that mysteriously has no effect — the failure mode where the
// config LOOKS applied is the one worth refusing.
func Parse(r io.Reader) (Config, error) {
	cfg := DefaultConfig()
	md, err := toml.NewDecoder(r).Decode(&cfg)
	if err != nil {
		return DefaultConfig(), errs.WrapCause(errs.ErrInvalidArgument, err, "invalid TOML")
	}
	if undec := md.Undecoded(); len(undec) > 0 {
		names := make([]string, 0, len(undec))
		for _, k := range undec {
			names = append(names, k.String())
		}
		return DefaultConfig(), errs.Wrap(errs.ErrInvalidArgument,
			"unknown %s: %s (known: editor.HorizontalWrap, keyboard.LeaderKey)",
			plural("key", len(names)), strings.Join(names, ", "))
	}
	if err := cfg.validate(); err != nil {
		return DefaultConfig(), err
	}
	return cfg, nil
}

// validate rejects values that decode fine but cannot work.
func (c Config) validate() error {
	// One key, because it is compared against a single keypress. A longer
	// string would simply never match, which is a setting that looks applied
	// and is not.
	if n := len([]rune(c.Keyboard.LeaderKey)); n != 1 {
		return errs.Wrap(errs.ErrInvalidArgument,
			"keyboard.LeaderKey = %q: want exactly one character, got %d",
			c.Keyboard.LeaderKey, n)
	}
	return nil
}

func plural(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
