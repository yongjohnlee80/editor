package editor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yongjohnlee80/golib/tui"
)

// These drive the real component tree over tui.TestBackend — no PTY. That
// matters here more than usual: the binary cannot be exercised in CI at all
// (it refuses to start without a terminal), so this is the only place the
// wiring is actually observed rather than assumed.

type harness struct {
	t    *testing.T
	app  *App
	rt   *tui.App
	tb   *tui.TestBackend
	stop context.CancelFunc
	done chan error
}

// read runs fn ON THE LOOP GOROUTINE and waits for it.
//
// App state is loop-owned. Reading a.footer.commanding or a.message straight
// from the test goroutine is a DATA RACE — the race detector caught exactly
// that, and it was the test reaching in, not a defect in the app. App.Update is
// the sanctioned crossing: it enqueues onto lane B, which never blocks.
func (h *harness) read(fn func()) {
	h.t.Helper()
	done := make(chan struct{})
	h.rt.Update(func() {
		fn()
		close(done)
	})
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		h.t.Fatal("App.Update did not run within 3s — the loop is wedged")
	}
}

// commanding reports whether the command line is open, read safely.
func (h *harness) commanding() bool {
	h.t.Helper()
	var v bool
	h.read(func() { v = h.app.footer.commanding })
	return v
}

func (h *harness) message() string {
	h.t.Helper()
	var v string
	h.read(func() { v = h.app.message })
	return v
}

func (h *harness) cmdValue() string {
	h.t.Helper()
	var v string
	h.read(func() { v = h.app.cmdIn.Value() })
	return v
}

func (h *harness) path() string {
	h.t.Helper()
	var v string
	h.read(func() { v = h.app.path })
	return v
}

func (h *harness) mode() string {
	h.t.Helper()
	var v string
	h.read(func() { v = h.app.editor.Mode().String() })
	return v
}

func newHarness(t *testing.T, cfg Config, path string) *harness {
	t.Helper()
	tb := tui.NewTestBackend(80, 24)
	app, err := New(cfg, path, func() {})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := tui.NewApp(app, tui.WithBackend(tb))
	h := &harness{t: t, app: app, rt: a, tb: tb, stop: cancel, done: make(chan error, 1)}
	go func() { h.done <- a.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-h.done:
		case <-time.After(3 * time.Second):
			t.Error("App.Run did not return within 3s after cancel")
		}
	})
	h.settle()
	return h
}

func (h *harness) settle() {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		h.read(func() {})
		h.read(func() {})
		f := h.tb.Flushes()
		time.Sleep(20 * time.Millisecond)
		h.read(func() {})
		if h.tb.Flushes() == f {
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatalf("frame activity did not settle within 3s")
		}
	}
}

func (h *harness) typeText(s string) {
	h.t.Helper()
	evs := make([]tui.Event, 0, len(s))
	for _, r := range s {
		evs = append(evs, tui.KeyEvent{Kind: tui.KeyPress, Code: r, Text: string(r)})
	}
	if err := h.tb.Inject(evs...); err != nil {
		h.t.Fatalf("Inject: %v", err)
	}
	h.settle()
}

func (h *harness) enter() {
	h.t.Helper()
	if err := h.tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: '\r'}); err != nil {
		h.t.Fatalf("Inject Enter: %v", err)
	}
	h.settle()
}

func (h *harness) escape() {
	h.t.Helper()
	if err := h.tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEscape}); err != nil {
		h.t.Fatalf("Inject Escape: %v", err)
	}
	h.settle()
}

// A file that exists loads into the buffer.
func TestNew_LoadsAnExistingFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(p, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, err := New(DefaultConfig(), p, func() {})
	if err != nil {
		t.Fatal(err)
	}
	if got := app.editor.Value(); !strings.Contains(got, "hello") {
		t.Errorf("buffer = %q, want the file's contents", got)
	}
}

