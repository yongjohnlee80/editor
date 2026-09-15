// Package editor is a small modal text editor built on golib/tui.
//
// The composition follows autodb's query editor: a widget.Editor in a Box, a
// StatusBar pinned to the bottom of a Dock, and the whole thing inside an
// OverlayHost so modals have somewhere to attach.
//
//	┌─ File ── Option ────────────────────────── Help ─┐
//	│ 1 the vim editor panel                           │
//	│   ╭── COMMAND: ───────╮                          │
//	│   │ :w                │                          │
//	│   ╰───────────────────╯                          │
//	└──────────────────────────────────────────────────┘
//	 NORMAL   path/to/file                         14:22
//
// The command line is rendered as a floating TextInput box centered in the
// middle of EditorPane when activated (e.g. typing ":" in Normal mode). The
// footer's sole responsibility is statistics and status reporting (mode, path
// or transient message, time).
package editor

import (
	"errors"
	"strings"
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
	a.resolver = resolver
	sink := a.handleKeyAction

	var err error
	a.editorPane, err = newEditorPane(cfg, path, resolver, sink)
	if err != nil {
		return nil, err
	}

	// footer renders the three-segment status bar at the bottom.
	a.footer = newFooter()

	// menu manages the top-level Borland-style menu bar (File, Option, Help),
	// its dropdowns, and the dialogs they open. The rows, the selection and the
	// popups are golib/tui's; what lives here is which commands this editor has
	// and what they do.
	placement := MenuPlacement(strings.ToLower(cfg.Menu.Placement))
	if placement == "" {
		placement = PlacementTop
	}
	a.menu = NewTopMenu(placement, TopMenuCallbacks{
		OnQuit: a.quit,
		OnStatusMessage: func(msg string) {
			a.setMessage(msg)
		},
		OnRestoreFocus: func() {
			a.editorPane.FocusActive()
		},
	})
	a.menu.SetHandlers(a.menuHandlers())
	if err := a.menu.SetModel(a.buildMenuModel()); err != nil {
		return nil, err
	}

	// dock is the top-level layout container. It describes the screen from
	// the outside in:
	//   ┌─ OverlayHost (modal layer) ──────────────────┐
	//   │  ┌─ Dock ──────────────────────────────────┐ │
	//   │  │  menu (top bar)          ← pinned top   │ │
	//   │  │──────────────────────────────────────────│ │
	//   │  │  editorPane (editor + cmdline) ← fills  │ │
	//   │  │──────────────────────────────────────────│ │
	//   │  │  footer (status bar)     ← pinned bottom │ │
	//   │  └─────────────────────────────────────────-┘ │
	//   └──────────────────────────────────────────────-┘
	// The bar orients the menu and decides which way dropdowns open; WHERE it
	// sits is this layout's decision, so the edge is pinned here to match the
	// orientation the bar was built with.
	dock := tui.NewDock()
	switch placement {
	case PlacementBottom:
		dock.Pin(tui.DockBottom, a.footer)
		dock.Pin(tui.DockBottom, a.menu.Bar())
	case PlacementLeft:
		dock.Pin(tui.DockLeft, a.menu.Bar())
		dock.Pin(tui.DockBottom, a.footer)
	case PlacementRight:
		dock.Pin(tui.DockRight, a.menu.Bar())
		dock.Pin(tui.DockBottom, a.footer)
	default:
		dock.Pin(tui.DockTop, a.menu.Bar())
		dock.Pin(tui.DockBottom, a.footer)
	}
	dock.Add(a.editorPane)

	// host wraps the dock in an OverlayHost so that dialogs and dropdown popups
	// have a surface to attach to on top of the rest of the UI without
	// disturbing the layout below.
	//
	// The menu finds this host by walking up from itself — it is an ancestor of
	// the bar — so its dropdowns need no wiring. Dialogs do: the menu is told
	// where to put them, because it is built before the host that will hold
	// them exists.
	a.host = widget.NewOverlayHost(dock)
	a.menu.AttachHost(a.host)

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

	// menu coordinates the Borland-style top menu bar, dropdowns, and modals.
	menu *TopMenu

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

	// resolver translates physical key events into semantic actions.
	resolver KeyResolver
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
	// The menu is mounted by now, so its Escape binding can be layered on.
	a.menu.InstallEscapeToLeave()
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

// HandleEvent handles periodic clock ticks, Alt menu shortcuts, and F10 menu toggling.
// Keyboard events are otherwise handled locally within EditorPane or MenuBar.
func (a *App) HandleEvent(ev tui.Event) bool {
	switch e := ev.(type) {
	case tui.TickEvent:
		a.refresh()
		return true
	case tui.FocusEvent:
		// A FocusEvent bubbles up from whichever node lost or gained focus, so
		// this is where the App learns the user has clicked into the buffer
		// while a dropdown was open. Not consumed: other components are
		// entitled to the same news.
		a.menu.CloseOnBlur()
	case tui.KeyEvent:
		// ESCAPE LEAVES THE MENU. The widget uses Escape to close one open
		// level and does not surrender focus when the last one goes, which is
		// right for a menu that might be nested inside something else. This
		// editor wants the key to walk all the way out, so the event only
		// arrives here once the menu has stopped consuming it — one Escape per
		// level, then one more that returns to the editor.
		if e.Kind == tui.KeyPress && e.Code == tui.KeyEscape && a.menu.Active() {
			a.menu.Deactivate()
			return true
		}
		if e.Kind != tui.KeyRelease && a.resolver != nil {
			if action, ok := a.resolver.Resolve(ScopeEditorNormal, e); ok {
				switch action {
				case ActionToggleMenuBar:
					a.toggleMenuBar()
					return true
				case ActionOpenMenuFile:
					a.openMenuCategory(0)
					return true
				case ActionOpenMenuOption:
					a.openMenuCategory(1)
					return true
				case ActionOpenMenuHelp:
					a.openMenuCategory(2)
					return true
				}
			}
		}
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
	case ActionToggleMenuBar:
		a.toggleMenuBar()
	case ActionOpenMenuFile:
		a.openMenuCategory(0)
	case ActionOpenMenuOption:
		a.openMenuCategory(1)
	case ActionOpenMenuHelp:
		a.openMenuCategory(2)
	}
}

// openMenuCategory activates the menu and opens the requested category.
//
// The shortcut counts categories and the model names them, so an index out of
// range is a keybinding pointing at a category this editor does not have —
// ignored rather than clamped, because opening the wrong menu is worse than
// opening none.
func (a *App) openMenuCategory(idx int) {
	id, ok := categoryAt(idx)
	if !ok {
		return
	}
	a.menu.OpenCategory(id)
}

// toggleMenuBar toggles activation of the top menu bar.
func (a *App) toggleMenuBar() { a.menu.Toggle() }

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
	if a.editorPane.Keyset() == widget.KeysetNano {
		modeStr = " NANO "
	}
	centre := a.editorPane.title()
	if a.message != "" {
		centre = a.message
	}
	timeStr := time.Now().Format("15:04:05") + " "
	a.footer.SetStatus(modeStr, centre, timeStr)
	a.ctx.MarkDirty()
}

