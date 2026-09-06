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
type App struct {
	cfg  Config
	quit func()
	ctx  *tui.Context

	editor *widget.Editor
	box    *widget.Box
	status *widget.StatusBar
	cmdIn  *widget.TextInput
	footer *footer
	host   *widget.OverlayHost

	// path is empty for a buffer that has never been written. That is the
	// state ":w" has to prompt about, so it is tracked rather than inferred
	// from an empty filename at save time.
	path  string
	dirty bool

	// message is the transient text the footer shows instead of the file path:
	// a write confirmation, or why a command was refused. Cleared on the next
	// command.
	message string
}

// New builds the editor around an optional file. An empty path opens an unnamed
// buffer, exactly as "vim" with no argument does.
//
// A path that does not exist is NOT an error: it opens empty and ":w" creates
// it. A path that exists but cannot be read IS an error, because silently
// showing an empty buffer for a file that is there invites overwriting it.
func New(cfg Config, path string, quit func()) (*App, error) {
	a := &App{cfg: cfg, quit: quit, path: path}

	wrap := widget.WrapNone
	if cfg.Editor.HorizontalWrap {
		// Soft wrap has no horizontal extent, so the Editor also stops
		// drawing a horizontal scroll indicator — the hiding is a
		// consequence of the wrap, not a second setting.
		wrap = widget.WrapSoft
	}
	a.editor = widget.NewEditor(widget.WithEditorWrap(wrap))

	if path != "" {
		b, err := os.ReadFile(path)
		switch {
		case err == nil:
			a.editor.SetValue(string(b))
		case os.IsNotExist(err):
			// A new file. Nothing to load, and ":w" will create it.
		default:
			return nil, errs.WrapCause(errs.ErrInvalidArgument, err,
				"editor: opening %s", path)
		}
	}

	a.box = widget.NewBox(a.editor, widget.WithTitle(a.title()))
	a.status = widget.NewStatusBar()
	a.cmdIn = widget.NewTextInput()
	a.footer = &footer{status: a.status, input: a.cmdIn}

	dock := tui.NewDock()
	dock.Pin(tui.DockBottom, a.footer)
	dock.Add(a.box)
	a.host = widget.NewOverlayHost(dock)
	return a, nil
}

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
	tui.SubscribeScoped(ctx, func(ev widget.ChangeEvent) {
		if ev.Owner == a.editor.NodeID() {
			a.dirty = true
			a.refresh()
		}
	})
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

func (a *App) Layout(c tui.Constraints) tui.Size {
	sz := a.ctx.LayoutChild(a.host, c)
	a.ctx.PlaceChild(a.host, tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H})
	return c.Constrain(sz)
}

func (a *App) Render(tui.Surface) {}

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
	if k.Kind == tui.KeyRelease || a.footer.commanding {
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
// the TextInput reports as a DismissEvent the footer handles.
func (a *App) openCommand(prefill string) {
	a.message = ""
	a.cmdIn.SetValue(prefill)
	a.footer.commanding = true
	a.ctx.FocusComponent(a.cmdIn)
	a.ctx.RequestLayout()
	a.refresh()
}

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

// footer shows the status bar, or the command line while one is being typed.
//
// Both children stay MOUNTED and only one is laid out, rather than mounting and
// unmounting on every ":". Remounting would give the input a new NodeID each
// time, which invalidates the SubmitEvent subscription that Init set up once.
type footer struct {
	ctx        *tui.Context
	status     *widget.StatusBar
	input      *widget.TextInput
	commanding bool
}

func (f *footer) Init(ctx *tui.Context) {
	f.ctx = ctx
	ctx.Mount(f.status)
	ctx.Mount(f.input)
}

func (f *footer) Layout(c tui.Constraints) tui.Size {
	active, idle := tui.Component(f.status), tui.Component(f.input)
	if f.commanding {
		active, idle = f.input, f.status
	}
	sz := f.ctx.LayoutChild(active, c)
	f.ctx.PlaceChild(active, tui.Rect{X: 0, Y: 0, W: c.MaxW, H: sz.H})
	// The idle child is given zero height rather than left unplaced: an
	// unplaced child keeps its previous rect and would paint over the active
	// one.
	f.ctx.LayoutChild(idle, tui.Constraints{})
	f.ctx.PlaceChild(idle, tui.Rect{})
	return c.Constrain(tui.Size{W: c.MaxW, H: sz.H})
}

func (f *footer) Render(tui.Surface) {}

// HandleEvent lets everything through. The footer is a layout shell: its
// children handle their own keys, and the App owns the ":" that opens the
// command line.
func (f *footer) HandleEvent(tui.Event) bool { return false }
