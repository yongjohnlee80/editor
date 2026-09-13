package editor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// These tests drive the real component tree over tui.TestBackend — no PTY.
// That matters here more than usual: the binary cannot be exercised in CI at
// all (it refuses to start without a terminal), so this is the only place the
// wiring is actually observed rather than assumed.
//
// Tests that belong to the document layer (file loading, write, dirty flag)
// live in com-editor_test.go and call newEditorPane directly, keeping this
// file focused on App orchestration: the command lifecycle, key routing,
// footer behaviour, and status bar.

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
	h.read(func() { v = h.app.editorPane.commanding })
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
	h.read(func() { v = h.app.editorPane.CommandValue() })
	return v
}

func (h *harness) path() string {
	h.t.Helper()
	var v string
	h.read(func() { v = h.app.editorPane.path })
	return v
}

func (h *harness) mode() string {
	h.t.Helper()
	var v string
	h.read(func() { v = h.app.editorPane.editor.Mode().String() })
	return v
}

func (h *harness) keyset() widget.Keyset {
	h.t.Helper()
	var ks widget.Keyset
	h.read(func() { ks = h.app.editorPane.Keyset() })
	return ks
}

func (h *harness) menuActive() bool {
	h.t.Helper()
	var act bool
	h.read(func() { act = h.app.menu.Active() })
	return act
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

func (h *harness) pressKey(code rune) {
	h.t.Helper()
	if err := h.tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: code}); err != nil {
		h.t.Fatalf("Inject Key %v: %v", code, err)
	}
	h.settle()
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

// The floating command line displays "COMMAND:" prompt centered in the editor when opened.
func TestCommand_DisplaysPromptAndCursorPosition(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	h.typeText(":")
	if !h.commanding() {
		t.Fatal(`":" did not open the command line`)
	}

	rendered := h.tb.String()
	if !strings.Contains(rendered, "COMMAND:") {
		t.Fatalf("editor did not display floating COMMAND: prompt; screen:\n%s", rendered)
	}

	x, y, visible := h.tb.CursorPos()
	if !visible {
		t.Fatal("cursor must be visible in command line")
	}
	// Floating command box is vertically centered in the 23-row editor pane (y = (23-3)/2 = 10; inner row 11)
	if y != 11 {
		t.Errorf("cursor Y = %d, want 11 (vertically centered floating box inner row)", y)
	}

	// Typing a character advances the cursor.
	h.typeText("q")
	x2, y2, _ := h.tb.CursorPos()
	if y2 != y {
		t.Errorf("cursor Y changed after typing: got %d, want %d", y2, y)
	}
	if x2 != x+1 {
		t.Errorf("cursor X after typing = %d, want %d", x2, x+1)
	}

	// Esc exits and restores focus to editor.
	h.escape()
	if h.commanding() {
		t.Fatal("Esc did not close the command line")
	}
	restored := h.tb.String()
	if !strings.Contains(restored, "NORMAL") {
		t.Errorf("status bar was not present after Esc; screen:\n%s", restored)
	}
}

