package editor

import (
	"os"

	"github.com/yongjohnlee80/golib/errs"
	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// EditorPane is the self-contained editor surface: a modal text buffer (the
// widget.Editor) wrapped in a titled border (widget.Box). It owns everything
// that belongs to "the document being edited":
//
//   - the widget.Editor and its enclosing widget.Box
//   - the file path and dirty flag
//   - loading the file from disk on construction
//   - writing the file back to disk via WriteFile (delegated to NewWriteFileCmd)
//   - producing the window title that reflects both path and dirty state
//
// EditorPane deliberately knows nothing about the command line, status bar,
// or the application quit callback. Those concerns belong to App.
//
// EditorPane implements the [Document] interface defined in document.go,
// so OS-level commands (NewReadFileCmd, NewWriteFileCmd) can operate on it
// without knowing anything about TUI widgets.
//
// # Visual structure
//
//	┌ path/to/file ──────────────┐
//	│ 1 the vim editor panel     │
//	│ 2                          │
//	└────────────────────────────┘
//
// # Separation of concerns
//
// App is responsible for orchestrating the full layout (dock, overlay host,
// footer), subscribing to events, and running ex commands. EditorPane only
// knows about the text it holds and the file it maps to on disk. App calls
// into EditorPane via the narrow surface below (NodeID, Mode, Value,
// MarkDirty, WriteFile) — EditorPane never calls back into App.
type EditorPane struct {
	// ctx is the mounted framework context retained from Init. Valid for the
	// component's lifetime; used to lay out and place the box child.
	ctx *tui.Context

	// editor is the core text-buffer widget. It owns the vi modal editing state
	// machine (Normal vs. Insert), cursor navigation, and buffer text mutations.
	editor *widget.Editor

	// box wraps editor in a bordered frame whose title shows the buffer name
	// and a [+] dirty indicator when the buffer has unsaved changes.
	box *widget.Box

	// path is the on-disk path for this buffer. Empty means "unnamed" —
	// the state that causes ":w" to prompt for a name rather than write.
	path string

	// dirty is true if the buffer has been modified since the last save. It is
	// flagged via MarkDirty and used to update the box title.
	dirty bool

	// resolver maps physical key events to semantic actions under ScopeEditorNormal.
	resolver KeyResolver

	// sink dispatches resolved semantic actions synchronously to the parent coordinator.
	sink KeyActionSink
}

// newEditorPane constructs the EditorPane for the given path and config,
// loading the file contents if the path exists.
//
// An empty path opens an unnamed buffer (same as "vim" with no argument).
// A path that does not exist is NOT an error: it opens empty and ":w" will
// create it. A path that exists but cannot be read IS an error, because
// silently showing an empty buffer for a file that is there invites overwriting it.
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
		// Soft wrap has no horizontal extent, so the Editor also stops
		// drawing a horizontal scroll indicator — the hiding is a
		// consequence of the wrap, not a second setting.
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

	// box is the main content area: the editor wrapped in a titled border.
	// It occupies the bulk of the screen and shows the buffer filename (and a
	// [+] dirty indicator) as its title.
	ep.box = widget.NewBox(ep.editor, widget.WithTitle(ep.title()))

	return ep, nil
}

// Init mounts the box (which transitively mounts the editor) into the TUI
// context and retains ctx for use during Layout.
//
// # Lifecycle
//
// Init is called exactly once by the TUI framework when EditorPane is first
// mounted as a child of the Dock (via ctx.Mount in App.Init). It runs before
// any Layout, Render, or HandleEvent call. The received ctx is the EditorPane's
// own component context — distinct from App's ctx — and is valid for the
// EditorPane's entire mounted lifetime.
//
// Call chain: tui.NewApp.Run → mount(nil, root) → App.Init → ctx.Mount(host)
// → … → ctx.Mount(editorPane) → EditorPane.Init.
func (ep *EditorPane) Init(ctx *tui.Context) {
	ep.ctx = ctx
	ctx.Mount(ep.box)
}

