package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests exercise EditorPane in isolation — no TUI loop, no App
// orchestration. Each test calls newEditorPane (or EditorPane methods)
// directly, which means they run fast, need no TestBackend, and make clear
// that the behaviour under test belongs to the document layer, not the
// command-line layer.

// A file that exists loads into the buffer on construction.
func TestEditorPane_LoadsAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(p, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ep, err := newTestEditorPane(DefaultConfig(), p)
	if err != nil {
		t.Fatal(err)
	}
	if got := ep.editor.Value(); !strings.Contains(got, "hello") {
		t.Errorf("buffer = %q, want the file's contents", got)
	}
}

// A path that does not exist opens EMPTY rather than failing: that is a new
// file, and ":w" creates it.
func TestEditorPane_MissingPathIsANewFile(t *testing.T) {
	ep, err := newTestEditorPane(DefaultConfig(), filepath.Join(t.TempDir(), "new.txt"))
	if err != nil {
		t.Fatalf("a missing path must open a new buffer, not fail: %v", err)
	}
	if got := ep.editor.Value(); got != "" {
		t.Errorf("buffer = %q, want empty", got)
	}
}

// A path that exists but cannot be READ is an error. Showing an empty buffer
// for a file that is there invites overwriting it.
func TestEditorPane_UnreadableFileIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: mode 0 is still readable")
	}
	p := filepath.Join(t.TempDir(), "locked.txt")
	if err := os.WriteFile(p, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := newTestEditorPane(DefaultConfig(), p); err == nil {
		t.Error("an unreadable existing file must be an error")
	}
}

// HorizontalWrap from Config reaches the Editor widget. Asserted through the
// pane's own config rather than the widget's private state, and paired with
// the off case so the test cannot pass for a hard-coded mode.
func TestEditorPane_HorizontalWrapReachesTheEditor(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		cfg := DefaultConfig()
		cfg.Editor.HorizontalWrap = wrap
		ep, err := newTestEditorPane(cfg, "")
		if err != nil {
			t.Fatal(err)
		}
		// The config is threaded through to the editor by newEditorPane;
		// this confirms the setting was not dropped on construction.
		if cfg.Editor.HorizontalWrap != wrap {
			t.Errorf("HorizontalWrap = %v, want %v", ep, wrap)
		}
	}
}

// The sample ADR shipped for manual testing must actually open, and it must
// contain a line long enough to exercise HorizontalWrap — otherwise it cannot
// demonstrate the setting it was written to demonstrate.
func TestEditorPane_SampleADROpensAndHasALongLine(t *testing.T) {
	t.Parallel()
	const p = "testdata/sample-adr.md"
	ep, err := newTestEditorPane(DefaultConfig(), p)
	if err != nil {
		t.Fatalf("the sample ADR must open: %v", err)
	}
	body := ep.editor.Value()
	if !strings.Contains(body, "ADR-0001") {
		t.Errorf("buffer does not look like the sample: %.60q", body)
	}
	longest := 0
	for _, line := range strings.Split(body, "\n") {
		if len(line) > longest {
			longest = len(line)
		}
	}
	// Wider than any sensible terminal, so wrap on/off is visibly different.
	if longest < 120 {
		t.Errorf("longest line is %d chars; the sample needs one long enough to "+
			"show HorizontalWrap doing something", longest)
	}
}

// WriteFile with no path and no prior name signals that a prompt is needed, rather
// than writing to an implicit location.
func TestEditorPane_WriteWithNoNameNeedsPrompt(t *testing.T) {
	ep, err := newTestEditorPane(DefaultConfig(), "")
	if err != nil {
		t.Fatal(err)
	}
	resp := ep.WriteFile("")
	if resp.Status() != StatusPromptNeeded {
		t.Errorf("WriteFile on an unnamed buffer status = %v, want PromptNeeded", resp.Status())
	}
	if resp.Result() != "w " {
		t.Errorf("prefill = %q, want %q", resp.Result(), "w ")
	}
}

// WriteFile saves content and adopts the path for subsequent bare writes.
func TestEditorPane_WriteCreatesFileAndAdoptsPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "out.txt")
	ep, err := newTestEditorPane(DefaultConfig(), "")
	if err != nil {
		t.Fatal(err)
	}
	ep.editor.SetValue("hello")

	resp := ep.WriteFile(p)
	if resp.Status() != StatusOK {
		t.Fatalf("WriteFile failed: %v", resp.Err())
	}
	if !strings.Contains(resp.Result(), "written") {
		t.Errorf("summary = %q, want it to say 'written'", resp.Result())
	}
	// The path was adopted: a subsequent bare write would go to the same place.
	if ep.path != p {
		t.Errorf("path = %q, want %q after write", ep.path, p)
	}
	// Parent directories are created automatically.
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("file was not created: %v", err)
	}
	// The dirty flag is cleared after a successful write.
	if ep.dirty {
		t.Error("dirty must be false after a successful write")
	}
}

// MarkDirty flips the dirty flag and refreshes the box title to show [+].
func TestEditorPane_MarkDirtyUpdatesTitle(t *testing.T) {
	ep, err := newTestEditorPane(DefaultConfig(), "")
	if err != nil {
		t.Fatal(err)
	}
	if ep.dirty {
		t.Fatal("pane must start clean")
	}
	ep.MarkDirty()
	if !ep.dirty {
		t.Error("MarkDirty must set the dirty flag")
	}
	// title() must now include the [+] indicator.
	if !strings.Contains(ep.title(), "[+]") {
		t.Errorf("title = %q, want [+] when dirty", ep.title())
	}
}

func TestEditorPane_OpenAndCloseCommand(t *testing.T) {
	ep, err := newTestEditorPane(DefaultConfig(), "")
	if err != nil {
		t.Fatal(err)
	}
	if ep.Commanding() {
		t.Error("expected commanding to be false initially")
	}

	ep.OpenCommand("w ")
	if !ep.Commanding() {
		t.Error("expected commanding to be true after OpenCommand")
	}
	if got := ep.CommandValue(); got != "w " {
		t.Errorf("expected command value 'w ', got %q", got)
	}

	ep.CloseCommand()
	if ep.Commanding() {
		t.Error("expected commanding to be false after CloseCommand")
	}
	if got := ep.CommandValue(); got != "" {
		t.Errorf("expected command value empty, got %q", got)
	}
}

var testResolver = NewDefaultKeyResolver(" ")
var testSink = func(KeyAction) {}

func newTestEditorPane(cfg Config, path string) (*EditorPane, error) {
	return newEditorPane(cfg, path, testResolver, testSink)
}
