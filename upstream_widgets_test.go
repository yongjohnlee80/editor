package editor

// UPSTREAM WIDGETS, DRIVEN FROM THIS REPO.
//
// The editor still carries its own Button, Modal and TopMenu. Replacing them is
// the migration proper; this file is the step before it, and it answers a
// narrower question that has to be answered first: do golib's widgets actually
// work when a consumer wires them up, as opposed to when golib's own tests do?
//
// That distinction has teeth. golib's suite drives widgets through a harness it
// owns, with a settle() that knows the loop's internals. A consumer has an App,
// a backend and injected events, and nothing else. A widget that depends on its
// author's harness to behave passes upstream and fails here.
//
// Every test below therefore uses ONLY the public surface: tui.NewApp, a
// TestBackend, injected key events, and App.Update to read state on the loop.

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// runApp starts an App over the component and returns a stop function.
//
// The sleeps match the convention already in this package's tests: the App runs
// its own loop on another goroutine, and a consumer has no barrier into it.
func runApp(t *testing.T, root tui.Component, tb *tui.TestBackend) (*tui.App, func()) {
	t.Helper()
	app := tui.NewApp(root, tui.WithBackend(tb))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	time.Sleep(60 * time.Millisecond)
	return app, func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("App.Run did not return within 3s of cancel")
		}
	}
}

// onLoop runs fn on the App loop and waits for it, so a test reads widget state
// on the goroutine that owns it rather than racing the loop.
func onLoop(t *testing.T, app *tui.App, fn func()) {
	t.Helper()
	done := make(chan struct{})
	app.Update(func() { fn(); close(done) })
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("App.Update did not run within 3s")
	}
}

// TestUpstreamButtonActivatesUnderThisRepo.
//
// The floor: an upstream Button, focused by the framework's own focus ring
// rather than by a setter, activates on Enter and stops activating when
// disabled. The editor's local Button has SetFocused; the upstream one does not,
// because focus is the runtime's. This proves the runtime actually delivers it.
func TestUpstreamButtonActivatesUnderThisRepo(t *testing.T) {
	var fired atomic.Int64
	btn := widget.NewButton("Save", widget.WithOnActivate(func() { fired.Add(1) }))

	tb := tui.NewTestBackend(60, 12)
	app, stop := runApp(t, btn, tb)
	defer stop()

	tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyTab})
	time.Sleep(50 * time.Millisecond)
	tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	if got := fired.Load(); got != 1 {
		t.Fatalf("the button fired %d times on Enter, want 1 — the framework focus "+
			"ring did not deliver activation to an upstream Button", got)
	}

	// Disabled is not merely unstyled: it must refuse activation.
	onLoop(t, app, func() { btn.SetEnabled(false) })
	tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	if got := fired.Load(); got != 1 {
		t.Errorf("the button fired %d times total, want still 1 — a disabled button "+
			"activated", got)
	}
}