// ":e" opens an existing file into the active buffer and updates the title.
func TestCommand_EditOpensFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "external.txt")
	if err := os.WriteFile(p, []byte("loaded externally\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newHarness(t, DefaultConfig(), "")

	h.typeText(":")
	h.typeText("e " + p)
	h.enter()

	if h.commanding() {
		t.Error("the command line stayed open after submit")
	}
	if got := h.path(); got != p {
		t.Errorf("path = %q, want %q after :e", got, p)
	}
	var val string
	h.read(func() { val = h.app.editorPane.Value() })
	if !strings.Contains(val, "loaded externally") {
		t.Errorf("buffer value = %q, want 'loaded externally'", val)
	}
}

// F10 activates and deactivates the top menu bar.
func TestTopMenu_F10TogglesMenuBar(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	if h.menuActive() {
		t.Fatal("menu bar should be inactive initially")
	}

	// Pressing F10 activates the menu bar.
	h.pressKey(tui.KeyF10)
	if !h.menuActive() {
		t.Fatal("F10 did not activate top menu bar")
	}

	screen := h.tb.String()
	if !strings.Contains(screen, "File") || !strings.Contains(screen, "Option") || !strings.Contains(screen, "Help") {
		t.Fatalf("menu bar did not render expected categories; screen:\n%s", screen)
	}

	// Pressing F10 again toggles it off.
	h.pressKey(tui.KeyF10)
	if h.menuActive() {
		t.Fatal("second F10 did not deactivate top menu bar")
	}

	// Re-activate and test Escape to dismiss.
	h.pressKey(tui.KeyF10)
	if !h.menuActive() {
		t.Fatal("F10 failed to re-activate top menu bar")
	}
	h.escape()
	if h.menuActive() {
		t.Fatal("Esc did not deactivate top menu bar")
	}
}

// Navigating to Option -> Keymaps allows switching between Vim and Nano keymaps.
func TestTopMenu_KeymapSwitchModal(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	if ks := h.keyset(); ks != widget.KeysetVim {
		t.Fatalf("initial keyset = %v, want KeysetVim", ks)
	}

	// F10 to activate menu bar
	h.pressKey(tui.KeyF10)

	// Move right to Option
	h.pressKey(tui.KeyRight)

	// Open dropdown
	h.pressKey(tui.KeyEnter)

	// Open Keymaps modal (first item in Option)
	h.pressKey(tui.KeyEnter)

	modalScreen := h.tb.String()
	if !strings.Contains(modalScreen, "Select Editor Keymap") {
		t.Fatalf("Keymaps modal was not rendered; screen:\n%s", modalScreen)
	}

	// Press '2' to switch to Nano
	h.pressKey('2')

	if ks := h.keyset(); ks != widget.KeysetNano {
		t.Fatalf("keyset after selecting Nano = %v, want KeysetNano", ks)
	}
	if msg := h.message(); !strings.Contains(msg, "Nano") {
		t.Errorf("expected status message mentioning Nano, got %q", msg)
	}

	// Switch back to Vim via menu
	h.pressKey(tui.KeyF10)
	h.pressKey(tui.KeyRight)
	h.pressKey(tui.KeyEnter)
	h.pressKey(tui.KeyEnter)

	// Press '1' to switch to Vim
	h.pressKey('1')

	if ks := h.keyset(); ks != widget.KeysetVim {
		t.Fatalf("keyset after selecting Vim = %v, want KeysetVim", ks)
	}
	if msg := h.message(); !strings.Contains(msg, "Vim") {
		t.Errorf("expected status message mentioning Vim, got %q", msg)
	}
}

// File -> Exit opens the exit modal and can be dismissed without quitting.
func TestTopMenu_ExitModalCancel(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	// F10 -> Enter (open File dropdown) -> navigate to Exit
	h.pressKey(tui.KeyF10)
	h.pressKey(tui.KeyEnter)

	// Move down to Exit (New -> Open -> Save -> Exit = 3 downs)
	h.pressKey(tui.KeyDown)
	h.pressKey(tui.KeyDown)
	h.pressKey(tui.KeyDown)
	h.pressKey(tui.KeyEnter)

	screen := h.tb.String()
	if !strings.Contains(screen, "Are you sure to quit?") {
		t.Fatalf("Exit confirmation modal did not appear; screen:\n%s", screen)
	}

	// Press 'n' to dismiss
	h.pressKey('n')
	if h.menuActive() {
		t.Fatal("modal and menu should be deactivated after 'n'")
	}
}

// Unimplemented menu items show the "Not Implemented" modal.
func TestTopMenu_NotImplementedModal(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	// F10 -> Enter (File) -> Enter (New)
	h.pressKey(tui.KeyF10)
	h.pressKey(tui.KeyEnter)
	h.pressKey(tui.KeyEnter)

	screen := h.tb.String()
	if !strings.Contains(screen, "Not Implemented") {
		t.Fatalf("Not Implemented modal did not appear; screen:\n%s", screen)
	}
	if !strings.Contains(screen, "File -> New") {
		t.Fatalf("modal message should mention File -> New; screen:\n%s", screen)
	}

	// Esc dismisses the modal
	h.escape()
	if h.menuActive() {
		t.Fatal("modal and menu should be deactivated after Esc")
	}
}