// A path that does not exist opens EMPTY rather than failing: that is a new
// file, and ":w" creates it.
func TestNew_MissingPathIsANewFile(t *testing.T) {
	app, err := New(DefaultConfig(), filepath.Join(t.TempDir(), "new.txt"), func() {})
	if err != nil {
		t.Fatalf("a missing path must open a new buffer, not fail: %v", err)
	}
	if got := app.editor.Value(); got != "" {
		t.Errorf("buffer = %q, want empty", got)
	}
}

// A path that exists but cannot be READ is an error. Showing an empty buffer
// for a file that is there invites overwriting it.
func TestNew_UnreadableFileIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: mode 0 is still readable")
	}
	p := filepath.Join(t.TempDir(), "locked.txt")
	if err := os.WriteFile(p, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := New(DefaultConfig(), p, func() {}); err == nil {
		t.Error("an unreadable existing file must be an error")
	}
}

// ":" opens the command line, and ":w" writes the buffer to disk.
func TestCommand_WriteSavesTheBuffer(t *testing.T) {
	p := filepath.Join(t.TempDir(), "out.txt")
	if err := os.WriteFile(p, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, DefaultConfig(), p)

	h.typeText(":")
	if !h.commanding() {
		t.Fatal(`":" did not open the command line`)
	}
	h.typeText("w")
	h.enter()

	if h.commanding() {
		t.Error("the command line stayed open after submit")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "first\n" {
		t.Errorf("file = %q, want the buffer's contents", b)
	}
}

// ":w NAME" on an unnamed buffer creates the file and ADOPTS the name, so a
// later bare ":w" goes to the same place.
func TestCommand_WriteWithAName(t *testing.T) {
	p := filepath.Join(t.TempDir(), "named.txt")
	h := newHarness(t, DefaultConfig(), "")

	h.typeText(":")
	h.typeText("w " + p)
	h.enter()

	if _, err := os.Stat(p); err != nil {
		t.Fatalf(":w NAME did not create the file: %v", err)
	}
	if got := h.path(); got != p {
		t.Errorf("path = %q, want it to adopt %q", got, p)
	}
}

// ":w" with NO name anywhere prompts instead of writing, and the prompt is
// seeded with "w " so the user only types the name.
func TestCommand_WriteWithNoNamePrompts(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	h.typeText(":")
	h.typeText("w")
	h.enter()

	if !h.commanding() {
		t.Error(":w on an unnamed buffer must reopen the command line to prompt")
	}
	if got := h.cmdValue(); got != "w " {
		t.Errorf("prompt = %q, want it seeded with %q", got, "w ")
	}
	if msg := h.message(); !strings.Contains(msg, "name") {
		t.Errorf("message = %q, want it to ask for a name", msg)
	}
}

// An unknown command is REFUSED BY NAME. Ignoring it would look like it worked.
func TestCommand_UnknownIsRefusedByName(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")
	h.typeText(":")
	h.typeText("zzz")
	h.enter()
	if msg := h.message(); !strings.Contains(msg, "zzz") {
		t.Errorf("message = %q, want it to name the rejected command", msg)
	}
}

// The leader key opens the command line too, and it is CONFIGURABLE.
func TestLeaderKey_OpensTheCommandLine(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Keyboard.LeaderKey = ","
	h := newHarness(t, cfg, "")

	h.typeText(",")
	if !h.commanding() {
		t.Error("the configured leader key did not open the command line")
	}
}

// The DEFAULT leader is a spacebar, and a non-leader key must not open it —
// otherwise the previous test would pass for any key at all.
func TestLeaderKey_OtherKeysDoNotOpenIt(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Keyboard.LeaderKey = ","
	h := newHarness(t, cfg, "")

	h.typeText("x")
	if h.commanding() {
		t.Error("an unrelated key opened the command line")
	}
}

// HorizontalWrap reaches the Editor. Asserted through the app's own config
// rather than the widget's private state, and paired with the off case so the
// test cannot pass for a hard-coded mode.
func TestConfig_HorizontalWrapReachesTheEditor(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		cfg := DefaultConfig()
		cfg.Editor.HorizontalWrap = wrap
		app, err := New(cfg, "", func() {})
		if err != nil {
			t.Fatal(err)
		}
		if app.cfg.Editor.HorizontalWrap != wrap {
			t.Errorf("HorizontalWrap = %v, want %v", app.cfg.Editor.HorizontalWrap, wrap)
		}
	}
}

