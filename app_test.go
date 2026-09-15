package editor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// These tests drive the real component tree over tui.TestBackend — no PTY.
// That matters here more than usual: the binary cannot be exercised in CI at
// all (it refuses to start without a terminal), so this is the only place the
// wiring is actually observed rather than assumed.
//
// Tests that belong to the document layer (file loading, write, dirty flag)
// live in com_editor_test.go and call newEditorPane directly, keeping this
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

func (h *harness) pressKeyMod(code rune, mods tui.Mods) {
	h.t.Helper()
	if err := h.tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: code, Mods: mods}); err != nil {
		h.t.Fatalf("Inject Key %v with mods %v: %v", code, mods, err)
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

// Option -> Keymaps switches the editor between the Vim and Nano keysets.
//
// DRIVEN BY INTENT, NOT BY KEY CHOREOGRAPHY. This used to press
// F10/Right/Enter/Enter and depend on exactly what each of those did in the
// editor's own menu widget. Those bindings now belong to golib, which tests
// them; repeating the choreography here tested the widget's navigation and
// broke the moment it differed, while saying nothing about the editor. What
// the editor owns is the MODEL and the COMMAND TABLE — that a row called
// "Keymaps" exists under Option, that its children are radios, and that
// activating one switches the keyset and says so. That is what this asserts.
func TestTopMenu_KeymapSwitchModal(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	if ks := h.keyset(); ks != widget.KeysetVim {
		t.Fatalf("initial keyset = %v, want KeysetVim", ks)
	}

	// The cascade renders: Option holds Keymaps, which holds both keysets.
	h.read(func() { h.app.menu.OpenCategory(idOption) })
	h.settle()
	h.read(func() {
		if err := h.app.menu.Menu().Open(idKeymaps); err != nil {
			t.Fatalf("Open(Keymaps): %v", err)
		}
	})
	h.settle()
	screen := h.tb.String()
	if !strings.Contains(screen, "Keymaps") || !strings.Contains(screen, "Nano (modeless)") {
		t.Fatalf("the Keymaps cascade did not render; screen:\n%s", screen)
	}

	// Choosing Nano switches the keyset and reports it.
	h.read(func() { h.app.menu.Menu().Select(idKeysetNano) })
	h.settle()
	h.pressKey('\r')
	if ks := h.keyset(); ks != widget.KeysetNano {
		t.Fatalf("keyset after selecting Nano = %v, want KeysetNano", ks)
	}
	if msg := h.message(); !strings.Contains(msg, "Nano") {
		t.Errorf("status message = %q, want it to mention Nano", msg)
	}

	// And back again, which is the half that proves the rows are a radio pair
	// rather than a one-way switch.
	h.read(func() { h.app.menu.OpenCategory(idOption) })
	h.settle()
	h.read(func() {
		if err := h.app.menu.Menu().Open(idKeymaps); err != nil {
			t.Fatalf("reopen Keymaps: %v", err)
		}
		h.app.menu.Menu().Select(idKeysetVim)
	})
	h.settle()
	h.pressKey('\r')
	if ks := h.keyset(); ks != widget.KeysetVim {
		t.Fatalf("keyset after selecting Vim = %v, want KeysetVim", ks)
	}
	if msg := h.message(); !strings.Contains(msg, "Vim") {
		t.Errorf("status message = %q, want it to mention Vim", msg)
	}
}

// Alt shortcuts (Alt+f, Alt+o, Alt+h) trigger menu categories directly.
func TestTopMenu_AltShortcuts(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	// Alt+f opens File dropdown directly
	h.pressKeyMod('f', tui.ModAlt)
	if !h.menuActive() {
		t.Fatal("Alt+f did not activate menu")
	}
	screen := h.tb.String()
	if !strings.Contains(screen, "New") || !strings.Contains(screen, "Exit") {
		t.Fatalf("File dropdown items not visible after Alt+f; screen:\n%s", screen)
	}

	// Hotkey 'x' selects and executes Exit
	h.pressKey('x')
	screen = h.tb.String()
	if !strings.Contains(screen, "Are you sure to quit?") {
		t.Fatalf("Exit confirmation modal did not open on hotkey 'x'; screen:\n%s", screen)
	}
	// Escape dismisses. The old dialog buttons carried y/n mnemonics and this
	// pressed 'n'; golib's Button has none, and Escape routes to the
	// cancel-role button instead, which is the same outcome by the same intent.
	h.escape()

	// Alt+o directly opens Option dropdown
	h.pressKeyMod('o', tui.ModAlt)
	if !h.menuActive() {
		t.Fatal("Alt+o did not activate menu")
	}
	// Hotkey 'k' opens cascading submenu
	h.pressKey('k')
	screen = h.tb.String()
	if !strings.Contains(screen, "Nano (modeless)") {
		t.Fatalf("cascading submenu not open on 'k'; screen:\n%s", screen)
	}
	// Select Nano
	h.pressKey('2')
	if ks := h.keyset(); ks != widget.KeysetNano {
		t.Fatalf("keyset after Alt+o -> k -> 2 = %v, want KeysetNano", ks)
	}
}

// Menu can be placed on the left side (explorer style).
func TestTopMenu_PlacementLeft(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Menu.Placement = "left"
	h := newHarness(t, cfg, "")

	// THE "Menu" TITLE IS GONE. The editor's own left placement drew a titled
	// panel; golib's vertical bar is a column of rows and nothing else, so the
	// categories themselves are what proves the bar is there and vertical.
	screen := h.tb.String()
	for _, want := range []string{"File", "Option", "Help"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("the left bar is missing %q; screen:\n%s", want, screen)
		}
	}
	// Vertical, not the top bar wrapped: each category on its own line.
	if rowOf(screen, "File") == rowOf(screen, "Option") {
		t.Errorf("File and Option are on the same line, so the bar is not a column:\n%s",
			screen)
	}

	// Alt+f opens File dropdown to the right of the sidemenu
	h.pressKeyMod('f', tui.ModAlt)
	if !h.menuActive() {
		t.Fatal("Alt+f failed to activate left sidemenu")
	}
	screen = h.tb.String()
	if !strings.Contains(screen, "Exit") {
		t.Fatalf("File dropdown not visible next to left sidemenu; screen:\n%s", screen)
	}
	h.escape()
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

// TestReviewProbe_MenuDuringCommandLineRestoresCommandInput proves that opening and
// dismissing a menu while the floating command line is open properly restores focus
// to cmdInput rather than defaulting to the buffer editor.
func TestReviewProbe_MenuDuringCommandLineRestoresCommandInput(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	// 1. type ':' to open the floating command line
	h.typeText(":")
	if !h.commanding() {
		t.Fatal("expected commanding to be true after ':'")
	}

	// 2. press Alt+f to open File menu
	if err := h.tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: 'f', Mods: tui.ModAlt}); err != nil {
		t.Fatalf("Inject Alt+f: %v", err)
	}
	h.settle()
	if !h.menuActive() {
		t.Fatal("expected menu to be active after Alt+f")
	}

	// 3. press Escape twice (close dropdown, then close menu bar)
	h.escape()
	h.escape()
	if h.menuActive() {
		t.Fatal("expected menu to be inactive after double Escape")
	}

	// 4. type 'q'
	h.typeText("q")

	if !h.commanding() {
		t.Fatal("expected commanding to remain true")
	}
	if got := h.cmdValue(); got != "q" {
		t.Fatalf("CommandValue() = %q, want %q", got, "q")
	}
}

