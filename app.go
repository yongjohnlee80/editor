// Package editor is a small modal text editor built on golib/tui.
//
// The composition follows autodb's query editor: a widget.Editor in a Box, a
// StatusBar pinned to the bottom of a Dock, and the whole thing inside an
// OverlayHost so modals have somewhere to attach.
//
//	┌ path/to/file ──────────────┐
//	│ 1 the vim editor panel     │
//	│ 2                          │
//	└────────────────────────────┘
//	 NORMAL   path/to/file   14:22
//
// The footer doubles as the command line: typing ":" replaces the status
// segments with an input, which is what vim does and what makes ":q" and ":w"
// land where a vim user looks for them.
package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yongjohnlee80/golib/errs"
	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// App is the root component. Build it with [New] and hand it to tui.NewApp.
// It implements [tui.Component] (Init, Layout, Render, HandleEvent), which
// the golib/tui application loop mounts and drives as the root of the UI tree.
type App struct {
	// cfg holds user/distro configuration (e.g. soft wrapping, leader key).
	cfg Config

	// quit terminates the application run loop when an exit command (:q, :wq) runs.
	quit func()

	// ctx is the mounted framework context retained from Init. Valid for the
	// component's lifetime; used for layout, focus, and dirty notifications.
	ctx *tui.Context

	// editor is the core text buffer widget. It owns the vi modal editing state
	// machine (Normal vs. Insert), cursor navigation, and buffer text mutations.
	editor *widget.Editor

	// box wraps editor in a bordered frame with a title bar displaying the buffer
	// filename and [+] dirty indicator.
	box *widget.Box

	// status renders the three-part status line at the bottom: mode on the left,
	// file path or transient message in the center, and wall clock on the right.
	status *widget.StatusBar

	// cmdPrompt is the static label displayed on the left side of the cursor in commanding mode.
	cmdPrompt *widget.Text

	// cmdIn is the single-line text input for ex commands, shown when ":" is pressed.
	cmdIn *widget.TextInput

	// footer manages swapping between status and cmdIn at the bottom of the screen
	// while keeping both mounted so NodeIDs and event subscriptions remain stable.
	footer *footer

	// host is the root OverlayHost (embedding *tui.Stack) wrapping the main dock
	// layout (box + footer) as its base layer. It acts as the z-stack anchor for
	// popups, floats, and modal dialogs:
	//   1. Z-ordering: upper layers paint on top of the base editor and receive input
	//      events first (reverse order hit-testing).
	//   2. Bus handshake: automatically listens for overlayOpenEvent and overlayCloseEvent
	//      so child widgets (like dropdowns) can mount popups without manual wiring.
	//   3. Floating modals: provides the surface where permanent or transient Float
	//      windows attach and unmount, restoring focus automatically upon close.
	host *widget.OverlayHost

	// path is empty for a buffer that has never been written. That is the
	// state ":w" has to prompt about, so it is tracked rather than inferred
	// from an empty filename at save time.
	path string

	// dirty is true if the buffer has been modified since the last save. It is
	// flagged by the editor and used to update the box title and prompt for confirmation
	dirty bool

	// message is the transient text the footer shows instead of the file path:
	// a write confirmation, or why a command was refused. Cleared on the next
	// command.
	message string
}

// Init mounts the root component tree to the TUI context, subscribes to editor
// mode and change events, wires the command line submit handler, starts the
// 1-second status bar clock, and focuses the editor.
//
// Lifecycle:
// Init implements [tui.Component]. It is not called directly on *App; instead,
// it is called via interface dispatch by the TUI framework on startup when the
// root component is mounted (tui.NewApp(app).Run(ctx) -> mount(nil, a.root) -> comp.Init).
// It runs exactly once before any Layout, Render, or HandleEvent calls. The passed
// ctx is valid for the component's entire mounted lifetime and is retained in a.ctx.
//
// app, err := editor.New(cfg, file, stop)
// tui.NewApp(app, tui.WithBackend(backend)).Run(ctx)
func (a *App) Init(ctx *tui.Context) {
	a.ctx = ctx
	ctx.Mount(a.host)

	// MODE comes from the Editor rather than being tracked here: the widget
	// owns the state machine, so mirroring it would be a second source of
	// truth that can disagree.
	tui.SubscribeScoped(ctx, func(ev widget.ModeChangedEvent) {
		if ev.Owner == a.editor.NodeID() {
			a.refresh()
		}
	})
	// CHANGE marks the buffer dirty and refreshes the status line after the
	// editor accepts an edit.
	tui.SubscribeScoped(ctx, func(ev widget.ChangeEvent) {
		if ev.Owner == a.editor.NodeID() {
			a.dirty = true
			a.refresh()
		}
	})
	// SUBMIT belongs to the command input; its value is dispatched as an editor
	// command rather than inserted into the active buffer.
	tui.SubscribeScoped(ctx, func(ev widget.SubmitEvent) {
		if ev.Owner == a.cmdIn.NodeID() {
			a.runCommand(ev.Value)
		}
	})
	// The clock is a repeating TickEvent addressed to this node, so it costs
	// nothing while idle and needs no goroutine of its own.
	ctx.Every(time.Second)
	ctx.FocusComponent(a.editor)
	a.refresh()
}

