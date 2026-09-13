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
	"errors"
	"time"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// New builds the editor around an optional file. An empty path opens an unnamed
// buffer, exactly as "vim" with no argument does.
//
// A path that does not exist is NOT an error: it opens empty and ":w" creates
// it. A path that exists but cannot be read IS an error, because silently
// showing an empty buffer for a file that is there invites overwriting it.
func New(cfg Config, path string, quit func()) (*App, error) {
	a := &App{cfg: cfg, quit: quit}

	// Construct pure key resolver configured with the validated LeaderKey.
	resolver := NewDefaultKeyResolver(cfg.Keyboard.LeaderKey)
	sink := a.handleKeyAction

	var err error
	a.editorPane, err = newEditorPane(cfg, path, resolver, sink)
	if err != nil {
		return nil, err
	}

	// footer renders the three-segment status bar at the bottom.
	a.footer = newFooter()

	// dock is the top-level layout container. It describes the screen from
	// the outside in:
	//   ┌─ OverlayHost (modal layer) ──────────────────┐
	//   │  ┌─ Dock ──────────────────────────────────┐ │
	//   │  │  editorPane (editor + cmdline) ← fills  │ │
	//   │  │──────────────────────────────────────────│ │
	//   │  │  footer (status bar)     ← pinned bottom │ │
	//   │  └─────────────────────────────────────────-┘ │
	//   └──────────────────────────────────────────────-┘
	// The footer is pinned to the bottom edge; the editorPane expands to fill
	// the remaining space above it.
	dock := tui.NewDock()
	dock.Pin(tui.DockBottom, a.footer)
	dock.Add(a.editorPane)

	// host wraps the dock in an OverlayHost so that modal widgets (e.g. a
	// file-picker or confirmation dialog) have a surface to attach to on top
	// of the rest of the UI without disturbing the layout below.
	a.host = widget.NewOverlayHost(dock)

	// registry is the command registry mapping ex command verbs to Handlers.
	a.registry = NewRegistry()
	a.registerCommands()

	return a, nil
}

// App is the root component. Build it with [New] and hand it to tui.NewApp.
// It implements [tui.Component] (Init, Layout, Render, HandleEvent), which
// the golib/tui application loop mounts and drives as the root of the UI tree.
//
// App is a thin orchestrator: it owns the layout shell (dock + overlay host),
// the command-line lifecycle (open/close/run), and status-bar refresh. All
// document-editing concerns (buffer text, file path, dirty state, file I/O,
// floating command line) live in EditorPane and the command registry; footer
// presentation lives in Footer.
type App struct {
	// cfg holds user/distro configuration (e.g. soft wrapping, leader key).
	cfg Config

	// quit terminates the application run loop when an exit command (:q, :wq) runs.
	quit func()

	// ctx is the mounted framework context retained from Init. Valid for the
	// component's lifetime; used for layout, focus, and dirty notifications.
	ctx *tui.Context

	// editorPane owns the editor widget, box, floating command line, file path, and dirty state.
	// App interacts with it through a narrow surface: NodeID, Mode,
	// MarkDirty, title, OpenCommand, and CloseCommand. EditorPane also implements Document for OS commands.
	editorPane *EditorPane

	// footer manages status bar presentation (statistics, mode, clock) at the bottom
	// of the screen.
	footer *Footer

	// host is the root OverlayHost (embedding *tui.Stack) wrapping the main dock
	// layout (editorPane + footer) as its base layer. It acts as the z-stack anchor
	// for popups, floats, and modal dialogs:
	//   1. Z-ordering: upper layers paint on top of the base editor and receive input
	//      events first (reverse order hit-testing).
	//   2. Bus handshake: automatically listens for overlayOpenEvent and overlayCloseEvent
	//      so child widgets (like dropdowns) can mount popups without manual wiring.
	//   3. Floating modals: provides the surface where permanent or transient Float
	//      windows attach and unmount, restoring focus automatically upon close.
	host *widget.OverlayHost

	// registry is the command registry mapping ex verbs (":w", ":e", ":q") to Handlers.
	registry *Registry

	// message is the transient text the footer shows instead of the file path:
	// a write confirmation, or why a command was refused. Cleared on the next
	// command.
	message string
}

