package editor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yongjohnlee80/golib/errs"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	c := DefaultConfig()
	if c.Editor.HorizontalWrap {
		t.Error("HorizontalWrap must default to false")
	}
	if c.Keyboard.LeaderKey != " " {
		t.Errorf("LeaderKey = %q, want a spacebar", c.Keyboard.LeaderKey)
	}
}

func TestParse_Accepts(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		in        string
		wrap      bool
		lead      string
		placement string
	}{
		"empty file keeps every default": {"", false, " ", "top"},
		"all sections": {`
[editor]
HorizontalWrap = true

[keyboard]
LeaderKey = ","

[menu]
Placement = "left"
`, true, ",", "left"},
		// A file that sets ONE key must not reset the other sections: the
		// parse layers onto the defaults rather than replacing them.
		"partial file layers onto defaults": {"[editor]\nHorizontalWrap = true\n", true, " ", "top"},
		"comments and blank lines":          {"# top\n\n[editor]  # section\nHorizontalWrap = false\n", false, " ", "top"},
		// '#' inside quotes is a value, not a comment.
		"hash inside a quoted value": {"[keyboard]\nLeaderKey = \"#\"\n", false, "#", "top"},
		"whitespace is tolerated":    {"  [editor]  \n  HorizontalWrap   =   true  \n", true, " ", "top"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(strings.NewReader(tc.in))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got.Editor.HorizontalWrap != tc.wrap {
				t.Errorf("HorizontalWrap = %v, want %v", got.Editor.HorizontalWrap, tc.wrap)
			}
			if got.Keyboard.LeaderKey != tc.lead {
				t.Errorf("LeaderKey = %q, want %q", got.Keyboard.LeaderKey, tc.lead)
			}
			if got.Menu.Placement != tc.placement {
				t.Errorf("Placement = %q, want %q", got.Menu.Placement, tc.placement)
			}
		})
	}
}

// Parse rejects unknown keys and malformed values: silently ignoring an unrecognised
// key is worse than refusing to start, because the setting looks applied.
func TestParse_Refuses(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]string{
		"unknown section":        "[editr]\nHorizontalWrap = true\n",
		"unknown key":            "[editor]\nHorizontalWrp = true\n",
		"key before any section": "HorizontalWrap = true\n",
		"unclosed section":       "[editor\n",
		"neither header nor kv":  "[editor]\nHorizontalWrap\n",
		"non-boolean wrap":       "[editor]\nHorizontalWrap = yes-please\n",
		"unquoted leader":        "[keyboard]\nLeaderKey = x\n",
		"multi-rune leader":      "[keyboard]\nLeaderKey = \"gg\"\n",
		"empty leader":           "[keyboard]\nLeaderKey = \"\"\n",
		"invalid menu placement": "[menu]\nPlacement = \"floating\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse(strings.NewReader(in))
			if err == nil {
				t.Fatalf("Parse(%q) must fail", in)
			}
			if !errors.Is(err, errs.ErrInvalidArgument) {
				t.Errorf("err = %v, want it to match errs.ErrInvalidArgument", err)
			}
		})
	}
}

// A multi-rune leader is refused for a concrete reason: it is compared against
// one keypress, so it could never match. Pinned separately because "it looks
// applied and is not" is exactly the failure the strictness exists to prevent.
func TestParse_MultiRuneLeaderSaysWhy(t *testing.T) {
	t.Parallel()
	_, err := Parse(strings.NewReader("[keyboard]\nLeaderKey = \"gg\"\n"))
	if err == nil || !strings.Contains(err.Error(), "exactly one character") {
		t.Errorf("err = %v, want it to name the one-character rule", err)
	}
}

// A MISSING file is not an error — "I have not written a config yet" is a
// normal state, and LoadFile returns the defaults.
func TestLoadFile_MissingIsNotAnError(t *testing.T) {
	t.Parallel()
	got, err := LoadFile(filepath.Join(t.TempDir(), "absent.toml"))
	if err != nil {
		t.Fatalf("a missing config must not be an error: %v", err)
	}
	if got != DefaultConfig() {
		t.Errorf("got %+v, want the defaults", got)
	}
}

func TestLoadFile_ReadsAFile(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "editor.toml")
	if err := os.WriteFile(p, []byte("[editor]\nHorizontalWrap = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Editor.HorizontalWrap {
		t.Error("HorizontalWrap did not come from the file")
	}
}

// A malformed file NAMES ITS LINE. Without that, a config error is a hunt.
func TestLoadFile_ErrorNamesPathAndLine(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "editor.toml")
	if err := os.WriteFile(p, []byte("[editor]\nHorizontalWrap = true\nnope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFile(p)
	if err == nil {
		t.Fatal("a malformed config must fail")
	}
	if !strings.Contains(err.Error(), p) || !strings.Contains(err.Error(), "line 3") {
		t.Errorf("err = %v, want it to name %s and line 3", err, p)
	}
}

// The example file shipped in the repo must parse, and must describe the
// DEFAULTS — otherwise it documents a configuration nobody runs.
func TestExampleConfigParsesAndMatchesDefaults(t *testing.T) {
	t.Parallel()
	f, err := os.Open("editor.example.toml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := Parse(f)
	if err != nil {
		t.Fatalf("editor.example.toml does not parse: %v", err)
	}
	if got != DefaultConfig() {
		t.Errorf("editor.example.toml = %+v, want it to state the defaults %+v", got, DefaultConfig())
	}
}