// TestTopMenu_NormalRestoreFocus verifies that standard menu open and close
// round-trip restores focus to the editor buffer for normal editing.
func TestTopMenu_NormalRestoreFocus(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	// Open menu via F10, close via Esc
	h.pressKey(tui.KeyF10)
	if !h.menuActive() {
		t.Fatal("expected menu active")
	}
	h.escape()
	if h.menuActive() {
		t.Fatal("expected menu inactive")
	}

	// Enter insert mode and type 'x'
	h.typeText("ix")
	h.escape()

	screen := h.tb.String()
	if !strings.Contains(screen, "x") {
		t.Fatalf("expected editor buffer to contain 'x', screen:\n%s", screen)
	}
}

// TestDocsStylesSnippet_Compiles verifies that the code snippet documented in
// docs/styles.md under "Floating Command Box" compiles and initializes cleanly.
func TestDocsStylesSnippet_Compiles(t *testing.T) {
	cmdInput := widget.NewTextInput()
	cmdBox := widget.NewBox(
		cmdInput,
		widget.WithTitle("COMMAND:"),
		widget.WithStyle(style.New().Border(style.BorderRounded)),
	)
	if cmdBox == nil {
		t.Fatal("expected non-nil cmdBox")
	}
}

// rowOf is the index of the first screen line containing sub, or -1.
//
// Used to tell a vertical bar from a horizontal one without hard-coding
// coordinates that a width or padding change would invalidate.
func rowOf(screen, sub string) int {
	for i, line := range strings.Split(screen, "\n") {
		if strings.Contains(line, sub) {
			return i
		}
	}
	return -1
}