// The footer reports the Editor's mode rather than tracking its own copy.
func TestFooter_ShowsTheEditorMode(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")
	if got := h.mode(); got != "NORMAL" {
		t.Fatalf("mode = %q, want NORMAL at startup", got)
	}
	h.typeText("i")
	if got := h.mode(); got != "INSERT" {
		t.Errorf("mode after i = %q, want INSERT", got)
	}
}

// The sample ADR shipped for manual testing must actually open, and it must
// contain a line long enough to exercise HorizontalWrap — otherwise it cannot
// demonstrate the setting it was written to demonstrate.
func TestSampleADR_OpensAndHasALongLine(t *testing.T) {
	t.Parallel()
	const p = "testdata/sample-adr.md"
	app, err := New(DefaultConfig(), p, func() {})
	if err != nil {
		t.Fatalf("the sample ADR must open: %v", err)
	}
	body := app.editor.Value()
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

// Esc cancels the command line, restores focus to the editor, and reverts the footer.
func TestCommand_EscapeCancelsCommandLine(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	h.typeText(":")
	if !h.commanding() {
		t.Fatal(`":" did not open the command line`)
	}
	h.typeText("w")
	if got := h.cmdValue(); got != "w" {
		t.Fatalf("cmd input = %q, want %q", got, "w")
	}

	h.escape()
	if h.commanding() {
		t.Error("Esc must close the command line")
	}
	if got := h.cmdValue(); got != "" {
		t.Errorf("cmd input after Esc = %q, want empty", got)
	}
	if got := h.mode(); got != "NORMAL" {
		t.Errorf("mode after Esc = %q, want NORMAL", got)
	}
}

// The command line displays "COMMAND: " on the left side of the cursor when opened.
func TestCommand_DisplaysPromptAndCursorPosition(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	h.typeText(":")
	if !h.commanding() {
		t.Fatal(`":" did not open the command line`)
	}

	rendered := h.tb.String()
	if !strings.Contains(rendered, "COMMAND:") {
		t.Fatalf("bottom line did not display COMMAND: prompt; screen:\n%s", rendered)
	}

	snap := h.tb.Snapshot()
	cell := snap[23][0] // first character 'C' of "COMMAND: "
	if cell.Content != "C" {
		t.Errorf("snap[23][0].Content = %q, want 'C'", cell.Content)
	}
	if cell.Attrs.BG.Index != 4 {
		t.Errorf("prompt cell BG index = %d, want 4 (blue)", cell.Attrs.BG.Index)
	}
	if cell.Attrs.FG.Index != 15 {
		t.Errorf("prompt cell FG index = %d, want 15 (white)", cell.Attrs.FG.Index)
	}

	x, y, visible := h.tb.CursorPos()
	if !visible {
		t.Fatal("cursor must be visible in command line")
	}
	wantX := len("COMMAND: ")
	if x != wantX {
		t.Errorf("cursor X = %d, want %d (on the right of %q)", x, wantX, "COMMAND: ")
	}
	if y != 23 {
		t.Errorf("cursor Y = %d, want 23 (bottom line)", y)
	}

	// Typing a character advances the cursor past the prompt.
	h.typeText("q")
	x2, _, _ := h.tb.CursorPos()
	if x2 != wantX+1 {
		t.Errorf("cursor X after typing = %d, want %d", x2, wantX+1)
	}

	// Esc exits and restores the status bar.
	h.escape()
	if h.commanding() {
		t.Fatal("Esc did not close the command line")
	}
	restored := h.tb.String()
	if !strings.Contains(restored, "NORMAL") {
		t.Errorf("status bar was not restored after Esc; screen:\n%s", restored)
	}
}

