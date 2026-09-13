package editor

import (
	"os"

	"github.com/yongjohnlee80/golib/errs"
	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// commandPromptStyle renders the command title / prompt in reverse-video blue/white
// (ANSI colour 6 background, colour 0 foreground) so it stands out visually.
var commandPromptStyle = style.New().
	Background(style.ANSI(6)).
	Foreground(style.ANSI(0))

// EditorPane is the self-contained editor surface: a modal text buffer (the
// widget.Editor) wrapped in a titled border (widget.Box), along with a floating
// command line (widget.TextInput wrapped in a bordered widget.Box) positioned in
// the middle of the pane.
//
// EditorPane processes all keyboard-related events including buffer text edits
// and ex commands:
//
//	┌ [title] ──────────────────────────────────────────┐
//	│ buffer line 1                                     │
//	│ buffer line 2                                     │
//	│              ╭── COMMAND: ──────────────────╮     │
//	│              │ :w                           │     │
//	│              ╰──────────────────────────────╯     │
//	│ buffer line 3                                     │
//	└───────────────────────────────────────────────────┘
//
// # Always-mounted design
//
// Both the main editor box and the floating cmdBox are mounted once in Init and
// remain mounted for the component's lifetime. When commanding is false, cmdBox
// receives zero constraints and a zero rect so it does not paint and consumes no
// space, preserving stable NodeIDs for event subscriptions.
type EditorPane struct {
	// ctx is the mounted framework context retained from Init. Valid for the
	// component's lifetime; used to lay out and place child widgets.
	ctx *tui.Context

	// editor is the core text-buffer widget. It owns the vi modal editing state
	// machine (Normal vs. Insert), cursor navigation, and buffer text mutations.
	editor *widget.Editor

	// box wraps editor in a bordered frame whose title shows the buffer name
	// and a [+] dirty indicator when the buffer has unsaved changes.
	box *widget.Box

	// cmdInput is the text input field that receives the command string.
	cmdInput *widget.TextInput

	// cmdBox wraps cmdInput in a bordered box centered in the middle of the editor.
	cmdBox *widget.Box

	// commanding is true while the command line popup is active.
	commanding bool

	// path is the on-disk path for this buffer. Empty means "unnamed" —
	// the state that causes ":w" to prompt for a name rather than write.
	path string

	// dirty is true if the buffer has been modified since the last save. It is
	// flagged via MarkDirty and used to update the box title.
	dirty bool

	// resolver maps physical key events to semantic actions under ScopeEditorNormal and ScopeCommandLine.
	resolver KeyResolver

	// sink dispatches resolved semantic actions synchronously to the parent coordinator.
	sink KeyActionSink
}

// newEditorPane constructs the EditorPane for the given path and config,
// loading the file contents if the path exists.
func newEditorPane(cfg Config, path string, resolver KeyResolver, sink KeyActionSink) (*EditorPane, error) {
	if resolver == nil {
		return nil, errs.Wrap(errs.ErrInvalidArgument, "editorPane: resolver must not be nil")
	}
	if sink == nil {
		return nil, errs.Wrap(errs.ErrInvalidArgument, "editorPane: sink must not be nil")
	}
	ep := &EditorPane{path: path, resolver: resolver, sink: sink}

	wrap := widget.WrapNone
	if cfg.Editor.HorizontalWrap {
		wrap = widget.WrapSoft
	}
	ep.editor = widget.NewEditor(widget.WithEditorWrap(wrap))

	if path != "" {
		b, err := os.ReadFile(path)
		switch {
		case err == nil:
			ep.editor.SetValue(string(b))
		case os.IsNotExist(err):
			// A new file. Nothing to load, and ":w" will create it.
		default:
			return nil, errs.WrapCause(errs.ErrInvalidArgument, err,
				"editor: opening %s", path)
		}
	}

	ep.box = widget.NewBox(ep.editor, widget.WithTitle(ep.title()))

	ep.cmdInput = widget.NewTextInput()
	ep.cmdBox = widget.NewBox(
		ep.cmdInput,
		widget.WithTitle("COMMAND:"),
		widget.WithStyle(style.New().Border(style.BorderRounded)),
	)

	return ep, nil
}

// Init mounts the editor box and the floating command box into the TUI context.
func (ep *EditorPane) Init(ctx *tui.Context) {
	ep.ctx = ctx
	ctx.Mount(ep.box)
	ctx.Mount(ep.cmdBox)
}

// Layout sizes and positions the editor box, and when commanding is true,
// centers the floating command box in the middle of the pane.
func (ep *EditorPane) Layout(c tui.Constraints) tui.Size {
	sz := ep.ctx.LayoutChild(ep.box, c)
	ep.ctx.PlaceChild(ep.box, tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H})

	if !ep.commanding {
		ep.ctx.LayoutChild(ep.cmdBox, tui.Constraints{})
		ep.ctx.PlaceChild(ep.cmdBox, tui.Rect{})
	} else {
		// Float in the middle: width up to 60 columns (bounded by available width), height 3 rows.
		boxW := min(60, sz.W)
		if boxW < 10 && sz.W > 0 {
			boxW = sz.W
		}
		boxH := min(3, sz.H)
		boxSz := ep.ctx.LayoutChild(ep.cmdBox, tui.Constraints{
			MinW: boxW, MaxW: boxW,
			MinH: boxH, MaxH: boxH,
		})
		x := (sz.W - boxSz.W) / 2
		if x < 0 {
			x = 0
		}
		y := (sz.H - boxSz.H) / 2
		if y < 0 {
			y = 0
		}
		ep.ctx.PlaceChild(ep.cmdBox, tui.Rect{X: x, Y: y, W: boxSz.W, H: boxSz.H})
	}

	return c.Constrain(sz)
}