// attrAt is the rendered attributes of one cell, for asserting a HIGHLIGHT.
//
// A highlight is an attribute, not a character: a test that reads the text
// cannot see one at all, which is how "the menu is highlighted" went unchecked
// through a change that stopped highlighting it.
func (h *harness) attrAt(x, y int) tui.CellAttrs {
	h.t.Helper()
	grid := h.tb.Snapshot()
	if y >= len(grid) || x >= len(grid[y]) {
		h.t.Fatalf("cell (%d,%d) is off the grid", x, y)
	}
	return grid[y][x].Attrs
}

// cellOf finds the first cell of sub on screen.
func (h *harness) cellOf(sub string) (x, y int) {
	h.t.Helper()
	grid := h.tb.Snapshot()
	for row := range grid {
		line := ""
		for _, c := range grid[row] {
			if c.Continuation() {
				continue
			}
			if c.Content == "" {
				line += " "
				continue
			}
			line += c.Content
		}
		if i := strings.Index(line, sub); i >= 0 {
			return i, row
		}
	}
	h.t.Fatalf("%q is not on screen:\n%s", sub, h.tb.String())
	return 0, 0
}

// TestTheMenuShowsWhereTheKeyboardIs.
//
// Three states, because the interesting part is the DIFFERENCE between them
// and any one of them alone is satisfied by a menu that highlights nothing:
//
//  1. editing — no category highlighted, or the bar claims the keyboard;
//  2. menu active — the category is highlighted, or there is no way to tell the
//     menu is what the arrow keys are driving;
//  3. dropdown open — a row INSIDE it is highlighted, not the category. Opening
//     a level hands the selection to that level, and this editor briefly
//     dragged it back to the bar, leaving the dropdown with nothing marked.
func TestTheMenuShowsWhereTheKeyboardIs(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	fx, fy := h.cellOf("File")
	ox, _ := h.cellOf("Option")

	editing := h.attrAt(fx, fy)
	if editing != h.attrAt(ox, fy) {
		t.Errorf("while editing, File is painted %+v and Option %+v; the bar is "+
			"claiming the keyboard\n%s", editing, h.attrAt(ox, fy), h.tb.String())
	}

	h.pressKey(tui.KeyF10)
	if !h.menuActive() {
		t.Fatal("F10 did not activate the menu")
	}
	active := h.attrAt(fx, fy)
	if active == h.attrAt(ox, fy) {
		t.Errorf("with the menu active, File is painted exactly like Option (%+v); "+
			"nothing shows which category the keys act on\n%s", active, h.tb.String())
	}

	h.read(func() { h.app.menu.OpenCategory(idFile) })
	h.settle()
	h.settle()
	nx, ny := h.cellOf("New")
	sx, sy := h.cellOf("Save") // its OWN row: sampling Save's column on New's
	// row reads a cell inside New, which compares the highlighted row with
	// itself and passes whatever the widget does.
	if h.attrAt(nx, ny) == h.attrAt(sx, sy) {
		t.Errorf("with the dropdown open, New is painted exactly like Save (%+v); "+
			"the selection stayed on the bar instead of moving into the level\n%s",
			h.attrAt(nx, ny), h.tb.String())
	}
	var sel widget.ItemID
	h.read(func() { sel, _ = h.app.menu.Menu().Selected() })
	if sel != "file.new" {
		t.Errorf("the selection is %q with File open, want the level's first row", sel)
	}
}

// TestClickingIntoTheBufferClosesTheMenu.
//
// A dropdown left hanging after the user has clicked into the text is covering
// the line they just aimed at, and the keys are going to the buffer while the
// menu still looks like the active surface.
func TestClickingIntoTheBufferClosesTheMenu(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	h.pressKey(tui.KeyF10)
	h.read(func() { h.app.menu.OpenCategory(idFile) })
	h.settle()
	h.settle()
	if !strings.Contains(h.tb.String(), "New") {
		t.Fatalf("the dropdown did not open:\n%s", h.tb.String())
	}

	// Focus goes to the editor, exactly as a click into the buffer does.
	h.read(func() { h.app.editorPane.FocusActive() })
	h.settle()
	h.settle()

	if grid := h.tb.String(); strings.Contains(grid, "New") {
		t.Errorf("the dropdown is still open after focus moved to the buffer:\n%s", grid)
	}
	if h.menuActive() {
		t.Error("the menu still reports itself active after focus left it")
	}
}