// TestUpstreamModalTrapsFocusAndDismissesWithItsButtons.
//
// The case the editor's own Modal exists to handle, done entirely upstream:
// a Modal opened into an OverlayHost traps focus, Tab cycles only its buttons,
// Enter on one activates it, and the dismissal reason reaches the callback.
//
// Focus confinement is the assertion that matters. A modal whose Tab escapes
// into the body behind it is not a modal, and it is exactly the property a
// widget cannot verify about itself without a real App underneath.
//
// NOTE FOR THE MIGRATION: activating a button does NOT close the dialog. Only
// Escape dismisses on its own; a button's role decides where Escape routes and
// which control takes initial focus, not whether the dialog closes. The caller
// dismisses from its own callback, which is what the buttons below do. Anything
// in this repo that relies on its local Modal closing itself when OK is pressed
// has to grow an explicit Dismiss during the migration.
func TestUpstreamModalTrapsFocusAndDismissesWithItsButtons(t *testing.T) {
	var okHits, cancelHits atomic.Int64
	var reason atomic.Int64
	reason.Store(-1)

	var modal *widget.Modal
	okBtn := widget.NewButton("OK",
		widget.WithRole(widget.ButtonRoleDefault),
		widget.WithOnActivate(func() {
			okHits.Add(1)
			modal.Dismiss(widget.DismissAccept)
		}))
	cancelBtn := widget.NewButton("Cancel",
		widget.WithRole(widget.ButtonRoleCancel),
		widget.WithOnActivate(func() {
			cancelHits.Add(1)
			modal.Dismiss(widget.DismissCancel)
		}))

	body := widget.NewText("Discard unsaved changes?")
	modal = widget.NewModal(body,
		widget.WithModalTitle("Confirm"),
		widget.WithButtons(okBtn, cancelBtn),
		widget.WithOnDismiss(func(r widget.DismissReason) {
			reason.Store(int64(r))
		}))

	// A focusable behind the modal, so "focus is trapped" has something to
	// escape TO. Without it the test would pass on an empty tab ring.
	behind := widget.NewButton("BEHIND", widget.WithOnActivate(func() {
		t.Error("a control BEHIND the modal activated; focus was not trapped")
	}))
	host := widget.NewOverlayHost(behind)

	tb := tui.NewTestBackend(60, 16)
	app, stop := runApp(t, host, tb)
	defer stop()

	var openErr error
	onLoop(t, app, func() { openErr = modal.Open(host) })
	if openErr != nil {
		t.Fatalf("Modal.Open: %v", openErr)
	}
	time.Sleep(60 * time.Millisecond)

	onLoop(t, app, func() {
		if !modal.IsOpen() {
			t.Error("IsOpen() is false after a successful Open")
		}
	})
	if got := tb.String(); !strings.Contains(got, "Confirm") {
		t.Errorf("the modal title is not on screen:\n%s", got)
	}

	// Tab several times — more than the modal has buttons. If the trap leaks,
	// focus reaches BEHIND and activating it fails the test from its callback.
	for range 6 {
		tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyTab})
		time.Sleep(15 * time.Millisecond)
	}
	tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	time.Sleep(60 * time.Millisecond)

	if okHits.Load()+cancelHits.Load() != 1 {
		t.Fatalf("after tabbing within the modal, OK fired %d and Cancel fired %d; "+
			"want exactly one activation inside the trap",
			okHits.Load(), cancelHits.Load())
	}
	if r := reason.Load(); r < 0 {
		t.Error("no dismissal reason was reported after a button activated and " +
			"called Dismiss")
	}
	onLoop(t, app, func() {
		if modal.IsOpen() {
			t.Error("the dialog is still open after its button dismissed it")
		}
	})
}

// TestUpstreamModalDismissesOnEscapeWithTheEscapeReason.
//
// Escape is the one dismissal path with no button behind it, and the reason
// carried matters: a caller that saves on any dismissal would save on a cancel.
func TestUpstreamModalDismissesOnEscapeWithTheEscapeReason(t *testing.T) {
	var reason atomic.Int64
	reason.Store(-1)

	modal := widget.NewModal(widget.NewText("body"),
		widget.WithModalTitle("Escapable"),
		widget.WithOnDismiss(func(r widget.DismissReason) { reason.Store(int64(r)) }))
	host := widget.NewOverlayHost(widget.NewText("base"))

	tb := tui.NewTestBackend(50, 12)
	app, stop := runApp(t, host, tb)
	defer stop()

	onLoop(t, app, func() {
		if err := modal.Open(host); err != nil {
			t.Fatalf("Modal.Open: %v", err)
		}
	})
	time.Sleep(60 * time.Millisecond)

	tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEscape})
	time.Sleep(60 * time.Millisecond)

	onLoop(t, app, func() {
		if modal.IsOpen() {
			t.Error("the modal is still open after Escape")
		}
	})
	if got := reason.Load(); got != int64(widget.DismissEscape) {
		t.Errorf("dismissal reason = %d, want DismissEscape (%d)",
			got, widget.DismissEscape)
	}
}