// Layout sizes and positions the root OverlayHost within the provided
// constraints, returning the constrained child size.
func (a *App) Layout(c tui.Constraints) tui.Size {
	sz := a.ctx.LayoutChild(a.host, c)
	a.ctx.PlaceChild(a.host, tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H})
	return c.Constrain(sz)
}

// Render is a no-op because App is a container component whose visual output is
// entirely produced by its mounted child hierarchy (a.host).
func (a *App) Render(tui.Surface) {}

// HandleEvent handles periodic clock ticks to update the status line time and
// intercepts normal-mode key events to trigger the command line.
func (a *App) HandleEvent(ev tui.Event) bool {
	switch t := ev.(type) {
	case tui.TickEvent:
		a.refresh()
		return true
	case tui.KeyEvent:
		return a.handleKey(t)
	}
	return false
}

// handleKey opens the command line. Everything else is left to bubble, so the
// Editor keeps every binding it publishes — this deliberately intercepts as
// little as possible.
func (a *App) handleKey(k tui.KeyEvent) bool {
	if k.Kind == tui.KeyRelease {
		return false
	}
	if a.footer.commanding {
		if k.Code == tui.KeyEscape {
			a.closeCommand()
			return true
		}
		return false
	}
	// Only from Normal mode: in Insert, ":" and the leader are text.
	if a.editor.Mode() != widget.ModeNormal {
		return false
	}
	if k.Text == ":" || (a.cfg.Keyboard.LeaderKey != "" && k.Text == a.cfg.Keyboard.LeaderKey) {
		a.openCommand("")
		return true
	}
	return false
}

// openCommand shows the command line, seeded with prefill. Esc cancels, which
// bubbles from the focused TextInput to App.handleKey.
func (a *App) openCommand(prefill string) {
	a.message = ""
	a.cmdIn.SetValue(prefill)
	a.footer.commanding = true
	a.ctx.FocusComponent(a.cmdIn)
	a.ctx.RequestLayout()
	a.refresh()
}

// closeCommand hides the command input, clears its buffer, restores focus to the
// editor, and requests layout and footer refreshes.
func (a *App) closeCommand() {
	a.footer.commanding = false
	a.cmdIn.SetValue("")
	a.ctx.FocusComponent(a.editor)
	a.ctx.RequestLayout()
	a.refresh()
}

// runCommand interprets one ex command. The vocabulary is deliberately tiny —
// q, q!, w, wq, w <path> — and anything else is REFUSED by name rather than
// ignored, so a typo says so instead of appearing to work.
func (a *App) runCommand(line string) {
	a.closeCommand()
	cmd := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ":"))
	if cmd == "" {
		return
	}
	verb, arg, _ := strings.Cut(cmd, " ")
	arg = strings.TrimSpace(arg)

	switch verb {
	case "q":
		if a.dirty {
			// vim's E37. Refusing beats discarding: the user can still
			// force it, and nothing is lost by asking.
			a.setMessage("unsaved changes — :q! to discard, :wq to save")
			return
		}
		a.quit()
	case "q!":
		a.quit()
	case "w", "wq":
		if err := a.write(arg); err != nil {
			a.setMessage(err.Error())
			return
		}
		if verb == "wq" {
			a.quit()
		}
	default:
		a.setMessage(fmt.Sprintf("not an editor command: %s", verb))
	}
}

// write saves the buffer. With no path anywhere — neither an argument nor a
// previous name — it PROMPTS by reopening the command line seeded with "w ",
// which is where a vim user would type the name anyway.
func (a *App) write(arg string) error {
	path := arg
	if path == "" {
		path = a.path
	}
	if path == "" {
		// openCommand CLEARS the message — correct for a fresh ":", wrong
		// here — so the prompt is opened first and the reason set after it.
		// The other order left the input seeded with "w " and no
		// explanation of why.
		a.openCommand("w ")
		a.setMessage("new file: type a name after :w")
		return nil
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return errs.WrapCause(errs.ErrInvalidArgument, err, "creating %s", dir)
		}
	}
	body := a.editor.Value()
	// A trailing newline, because a POSIX text file ends with one and every
	// other tool that reads this file expects it.
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return errs.WrapCause(errs.ErrInvalidArgument, err, "writing %s", path)
	}
	a.path = path
	a.dirty = false
	a.box.SetTitle(a.title())
	lines := 0
	if body != "" {
		lines = strings.Count(body, "\n")
	}
	a.setMessage(fmt.Sprintf("%q %dL written", path, lines))
	return nil
}

// setMessage updates the transient status line message and triggers a footer
// refresh.
func (a *App) setMessage(s string) {
	a.message = s
	a.refresh()
}

// refresh repaints the footer: MODE left, path (or the last message) centre,
// clock right.
func (a *App) refresh() {
	if a.ctx == nil {
		return
	}
	a.status.SetLeft(" " + a.editor.Mode().String() + " ")
	centre := a.title()
	if a.message != "" {
		centre = a.message
	}
	a.status.SetCenter(centre)
	a.status.SetRight(time.Now().Format("15:04:05") + " ")
	a.ctx.MarkDirty()
}

// title returns the display title of the buffer for the enclosing box header,
// showing "[No Name]" for an unnamed buffer and appending "[+]" when modified.
func (a *App) title() string {
	name := a.path
	if name == "" {
		name = "[No Name]"
	}
	if a.dirty {
		name += " [+]"
	}
	return name
}
