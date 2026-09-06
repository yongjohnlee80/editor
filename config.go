package editor

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/yongjohnlee80/golib/errs"
)

// Config is the editor's on-disk configuration.
//
// Every field has a working zero-ish default, so a missing config file is not an
// error and neither is a file that sets only one key. LoadFile returns defaults
// when the path does not exist, because "I have not written a config yet" is a
// normal state and not a misconfiguration.
type Config struct {
	Editor   EditorConfig
	Keyboard KeyboardConfig
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
	HorizontalWrap bool
}

// KeyboardConfig is the [keyboard] section.
type KeyboardConfig struct {
	// LeaderKey opens the command line, exactly as ":" does. It exists so the
	// command line is reachable without a shifted key; ":" always works too,
	// so a caller cannot lock themselves out by setting this badly.
	//
	// It is a single key. Defaults to " " (spacebar).
	LeaderKey string
}

// DefaultConfig is the configuration used when no file is found, and the base
// every parsed file is layered onto.
func DefaultConfig() Config {
	return Config{
		Editor:   EditorConfig{HorizontalWrap: false},
		Keyboard: KeyboardConfig{LeaderKey: " "},
	}
}

// LoadFile reads a config file, layering it onto [DefaultConfig].
//
// A MISSING FILE IS NOT AN ERROR: it returns the defaults with a nil error, so
// callers do not have to distinguish "no config" from "empty config". Anything
// else that goes wrong IS an error — an unreadable file, a malformed line, an
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

// Parse reads the config format from r, layering it onto [DefaultConfig].
//
// THIS IS A DELIBERATELY SMALL SUBSET OF TOML, not a TOML parser: section
// headers, key = value, booleans, quoted strings, and # comments. That covers
// the whole documented surface (two sections, two keys) with no dependency.
//
// It is strict about what it does not understand. An unknown section or key is
// an error rather than a silent skip, so a typo surfaces at startup instead of
// as a setting that mysteriously has no effect. If the config grows past what
// this can express — arrays, nested tables, datetimes — replace it with a real
// TOML library rather than extending this.
func Parse(r io.Reader) (Config, error) {
	cfg := DefaultConfig()
	sc := bufio.NewScanner(r)
	section := ""
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(stripComment(sc.Text()))
		if text == "" {
			continue
		}
		if strings.HasPrefix(text, "[") {
			if !strings.HasSuffix(text, "]") {
				return cfg, errs.Wrap(errs.ErrInvalidArgument,
					"line %d: %q opens a section but never closes it", line, text)
			}
			section = strings.TrimSpace(text[1 : len(text)-1])
			if section != "editor" && section != "keyboard" {
				return cfg, errs.Wrap(errs.ErrInvalidArgument,
					"line %d: unknown section [%s] (known: [editor], [keyboard])", line, section)
			}
			continue
		}
		key, raw, ok := strings.Cut(text, "=")
		if !ok {
			return cfg, errs.Wrap(errs.ErrInvalidArgument,
				"line %d: %q is neither a section header nor key = value", line, text)
		}
		key, raw = strings.TrimSpace(key), strings.TrimSpace(raw)
		if section == "" {
			return cfg, errs.Wrap(errs.ErrInvalidArgument,
				"line %d: %q appears before any [section]", line, key)
		}
		if err := assign(&cfg, section, key, raw); err != nil {
			return cfg, errs.Wrap(err, "line %d", line)
		}
	}
	if err := sc.Err(); err != nil {
		return DefaultConfig(), errs.WrapCause(errs.ErrInvalidArgument, err, "editor: reading config")
	}
	return cfg, nil
}

func assign(cfg *Config, section, key, raw string) error {
	switch section + "." + key {
	case "editor.HorizontalWrap":
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return errs.Wrap(errs.ErrInvalidArgument,
				"editor.HorizontalWrap = %s: want true or false", raw)
		}
		cfg.Editor.HorizontalWrap = v
	case "keyboard.LeaderKey":
		v, err := unquote(raw)
		if err != nil {
			return errs.Wrap(err, "keyboard.LeaderKey")
		}
		// One key, because it is compared against a single keypress. A longer
		// string would simply never match, which is a setting that looks
		// applied and is not.
		if len([]rune(v)) != 1 {
			return errs.Wrap(errs.ErrInvalidArgument,
				"keyboard.LeaderKey = %s: want exactly one character, got %d", raw, len([]rune(v)))
		}
		cfg.Keyboard.LeaderKey = v
	default:
		return errs.Wrap(errs.ErrInvalidArgument,
			"unknown key %q in section [%s]", key, section)
	}
	return nil
}

// stripComment removes a trailing # comment, honouring quotes so a '#' inside a
// quoted value survives.
func stripComment(s string) string {
	inQuote := false
	for i, r := range s {
		switch r {
		case '"':
			inQuote = !inQuote
		case '#':
			if !inQuote {
				return s[:i]
			}
		}
	}
	return s
}

func unquote(raw string) (string, error) {
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		v, err := strconv.Unquote(raw)
		if err != nil {
			return "", errs.WrapCause(errs.ErrInvalidArgument, err, "%s is not a valid quoted string", raw)
		}
		return v, nil
	}
	return "", errs.Wrap(errs.ErrInvalidArgument, "%s must be quoted", raw)
}