// saveActionID identifies the one command these menu tests dispatch.
const saveActionID tui.ActionID = "editor.test.save"

// saveAction is a menu command's intent. tui.Action is an interface carrying an
// identity, so a model can be built from configuration and still name actions
// the program resolves later.
type saveAction struct{}

func (saveAction) ActionID() tui.ActionID { return saveActionID }

// TestUpstreamMenuOpensASubmenuAndRunsACommand.
//
// The editor's TopMenu owns categories, submenu levels and execution. This is
// the same shape expressed in the upstream model: a submenu row with children,
// opened by ID, and a command inside it whose Action runs.
func TestUpstreamMenuOpensASubmenuAndRunsACommand(t *testing.T) {
	var ran atomic.Int64

	items := []widget.MenuItemModel{
		widget.NewSubmenu("file", "File", []widget.MenuItemModel{
			widget.NewCommand("file.save", "Save", saveAction{}),
		}),
		widget.NewCommand("quit", "Quit", nil),
	}

	// An item's Action is an IDENTITY, not a callback — the menu dispatches it
	// and an executor decides what it means. That split is why the editor can
	// route menu commands through its own command table instead of closing over
	// application state inside the model.
	menu := widget.NewMenu(widget.WithActionExecutor(func(inv tui.ActionInvocation) bool {
		if inv.Action == nil {
			return false
		}
		if inv.Action.ActionID() == saveActionID {
			ran.Add(1)
			return true
		}
		return false
	}))
	tb := tui.NewTestBackend(60, 14)
	host := widget.NewOverlayHost(menu)
	app, stop := runApp(t, host, tb)
	defer stop()

	var setErr, openErr error
	onLoop(t, app, func() { setErr = menu.SetModel(items) })
	if setErr != nil {
		t.Fatalf("SetModel: %v", setErr)
	}
	time.Sleep(40 * time.Millisecond)

	onLoop(t, app, func() { openErr = menu.Open("file") })
	if openErr != nil {
		t.Fatalf("Open(file): %v", openErr)
	}
	time.Sleep(60 * time.Millisecond)

	onLoop(t, app, func() {
		if got := menu.OpenLevels(); got == 0 {
			t.Error("OpenLevels() is 0 after opening a submenu")
		}
		if !menu.Select("file.save") {
			t.Error("Select(file.save) returned false; the child row is not selectable")
		}
	})
	time.Sleep(40 * time.Millisecond)

	// The menu is a TAB STOP, not an always-on key sink: Enter reaches a row
	// only once the menu holds focus. Skipping this sent Enter nowhere and made
	// a working dispatch look broken.
	tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyTab})
	time.Sleep(40 * time.Millisecond)
	tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	time.Sleep(60 * time.Millisecond)

	if got := ran.Load(); got != 1 {
		t.Errorf("the submenu command ran %d times, want 1 — activating a selected "+
			"child row did not reach its Action", got)
	}
}

// TestUpstreamMenuRefusesADuplicateID.
//
// SetModel returns an error rather than panicking, which is the contract a
// consumer building menus from configuration depends on: bad config should
// surface as a value, not take the process down.
func TestUpstreamMenuRefusesADuplicateID(t *testing.T) {
	menu := widget.NewMenu()
	err := menu.SetModel([]widget.MenuItemModel{
		widget.NewCommand("dup", "One", nil),
		widget.NewCommand("dup", "Two", nil),
	})
	if err == nil {
		t.Error("SetModel accepted two rows with the same ID; a duplicate ID makes " +
			"Select and SetEnabled ambiguous")
	}
	// The control: the same model with distinct IDs is accepted, so the error
	// above is about the duplication and not about the model shape.
	if err := menu.SetModel([]widget.MenuItemModel{
		widget.NewCommand("one", "One", nil),
		widget.NewCommand("two", "Two", nil),
	}); err != nil {
		t.Errorf("SetModel rejected a valid two-row model: %v", err)
	}
}
