package editor

// THE APPLICATION MENU.
//
// This file used to be a menu WIDGET: 1,362 lines of rows, levels, popups,
// selection, hit-testing and keyboard handling, all of it a second
// implementation of what golib/tui now ships. That is gone. What remains here
// is the part that was never golib's to own — which category names this editor
// has, which command each row runs, and what the menu does to the rest of the
// application when it opens and closes.
//
// The split is worth stating because it is the whole point of the migration:
//
//   - golib/tui owns BEHAVIOUR: rows, submenu levels, selection, hotkeys,
//     hit-testing, popup placement, focus.
//   - this file owns POLICY: the model, the command table, and the coupling to
//     the editor pane (restoring focus, posting status messages, quitting).
//
// A row's Action is an IDENTITY, not a closure. The model can therefore be
// built, compared and tested as data, and the executor below is the single
// place where a menu row turns into an effect.

import (
	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// MenuPlacement is the configured edge for the menu bar. It stays a string
// because it comes from the config file, where "top" is friendlier than an
// integer; barPlacement translates it at construction.
type MenuPlacement string

const (
	PlacementTop    MenuPlacement = "top"
	PlacementBottom MenuPlacement = "bottom"
	PlacementLeft   MenuPlacement = "left"
	PlacementRight  MenuPlacement = "right"
)

// barPlacement maps the configured edge onto the widget's orientation.
//
// An unrecognised value becomes Top rather than an error: the menu bar is not
// where a typo in a config file should stop the editor from starting.
func barPlacement(p MenuPlacement) widget.BarPlacement {
	switch p {
	case PlacementBottom:
		return widget.BarPlacementBottom
	case PlacementLeft:
		return widget.BarPlacementLeft
	case PlacementRight:
		return widget.BarPlacementRight
	default:
		return widget.BarPlacementTop
	}
}

// Menu command identities. Every row that does something names one of these,
// and the executor is the only place that knows what they mean.
const (
	actFileNew    tui.ActionID = "editor.file.new"
	actFileOpen   tui.ActionID = "editor.file.open"
	actFileSave   tui.ActionID = "editor.file.save"
	actFileExit   tui.ActionID = "editor.file.exit"
	actKeysetVim  tui.ActionID = "editor.option.keyset.vim"
	actKeysetNano tui.ActionID = "editor.option.keyset.nano"
	actHelpAbout  tui.ActionID = "editor.help.about"
)

// Row identities, needed because opening a category by name is how the
// keyboard shortcuts (Alt-F and friends) reach the menu.
const (
	idFile       widget.ItemID = "file"
	idOption     widget.ItemID = "option"
	idHelp       widget.ItemID = "help"
	idKeymaps    widget.ItemID = "option.keymaps"
	idKeysetVim  widget.ItemID = "option.keymaps.vim"
	idKeysetNano widget.ItemID = "option.keymaps.nano"
)

// keysetGroup ties the two keymap rows into one radio set, so choosing either
// clears the other without this file tracking which was last picked.
const keysetGroup = "keyset"

// menuCommand is a row's intent. tui.Action is an interface carrying identity
// only, which is what lets the model stay data.
type menuCommand struct{ id tui.ActionID }

func (c menuCommand) ActionID() tui.ActionID { return c.id }

// cmd is shorthand for the model below.
func cmd(id tui.ActionID) tui.Action { return menuCommand{id} }

// TopMenuCallbacks is how the menu reaches the rest of the editor. They are
// callbacks rather than a back-pointer to App so the menu can be built and
// driven in a test without one.
type TopMenuCallbacks struct {
	// OnQuit ends the program. Called only after the exit dialog is confirmed.
	OnQuit func()
	// OnStatusMessage posts a transient message to the footer.
	OnStatusMessage func(string)
	// OnRestoreFocus hands focus back to the editor pane. Called whenever the
	// menu finishes, because a menu that closes without returning focus leaves
	// the user typing into nothing.
	OnRestoreFocus func()
}

// TopMenu is the editor's application menu: the upstream Menu and MenuBar, the
// command table over them, and the dialogs the menu opens.
type TopMenu struct {
	menu      *widget.Menu
	bar       *widget.MenuBar
	cb        TopMenuCallbacks
	placement MenuPlacement

	// host is where dialogs are opened. Set by AttachHost once the App has
	// built its OverlayHost; until then OpenModal has nowhere to put a dialog.
	host *widget.OverlayHost
	// modal is the dialog currently up, or nil. One at a time: this editor has
	// no flow that stacks dialogs, and tracking one is honest about that.
	modal *widget.Modal

	// handlers is the command table: identity to effect.
	handlers map[tui.ActionID]func()
	// keysetOf lets the keymap rows report which keyset they select without a
	// second switch.
	keysetOf map[tui.ActionID]widget.Keyset
}

// NewTopMenu builds the menu, its bar and its command table.
//
// The model is supplied separately by SetModel, because the rows depend on
// editor state (which keyset is current) that the caller assembles.
func NewTopMenu(placement MenuPlacement, cb TopMenuCallbacks) *TopMenu {
	tm := &TopMenu{cb: cb, placement: placement}
	tm.keysetOf = map[tui.ActionID]widget.Keyset{
		actKeysetVim:  widget.KeysetVim,
		actKeysetNano: widget.KeysetNano,
	}
	tm.menu = widget.NewMenu(
		widget.WithMenuStyle(defaultMenuStyle),
		// This editor is Vim-shaped, so hjkl navigates the menu as well. A
		// declared row hotkey still wins over the alias, so the File/Option/Help
		// mnemonics stay reachable.
		widget.WithMenuVimNavigation(true),
		widget.WithActionExecutor(tm.execute),
	)
	tm.bar = widget.NewMenuBar(tm.menu, widget.WithBarPlacement(barPlacement(placement)))
	return tm
}

// Bar is the component to dock. The bar orients the menu and decides which way
// dropdowns open; WHERE it sits on screen is the layout's business, so the
// caller pins it to the matching edge itself.
func (tm *TopMenu) Bar() *widget.MenuBar { return tm.bar }

// Menu exposes the underlying widget, for tests that drive rows directly.
func (tm *TopMenu) Menu() *widget.Menu { return tm.menu }

// Placement reports the configured edge.
func (tm *TopMenu) Placement() MenuPlacement { return tm.placement }

// AttachHost tells the menu where dialogs go. Separate from construction
// because the OverlayHost is built around the layout the bar is already part
// of, so it does not exist yet when the menu is made.
func (tm *TopMenu) AttachHost(h *widget.OverlayHost) { tm.host = h }

// SetHandlers installs the command table. Every identity a row names must have
// an entry, or activating that row does nothing.
func (tm *TopMenu) SetHandlers(h map[tui.ActionID]func()) { tm.handlers = h }

// SetModel installs the rows and reports a malformed model rather than
// panicking, since the model is assembled from editor state.
func (tm *TopMenu) SetModel(items []widget.MenuItemModel) error {
	return tm.menu.SetModel(items)
}

// SetChecked marks one keymap row and clears the other. The rows are radios in
// one group, so upstream clears the siblings; this is for seeding the initial
// state before the user has touched anything.
func (tm *TopMenu) SetChecked(id widget.ItemID, v bool) bool {
	return tm.menu.SetChecked(id, v)
}

// execute turns an activated row into an effect.
//
// Returning false for an unknown identity is deliberate: it tells the runtime
// the action was not handled here, which is the honest answer and keeps a typo
// in the model visible instead of silently swallowed.
func (tm *TopMenu) execute(inv tui.ActionInvocation) bool {
	if inv.Action == nil {
		return false
	}
	fn, ok := tm.handlers[inv.Action.ActionID()]
	if !ok || fn == nil {
		return false
	}
	fn()
	return true
}

// Active reports whether the menu is the surface the user is driving.
//
// ASKED, NOT REMEMBERED. This was a bool the menu set in Activate and cleared
// in Deactivate, which is only true while nothing else moves focus — and plenty
// does: the runtime's own focus ring, a dialog opening over the top, a click
// elsewhere. A remembered flag then reports a menu that is still "active" after
// the user has tabbed away. Focus is the thing the question is really about, so
// the question is put to the runtime.
func (tm *TopMenu) Active() bool {
	ctx := tm.menu.Context()
	return ctx != nil && ctx.Focused()
}

// ModalActive reports whether a dialog is up.
func (tm *TopMenu) ModalActive() bool { return tm.modal != nil }

// ActiveModal is the dialog currently up, or nil.
func (tm *TopMenu) ActiveModal() *widget.Modal { return tm.modal }

// CloseOnBlur closes the cascade when focus leaves the menu.
//
// APPLICATION POLICY, NOT THE WIDGET'S. golib deliberately keeps a level open
// across a focus loss — a renderer drawing a cascade must still see it, and an
// involuntary loss is not a decision the user made about the menu. This editor
// wants the other rule: clicking into the buffer means "I am done with the
// menu", and a dropdown left hanging over the text is covering the line the
// user just aimed at.
//
// Called from App.HandleEvent, which sees the FocusEvent bubble past on its way
// up from the menu.
func (tm *TopMenu) CloseOnBlur() {
	if tm.menu.OpenLevels() == 0 || tm.Active() {
		return
	}
	tm.menu.Close()
	if ctx := tm.menu.Context(); ctx != nil {
		ctx.MarkDirty()
	}
}

// Activate gives the menu focus.
func (tm *TopMenu) Activate() {
	ctx := tm.menu.Context()
	if ctx == nil {
		return
	}
	// ALWAYS FROM THE FIRST CATEGORY. The selection persists across a close, so
	// without this, reaching the menu again resumed wherever the last visit
	// ended — press F10 after using Help and the bar comes up on Help, which is
	// not where anybody expects to start.
	if !tm.Active() {
		if id, ok := categoryAt(0); ok {
			tm.menu.Select(id)
		}
	}
	ctx.RequestFocus()
}

// Deactivate closes any open level and hands focus back to the editor.
//
// Both halves matter. Closing the levels without restoring focus leaves the
// keyboard pointed at a menu that is no longer on screen.
func (tm *TopMenu) Deactivate() {
	tm.menu.Close()
	if tm.cb.OnRestoreFocus != nil {
		tm.cb.OnRestoreFocus()
	}
}

// Toggle activates the menu, or deactivates it if it already holds focus.
func (tm *TopMenu) Toggle() {
	if tm.Active() {
		tm.Deactivate()
		return
	}
	tm.Activate()
}

// OpenCategory focuses the menu and opens one top-level dropdown.
//
// An unopenable category — absent, hidden, disabled, or not yet laid out —
// leaves the menu focused with nothing dropped rather than failing: the
// shortcut behaves as "go to the menu", which is what a user pressing Alt-F at
// a bad moment wants over nothing at all.
func (tm *TopMenu) OpenCategory(id widget.ItemID) {
	tm.Activate()
	// NO Select AFTERWARDS. Opening a level hands the selection to that level's
	// first row, which is what puts the highlight on "New" when File drops
	// down. Selecting the category again here dragged it back to the bar, so
	// the dropdown opened with nothing in it highlighted and the arrow keys
	// appeared to do nothing.
	_ = tm.menu.Open(id)
}

// categoryAt maps a positional shortcut onto a row identity. The keyboard
// layer counts categories; the model names them.
func categoryAt(idx int) (widget.ItemID, bool) {
	switch idx {
	case 0:
		return idFile, true
	case 1:
		return idOption, true
	case 2:
		return idHelp, true
	}
	return "", false
}

// status posts a transient footer message, if anyone is listening.
func (tm *TopMenu) status(msg string) {
	if tm.cb.OnStatusMessage != nil {
		tm.cb.OnStatusMessage(msg)
	}
}