// Layout sizes and positions the box within the provided constraints and
// returns the EditorPane's own size to the parent (Dock).
//
// # Lifecycle
//
// Layout is called by the TUI framework on every layout pass — at startup,
// on ctx.RequestLayout, and after any terminal resize event. The framework
// calls it top-down: Dock.Layout → EditorPane.Layout. After Layout returns,
// the framework calls Render bottom-up (leaves first), so the sizes computed
// here are already committed when Render runs.
//
// c (tui.Constraints) carries the maximum width and height the Dock is
// offering after the pinned footer has been reserved at the bottom. The box
// expands to fill whatever space remains.
func (ep *EditorPane) Layout(c tui.Constraints) tui.Size {
	sz := ep.ctx.LayoutChild(ep.box, c)
	ep.ctx.PlaceChild(ep.box, tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H})
	return c.Constrain(sz)
}

// Render is a no-op because EditorPane is a container component whose visual
// output is entirely produced by its mounted child hierarchy (box → editor).
func (ep *EditorPane) Render(tui.Surface) {}

// HandleEvent intercepts unconsumed keyboard events that bubble up from the
// focused child widget (widget.Editor). In Normal mode, it resolves unhandled
// keys (such as ":" or LeaderKey) through the KeyResolver and forwards any
// resulting KeyAction synchronously to the KeyActionSink.
//
// In Insert mode, keys are consumed directly by widget.Editor and never reach
// this ancestor component.
func (ep *EditorPane) HandleEvent(ev tui.Event) bool {
	ke, ok := ev.(tui.KeyEvent)
	if !ok || ke.Kind == tui.KeyRelease {
		return false
	}
	// Only resolve actions in Normal mode.
	if ep.Mode() != widget.ModeNormal {
		return false
	}
	if action, ok := ep.resolver.Resolve(ScopeEditorNormal, ke); ok {
		ep.sink(action)
		return true
	}
	return false
}

// NodeID returns the editor widget's stable NodeID. App uses this to
// discriminate ModeChangedEvent and ChangeEvent by owner — only events whose
// Owner matches this ID are routed to EditorPane state updates.
func (ep *EditorPane) NodeID() tui.NodeID {
	return ep.editor.NodeID()
}

// Mode returns the editor's current modal editing mode (Normal / Insert).
// App forwards this to the status bar so the footer always reflects the live
// state without maintaining a redundant copy.
func (ep *EditorPane) Mode() widget.EditorMode {
	return ep.editor.Mode()
}

// Value returns the current buffer text verbatim, including any trailing
// newline that ":w" appended on the previous save.
func (ep *EditorPane) Value() string {
	return ep.editor.Value()
}

// MarkDirty records that the buffer has unsaved changes and refreshes the box
// title to show the [+] indicator. App calls this when a ChangeEvent whose
// Owner matches EditorPane.NodeID arrives in Init's subscription.
func (ep *EditorPane) MarkDirty() {
	ep.dirty = true
	ep.box.SetTitle(ep.title())
}

// MarkClean clears the dirty flag after a successful write and refreshes the
// box title to drop the [+] indicator. Implements [Document].
func (ep *EditorPane) MarkClean() {
	ep.dirty = false
	ep.box.SetTitle(ep.title())
}

// Path returns the current on-disk path for this buffer, or "" if the buffer
// has never been written (unnamed buffer). Implements [Document].
func (ep *EditorPane) Path() string {
	return ep.path
}

// SetPath adopts a new on-disk path and refreshes the box title. Called by
// WriteFileCmd after a successful write so that a subsequent bare ":w" goes
// to the same file. Implements [Document].
func (ep *EditorPane) SetPath(path string) {
	ep.path = path
	ep.box.SetTitle(ep.title())
}

// SetValue replaces the entire buffer text. Called by ReadFileCmd after
// successfully reading a file from disk. Implements [Document].
func (ep *EditorPane) SetValue(v string) {
	ep.editor.SetValue(v)
}

// WriteFile saves the buffer to disk using the standard NewWriteFileCmd command.
// arg is the optional path argument from a ":w <path>" command; when absent
// the buffer's existing ep.path is used.
//
// It executes NewWriteFileCmd(ep) directly, ensuring single source of truth
// for file write logic, directory creation, trailing newline formatting, and
// status response reporting.
func (ep *EditorPane) WriteFile(arg string) CommandResponse[string] {
	cmd := NewWriteFileCmd(ep)
	return cmd(ep.ctx, arg)
}

// title returns the display title of the buffer for the enclosing box header,
// showing "[No Name]" for an unnamed buffer and appending "[+]" when modified.
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