// Render is a no-op because EditorPane's children produce the visual output.
func (ep *EditorPane) Render(tui.Surface) {}

// HandleEvent intercepts unconsumed keyboard events. When commanding is active,
// it handles command cancellation (<Escape>). In Normal mode, it handles ":" or
// the LeaderKey to activate the floating command line directly.
func (ep *EditorPane) HandleEvent(ev tui.Event) bool {
	ke, ok := ev.(tui.KeyEvent)
	if !ok || ke.Kind == tui.KeyRelease {
		return false
	}

	if ep.commanding {
		if action, ok := ep.resolver.Resolve(ScopeCommandLine, ke); ok {
			switch action {
			case ActionCancelCommandLine:
				ep.CloseCommand()
				if ep.sink != nil {
					ep.sink(action)
				}
				return true
			case ActionToggleMenuBar:
				if ep.sink != nil {
					ep.sink(action)
				}
				return true
			}
		}
		return false
	}

	// In non-Normal modes (Insert mode or Nano modeless editing), F10 toggles the menu bar.
	if ep.Mode() != widget.ModeNormal {
		if ke.Code == tui.KeyF10 {
			if ep.sink != nil {
				ep.sink(ActionToggleMenuBar)
			}
			return true
		}
		return false
	}
	if action, ok := ep.resolver.Resolve(ScopeEditorNormal, ke); ok {
		switch action {
		case ActionOpenCommandLine:
			ep.OpenCommand("")
			if ep.sink != nil {
				ep.sink(action)
			}
			return true
		case ActionToggleMenuBar:
			if ep.sink != nil {
				ep.sink(action)
			}
			return true
		}
	}
	return false
}

// SetKeyset configures the editor buffer's editing profile (e.g. KeysetVim, KeysetNano).
// Switching to KeysetNano disables modal editing and enters Insert mode; switching to
// KeysetVim restores modal editing in Normal mode.
func (ep *EditorPane) SetKeyset(ks widget.Keyset) {
	opt := widget.WithKeyset(ks)
	opt(ep.editor)
	if ep.ctx != nil {
		ep.ctx.MarkDirty()
	}
}

// Keyset returns the editor buffer's active editing profile.
func (ep *EditorPane) Keyset() widget.Keyset {
	return ep.editor.Keyset()
}

// NodeID returns the editor widget's stable NodeID.
func (ep *EditorPane) NodeID() tui.NodeID {
	return ep.editor.NodeID()
}

// CmdInputNodeID returns the command input widget's stable NodeID.
func (ep *EditorPane) CmdInputNodeID() tui.NodeID {
	return ep.cmdInput.NodeID()
}

// Commanding reports whether the floating command line is currently active.
func (ep *EditorPane) Commanding() bool {
	return ep.commanding
}

// CommandValue returns the current text in the command line input.
func (ep *EditorPane) CommandValue() string {
	return ep.cmdInput.Value()
}

// OpenCommand activates the floating command line with the given prefill text,
// focuses the command input, and requests a layout pass.
func (ep *EditorPane) OpenCommand(prefill string) {
	ep.cmdInput.SetValue(prefill)
	ep.commanding = true
	if ep.ctx != nil {
		ep.ctx.FocusComponent(ep.cmdInput)
		ep.ctx.RequestLayout()
	}
}

// CloseCommand deactivates the floating command line, clears its text,
// restores focus to the editor, and requests a layout pass.
func (ep *EditorPane) CloseCommand() {
	ep.commanding = false
	ep.cmdInput.SetValue("")
	if ep.ctx != nil {
		ep.ctx.FocusComponent(ep.editor)
		ep.ctx.RequestLayout()
	}
}

// Mode returns the editor's current modal editing mode (Normal / Insert).
func (ep *EditorPane) Mode() widget.EditorMode {
	return ep.editor.Mode()
}

// Value returns the current buffer text verbatim.
func (ep *EditorPane) Value() string {
	return ep.editor.Value()
}

// MarkDirty records that the buffer has unsaved changes and refreshes the box title.
func (ep *EditorPane) MarkDirty() {
	ep.dirty = true
	ep.box.SetTitle(ep.title())
}

// MarkClean clears the dirty flag after a successful write and refreshes the box title.
func (ep *EditorPane) MarkClean() {
	ep.dirty = false
	ep.box.SetTitle(ep.title())
}

// Path returns the current on-disk path for this buffer.
func (ep *EditorPane) Path() string {
	return ep.path
}

// SetPath adopts a new on-disk path and refreshes the box title.
func (ep *EditorPane) SetPath(path string) {
	ep.path = path
	ep.box.SetTitle(ep.title())
}

// SetValue replaces the entire buffer text.
func (ep *EditorPane) SetValue(v string) {
	ep.editor.SetValue(v)
}

// WriteFile saves the buffer to disk using the standard NewWriteFileCmd command.
func (ep *EditorPane) WriteFile(arg string) CommandResponse[string] {
	cmd := NewWriteFileCmd(ep)
	return cmd(ep.ctx, arg)
}

// title returns the display title of the buffer for the enclosing box header.
func (ep *EditorPane) title() string {
	name := ep.path
	if name == "" {
		name = "[No Name]"
	}
	if ep.dirty {
		name += " [+]"
	}
	return name
}