// registerCommands registers built-in ex commands in the command registry.
func (a *App) registerCommands() {
	// OS commands: file save (:w, :write) and file open (:e, :edit).
	InitOSCommands(a.registry, a.editorPane)

	// Quit command (:q).
	quitCmd := Command[struct{}](func(_ *tui.Context, _ string) CommandResponse[struct{}] {
		if a.editorPane.dirty {
			return Refuse[struct{}](errors.New("unsaved changes — :q! to discard, :wq to save"))
		}
		a.quit()
		return Ok(struct{}{})
	})
	a.registry.Add(Register(quitCmd, "quit editor"), "q", "quit")

	// Force-quit command (:q!).
	forceQuitCmd := Command[struct{}](func(_ *tui.Context, _ string) CommandResponse[struct{}] {
		a.quit()
		return Ok(struct{}{})
	})
	a.registry.Add(Register(forceQuitCmd, "quit without saving"), "q!")

	// Write and quit (:wq).
	writeCmd := NewWriteFileCmd(a.editorPane)
	wqCmd := Command[string](func(ctx *tui.Context, arg string) CommandResponse[string] {
		resp := writeCmd(ctx, arg)
		if resp.Status() == StatusOK {
			a.quit()
		}
		return resp
	})
	a.registry.Add(Register(wqCmd, "write and quit"), "wq")
}

// Init mounts the root component tree to the TUI context, subscribes to editor
// mode and change events, wires the command line submit handler, starts the
// 1-second status bar clock, and focuses the editor.
//
// # Lifecycle
//
// Init implements [tui.Component]. It is not called directly on *App; instead,
// it is called via interface dispatch by the TUI framework on startup when the
// root component is mounted (tui.NewApp(app).Run(ctx) → mount(nil, a.root) →
// comp.Init). It runs exactly once before any Layout, Render, or HandleEvent
// calls. The passed ctx is valid for the component's entire mounted lifetime
// and is retained in a.ctx.
//
//	app, err := editor.New(cfg, file, stop)
//	tui.NewApp(app, tui.WithBackend(backend)).Run(ctx)
func (a *App) Init(ctx *tui.Context) {
	a.ctx = ctx
	ctx.Mount(a.host)

	// MODE comes from the EditorPane rather than being tracked here: the widget
	// owns the state machine, so mirroring it would be a second source of
	// truth that can disagree.
	tui.SubscribeScoped(ctx, func(ev widget.ModeChangedEvent) {
		if ev.Owner == a.editorPane.NodeID() {
			a.refresh()
		}
	})
	// CHANGE marks the buffer dirty and refreshes the status line after the
	// editor accepts an edit.
	tui.SubscribeScoped(ctx, func(ev widget.ChangeEvent) {
		if ev.Owner == a.editorPane.NodeID() {
			a.editorPane.MarkDirty()
			a.refresh()
		}
	})
	// SUBMIT belongs to the command input; its value is dispatched as an editor
	// command rather than inserted into the active buffer.
	tui.SubscribeScoped(ctx, func(ev widget.SubmitEvent) {
		if ev.Owner == a.editorPane.CmdInputNodeID() {
			a.runCommand(ev.Value)
		}
	})
	// The clock is a repeating TickEvent addressed to this node, so it costs
	// nothing while idle and needs no goroutine of its own.
	ctx.Every(time.Second)
	ctx.FocusComponent(a.editorPane)
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

// HandleEvent handles periodic clock ticks to update the status line time.
// Keyboard events are handled locally within EditorPane.
func (a *App) HandleEvent(ev tui.Event) bool {
	switch ev.(type) {
	case tui.TickEvent:
		a.refresh()
		return true
	}
	return false
}

// handleKeyAction acts as the single application-level action coordinator,
// executing cross-component focus and visibility transitions in response to
// semantic KeyActions dispatched by focused components.
func (a *App) handleKeyAction(action KeyAction) {
	switch action {
	case ActionOpenCommandLine:
		a.openCommand("")
	case ActionCancelCommandLine:
		a.closeCommand()
	}
}

// openCommand shows the command line, seeded with prefill.
func (a *App) openCommand(prefill string) {
	a.message = ""
	a.editorPane.OpenCommand(prefill)
	a.refresh()
}

// closeCommand hides the command input, clears its buffer, restores focus to the
// editor, and requests layout and footer refreshes.
func (a *App) closeCommand() {
	a.editorPane.CloseCommand()
	a.refresh()
}

// runCommand interprets one ex command using the command registry.
func (a *App) runCommand(line string) {
	a.closeCommand()
	resp := a.registry.Dispatch(a.ctx, line)

	switch resp.Status() {
	case StatusOK:
		if s, ok := resp.Result().(string); ok && s != "" {
			a.setMessage(s)
		}
	case StatusPromptNeeded:
		prefill := ""
		if s, ok := resp.Result().(string); ok {
			prefill = s
		}
		a.openCommand(prefill)
		a.setMessage("new file: type a name after :w")
	case StatusRefused, StatusUnknown:
		if resp.Err() != nil {
			a.setMessage(resp.Err().Error())
		}
	}
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
	modeStr := " " + a.editorPane.Mode().String() + " "
	centre := a.editorPane.title()
	if a.message != "" {
		centre = a.message
	}
	timeStr := time.Now().Format("15:04:05") + " "
	a.footer.SetStatus(modeStr, centre, timeStr)
	a.ctx.MarkDirty()
}