// buildMenuModel is the editor's menu as DATA.
//
// Every row names an action identity; menuHandlers below says what each one
// does. Keeping the two apart is what lets the model be built from editor state
// (which keyset is live) without closing over the App in every row, and what
// makes the menu testable by comparing values instead of by clicking.
//
// Help is pegged to the far end of the bar with MenuItemModel.PegRight, which
// is the upstream replacement for the RightPeg flag this editor's own menu
// carried.
func (a *App) buildMenuModel() []widget.MenuItemModel {
	vim := widget.NewRadio(idKeysetVim, "1. Vim  (modal)", keysetGroup, cmd(actKeysetVim))
	vim.Hotkey, vim.HotkeyIdx = '1', 0
	vim.Checked = a.editorPane.Keyset() == widget.KeysetVim

	nano := widget.NewRadio(idKeysetNano, "2. Nano (modeless)", keysetGroup, cmd(actKeysetNano))
	nano.Hotkey, nano.HotkeyIdx = '2', 0
	nano.Checked = a.editorPane.Keyset() == widget.KeysetNano

	file := widget.NewSubmenu(idFile, "File", []widget.MenuItemModel{
		hotkeyed(widget.NewCommand("file.new", "New", cmd(actFileNew)), 'n', 0),
		hotkeyed(widget.NewCommand("file.open", "Open", cmd(actFileOpen)), 'o', 0),
		hotkeyed(widget.NewCommand("file.save", "Save", cmd(actFileSave)), 's', 0),
		hotkeyed(widget.NewCommand("file.exit", "Exit", cmd(actFileExit)), 'x', 1),
	})
	file.Hotkey, file.HotkeyIdx = 'f', 0

	keymaps := widget.NewSubmenu(idKeymaps, "Keymaps", []widget.MenuItemModel{vim, nano})
	keymaps.Hotkey, keymaps.HotkeyIdx = 'k', 0

	option := widget.NewSubmenu(idOption, "Option", []widget.MenuItemModel{keymaps})
	option.Hotkey, option.HotkeyIdx = 'o', 0

	help := widget.NewSubmenu(idHelp, "Help", []widget.MenuItemModel{
		hotkeyed(widget.NewCommand("help.about", "About", cmd(actHelpAbout)), 'a', 0),
	})
	help.Hotkey, help.HotkeyIdx = 'h', 0
	// Help sits at the far end of the bar, where it has sat in this kind of
	// application for thirty years.
	help.PegRight = true

	return []widget.MenuItemModel{file, option, help}
}

// hotkeyed sets a row's mnemonic. A helper because the constructors take the
// fields every row needs and leave the optional ones to the caller, and three
// lines per row would bury the model in assignments.
func hotkeyed(m widget.MenuItemModel, key rune, idx int) widget.MenuItemModel {
	m.Hotkey, m.HotkeyIdx = key, idx
	return m
}

// menuHandlers is the command table: what each menu identity does.
//
// The keyset rows are the only ones with real behaviour so far; the rest report
// that they are unimplemented, which is what they did before.
func (a *App) menuHandlers() map[tui.ActionID]func() {
	notImplemented := func(what string) func() {
		return func() { a.menu.OpenNotImplemented(what) }
	}
	setKeyset := func(k widget.Keyset, msg string) func() {
		return func() {
			a.editorPane.SetKeyset(k)
			a.setMessage(msg)
			a.menu.Deactivate()
		}
	}
	return map[tui.ActionID]func(){
		actFileNew:    notImplemented("File -> New"),
		actFileOpen:   notImplemented("File -> Open"),
		actFileSave:   notImplemented("File -> Save"),
		actFileExit:   func() { a.menu.OpenExitModal() },
		actHelpAbout:  notImplemented("Help -> About"),
		actKeysetVim:  setKeyset(widget.KeysetVim, "switched keymap to Vim (modal)"),
		actKeysetNano: setKeyset(widget.KeysetNano, "switched keymap to Nano (modeless)"),
	}
}