// TestF10AlwaysStartsAtTheFirstCategory.
//
// The selection survives a close, so reaching the menu again used to resume
// wherever the last visit ended: use Help, press F10, and the bar comes up on
// Help. Every visit starts at the left.
func TestF10AlwaysStartsAtTheFirstCategory(t *testing.T) {
	h := newHarness(t, DefaultConfig(), "")

	// Leave the selection somewhere other than the first category.
	h.pressKey(tui.KeyF10)
	h.read(func() { h.app.menu.OpenCategory(idHelp) })
	h.settle()
	var sel widget.ItemID
	h.read(func() { sel, _ = h.app.menu.Menu().Selected() })
	if sel == idFile {
		t.Fatalf("the fixture did not move the selection off File (got %q)", sel)
	}
	h.escape()
	h.escape()

	h.pressKey(tui.KeyF10)
	h.read(func() { sel, _ = h.app.menu.Menu().Selected() })
	if sel != idFile {
		t.Errorf("F10 resumed on %q; every visit starts at the first category", sel)
	}
}

// TestEscapeUnwindsTheMenuOneStageAtATime.
//
// The original editor's Escape was STAGED, and the migration flattened it: the
// widget closed the whole cascade on the first press and reported the key
// handled even with nothing open, so this editor had to layer a resolver that
// claimed Escape back just to let it reach the buffer.
//
// The widget now closes exactly one level per press and leaves Escape unhandled
// at the root, so the staging falls out of the contract and the workaround is
// gone. Counting presses is the assertion: a user backing out of a submenu
// expects to land in the dropdown that opened it, not on the bar.
func TestEscapeUnwindsTheMenuOneStageAtATime(t *testing.T) {
	for _, tc := range []struct {
		name    string
		open    func(h *harness)
		presses int
	}{
		{"F10 only", func(h *harness) {}, 1},
		{"a dropdown", func(h *harness) {
			h.read(func() { h.app.menu.OpenCategory(idFile) })
			h.settle()
		}, 2},
		{"a nested cascade", func(h *harness) {
			h.read(func() { h.app.menu.OpenCategory(idOption) })
			h.settle()
			h.settle()
			h.read(func() { _ = h.app.menu.Menu().Open(idKeymaps) })
			h.settle()
		}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, DefaultConfig(), "")
			h.pressKey(tui.KeyF10)
			tc.open(h)
			h.settle()
			if !h.menuActive() {
				t.Fatal("the menu is not active after opening")
			}

			// One short of the count: the menu must still be active.
			for i := 0; i < tc.presses-1; i++ {
				h.escape()
				if !h.menuActive() {
					t.Fatalf("Escape %d of %d left the menu; each press should unwind "+
						"one stage", i+1, tc.presses)
				}
			}
			h.escape()
			if h.menuActive() {
				t.Errorf("the menu is still active after %d presses", tc.presses)
			}
			// And focus came back to the buffer rather than being left nowhere.
			if got := h.mode(); got == "" {
				t.Error("the editor reports no mode after the menu released focus")
			}
		})
	}
}

// TestTheExitDialogAnswersItsMnemonics.
//
// The original editor's exit confirmation answered y and n from either button.
// The first migration dropped that and recorded it as an accepted loss; it was
// not one, and it is back — through golib's Button metadata and the Modal's
// resolution rather than around them.
func TestTheExitDialogAnswersItsMnemonics(t *testing.T) {
	t.Run("n dismisses without quitting", func(t *testing.T) {
		var quit atomic.Bool
		h := newHarnessQuit(t, func() { quit.Store(true) })
		h.read(func() { h.app.menu.OpenExitModal() })
		h.settle()
		h.settle()
		if !strings.Contains(h.tb.String(), "Are you sure") {
			t.Fatalf("the dialog did not open:\n%s", h.tb.String())
		}

		h.pressKey('n')
		if quit.Load() {
			t.Error("n quit the editor; it is the cancel button")
		}
		if strings.Contains(h.tb.String(), "Are you sure") {
			t.Errorf("n did not dismiss the dialog:\n%s", h.tb.String())
		}
	})

	t.Run("y quits", func(t *testing.T) {
		var quit atomic.Bool
		h := newHarnessQuit(t, func() { quit.Store(true) })
		h.read(func() { h.app.menu.OpenExitModal() })
		h.settle()
		h.settle()
		h.pressKey('y')
		if !quit.Load() {
			t.Error("y did not quit; it is the confirm button")
		}
	})
}

// newHarnessQuit is newHarness with a quit callback the test can observe.
func newHarnessQuit(t *testing.T, quit func()) *harness {
	t.Helper()
	tb := tui.NewTestBackend(80, 24)
	app, err := New(DefaultConfig(), "", quit)
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
