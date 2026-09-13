package editor

import (
	"fmt"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// Active modal types managed by TopMenu.
type menuModalType uint8

const (
	modalNone menuModalType = iota
	modalExit
	modalKeymaps
	modalNotImplemented
)

var (
	// Menu bar base styling: Borland Turbo Vision light gray fill with black text.
	menuBarStyle = style.New().
			Background(style.ANSI(7)).
			Foreground(style.ANSI(0))

	// Hotkey accent: bold red letter.
	menuAccentStyle = style.New().
			Background(style.ANSI(7)).
			Foreground(style.ANSI(1)).
			Bold(true)

	// Selection highlight: autodb explorer green selection fill with bright white text.
	menuHighlightStyle = style.New().
				Background(style.ANSI(2)).
				Foreground(style.ANSI(15)).
				Bold(true)

	// Hotkey accent when selection highlight is active.
	menuHighlightAccentStyle = style.New().
					Background(style.ANSI(2)).
					Foreground(style.ANSI(11)).
					Bold(true)

	// Dropdown box border and body styling.
	dropdownBorder = style.New().
			Background(style.ANSI(7)).
			Foreground(style.ANSI(0)).
			Border(style.BorderNormal)

	// Modal scrim background stipple style.
	scrimStyle = style.New().Foreground(style.ANSI(8)).Faint(true)

	// Modal window card styling.
	modalCardStyle = style.New().
			Background(style.ANSI(0)).
			Foreground(style.ANSI(7)).
			Border(style.BorderRounded)

	modalTitleStyle = style.New().
			Foreground(style.ANSI(14)).
			Bold(true)

	buttonNormalStyle = style.New().
				Background(style.ANSI(7)).
				Foreground(style.ANSI(0))

	buttonFocusedStyle = style.New().
				Background(style.ANSI(2)).
				Foreground(style.ANSI(15)).
				Bold(true)
)

// MenuItem represents one actionable entry within a dropdown category.
type MenuItem struct {
	Name string
}

// MenuCategory represents a top-level category on the menu bar (e.g. File, Option, Help).
type MenuCategory struct {
	Name  string
	Items []MenuItem
}

// TopMenuCallbacks holds the external actions triggered by menu items and modals.
type TopMenuCallbacks struct {
	OnQuit          func()
	OnSetKeyset     func(widget.Keyset)
	OnStatusMessage func(string)
	OnRestoreFocus  func()
}

// TopMenu is the Borland Turbo Vision / classic Macintosh-style top menu bar subsystem.
// It consists of two coordinated components:
//  1. MenuBar: The persistent 1-row header bar pinned to DockTop.
//  2. MenuOverlay: The overlay layer on OverlayHost that hosts dropdown popups and modal dialogs.
//
// # Architectural Layout & Hierarchy
//
//	┌─ OverlayHost (Modal Layer) ──────────────────────────────────┐
//	│  ┌─ MenuOverlay (Dropdown Popups & Centered Modals) ───────┐ │
//	│  │  ┌─ Dropdown / Modal Card ────────────────────────────┐ │ │
//	│  │  │  File -> New, Open, Save, Exit                     │ │ │
//	│  │  └────────────────────────────────────────────────────┘ │ │
//	│  └─────────────────────────────────────────────────────────┘ │
//	│  ┌─ Dock ──────────────────────────────────────────────────┐ │
//	│  │  MenuBar: [ File ] [ Option ]                    [ Help ]│ │ ← Pinned DockTop
//	│  │─────────────────────────────────────────────────────────│ │
//	│  │  EditorPane (Buffer + Floating Command Line)            │ │
//	│  │─────────────────────────────────────────────────────────│ │
//	│  │  Footer (Mode, Status, Clock)                           │ │ ← Pinned DockBottom
//	│  └─────────────────────────────────────────────────────────┘ │
//	└──────────────────────────────────────────────────────────────┘
type TopMenu struct {
	cb TopMenuCallbacks

	// categories holds the static menu definition.
	categories []MenuCategory

	// Menu bar state
	active           bool
	selectedCategory int

	// Dropdown popup state
	dropdownOpen bool
	selectedItem int

	// Modal state
	modal           menuModalType
	modalMsg        string // message for modalNotImplemented
	exitChoice      int    // 0 = Yes, 1 = No
	selectedKeymap  int    // 0 = Vim, 1 = Nano
	currentKeyset   widget.Keyset

	bar     *MenuBar
	overlay *MenuOverlay
}

// newTopMenu constructs the TopMenu subsystem with its categories and child components.
func newTopMenu(cb TopMenuCallbacks) *TopMenu {
	tm := &TopMenu{
		cb: cb,
		categories: []MenuCategory{
			{
				Name: "File",
				Items: []MenuItem{
					{Name: "New"},
					{Name: "Open"},
					{Name: "Save"},
					{Name: "Exit"},
				},
			},
			{
				Name: "Option",
				Items: []MenuItem{
					{Name: "Keymaps"},
				},
			},
			{
				Name: "Help",
				Items: []MenuItem{
					{Name: "About"},
				},
			},
		},
		currentKeyset: widget.KeysetVim,
	}

	tm.bar = &MenuBar{menu: tm}
	tm.overlay = &MenuOverlay{menu: tm}
	return tm
}

// Bar returns the 1-row header bar component for mounting in DockTop.
func (tm *TopMenu) Bar() *MenuBar {
	return tm.bar
}

// Overlay returns the overlay layer component for mounting on OverlayHost.
func (tm *TopMenu) Overlay() *MenuOverlay {
	return tm.overlay
}

// Active reports whether the menu bar, an open dropdown, or an open modal has focus.
func (tm *TopMenu) Active() bool {
	return tm.active || tm.modal != modalNone
}

// DropdownOpen reports whether a dropdown menu is currently visible.
func (tm *TopMenu) DropdownOpen() bool {
	return tm.dropdownOpen
}

// ModalActive reports whether a modal is currently displayed.
func (tm *TopMenu) ModalActive() bool {
	return tm.modal != modalNone
}

// Activate engages the menu bar, setting the cursor on the File menu.
func (tm *TopMenu) Activate(ctx *tui.Context) {
	tm.active = true
	tm.dropdownOpen = false
	tm.selectedCategory = 0
	tm.selectedItem = 0
	if ctx != nil {
		ctx.FocusComponent(tm.bar)
		ctx.RequestLayout()
		ctx.MarkDirty()
	}
}

// Deactivate disengages the menu bar and any open dropdown, restoring focus to the editor.
func (tm *TopMenu) Deactivate(ctx *tui.Context) {
	tm.active = false
	tm.dropdownOpen = false
	tm.modal = modalNone
	if ctx != nil {
		ctx.RequestLayout()
		ctx.MarkDirty()
	}
	if tm.cb.OnRestoreFocus != nil {
		tm.cb.OnRestoreFocus()
	}
}

// Toggle toggles the active state of the menu bar.
func (tm *TopMenu) Toggle(ctx *tui.Context) {
	if tm.Active() {
		tm.Deactivate(ctx)
	} else {
		tm.Activate(ctx)
	}
}

// executeItem dispatches the semantic action for the chosen dropdown item.
func (tm *TopMenu) executeItem(catIdx, itemIdx int, ctx *tui.Context) {
	tm.dropdownOpen = false
	cat := tm.categories[catIdx]
	item := cat.Items[itemIdx]

	switch cat.Name {
	case "File":
		if item.Name == "Exit" {
			tm.openExitModal(ctx)
			return
		}
		tm.openNotImplemented(fmt.Sprintf("File -> %s", item.Name), ctx)
	case "Option":
		if item.Name == "Keymaps" {
			tm.openKeymapsModal(ctx)
			return
		}
		tm.openNotImplemented(fmt.Sprintf("Option -> %s", item.Name), ctx)
	case "Help":
		tm.openNotImplemented(fmt.Sprintf("Help -> %s", item.Name), ctx)
	default:
		tm.openNotImplemented(item.Name, ctx)
	}
}

func (tm *TopMenu) openExitModal(ctx *tui.Context) {
	tm.modal = modalExit
	tm.exitChoice = 0 // default: Yes
	if ctx != nil {
		ctx.RequestLayout()
		ctx.MarkDirty()
	}
}

func (tm *TopMenu) openKeymapsModal(ctx *tui.Context) {
	tm.modal = modalKeymaps
	if tm.currentKeyset == widget.KeysetNano {
		tm.selectedKeymap = 1
	} else {
		tm.selectedKeymap = 0
	}
	if ctx != nil {
		ctx.RequestLayout()
		ctx.MarkDirty()
	}
}

func (tm *TopMenu) openNotImplemented(msg string, ctx *tui.Context) {
	tm.modal = modalNotImplemented
	tm.modalMsg = msg
	if ctx != nil {
		ctx.RequestLayout()
		ctx.MarkDirty()
	}
}

// MenuBar is the single-row strip pinned to the top of the application dock.
type MenuBar struct {
	ctx  *tui.Context
	menu *TopMenu
}

// Init mounts the menu bar into the TUI context.
func (mb *MenuBar) Init(ctx *tui.Context) {
	mb.ctx = ctx
}

// Layout sizes the menu bar to fill the full width at exactly 1 row height.
func (mb *MenuBar) Layout(c tui.Constraints) tui.Size {
	return c.Constrain(tui.Size{W: c.MaxW, H: 1})
}

// Render draws the Borland Turbo Vision menu strip.
func (mb *MenuBar) Render(s tui.Surface) {
	sz := s.Size()
	if sz.W <= 0 || sz.H <= 0 {
		return
	}

	// 1. Fill entire bar background with light gray.
	s.Fill(tui.Rect{X: 0, Y: 0, W: sz.W, H: 1}, " ", menuBarStyle)

	// 2. Render Left menus: File, Option
	x := 1
	for i := 0; i < 2 && i < len(mb.menu.categories); i++ {
		cat := mb.menu.categories[i]
		isSel := mb.menu.active && mb.menu.selectedCategory == i
		w := mb.renderCategoryItem(s, x, cat.Name, isSel)
		x += w + 1
	}

	// 3. Render Help menu: Pegged to the right edge.
	if len(mb.menu.categories) >= 3 {
		helpCat := mb.menu.categories[2]
		helpLabel := " " + helpCat.Name + " "
		helpW := len(helpLabel)
		helpX := sz.W - helpW - 1
		if helpX < x {
			helpX = x
		}
		isSel := mb.menu.active && mb.menu.selectedCategory == 2
		mb.renderCategoryItem(s, helpX, helpCat.Name, isSel)
	}
}

// renderCategoryItem draws a single menu item (e.g. " File ") with an accented hotkey.
func (mb *MenuBar) renderCategoryItem(s tui.Surface, x int, name string, isSel bool) int {
	label := " " + name + " "
	st := menuBarStyle
	accSt := menuAccentStyle
	if isSel {
		st = menuHighlightStyle
		accSt = menuHighlightAccentStyle
	}

	// Draw background and outer space
	s.SetCell(x, 0, " ", st)
	// First character is accented hotkey
	firstRune := string([]rune(name)[0])
	s.SetCell(x+1, 0, firstRune, accSt)
	// Remaining characters
	rest := string([]rune(name)[1:]) + " "
	drawText(s, x+2, 0, rest, st)

	return len(label)
}

// HandleEvent manages keyboard navigation across the top menu bar.
func (mb *MenuBar) HandleEvent(ev tui.Event) bool {
	ke, ok := ev.(tui.KeyEvent)
	if !ok || ke.Kind == tui.KeyRelease {
		return false
	}

	ctx := mb.ctx

	// Global F10 toggles menu bar
	if ke.Code == tui.KeyF10 {
		mb.menu.Toggle(ctx)
		return true
	}

	// When a modal is open, delegate event to modal handler
	if mb.menu.modal != modalNone {
		return mb.handleModalKey(ke, ctx)
	}

	if !mb.menu.active {
		return false
	}

	// When a dropdown is open:
	if mb.menu.dropdownOpen {
		return mb.handleDropdownKey(ke, ctx)
	}

	// When menu bar is active (dropdown closed):
	switch ke.Code {
	case tui.KeyEscape:
		mb.menu.Deactivate(ctx)
		return true
	case tui.KeyLeft, 'h':
		mb.menu.selectedCategory = (mb.menu.selectedCategory - 1 + len(mb.menu.categories)) % len(mb.menu.categories)
		mb.markDirty()
		return true
	case tui.KeyRight, 'l':
		mb.menu.selectedCategory = (mb.menu.selectedCategory + 1) % len(mb.menu.categories)
		mb.markDirty()
		return true
	case tui.KeyDown, tui.KeyEnter, ' ', 'j':
		mb.menu.dropdownOpen = true
		mb.menu.selectedItem = 0
		mb.requestLayout()
		mb.markDirty()
		return true
	}

	return false
}

func (mb *MenuBar) markDirty() {
	if mb.ctx != nil {
		mb.ctx.MarkDirty()
	}
}

func (mb *MenuBar) requestLayout() {
	if mb.ctx != nil {
		mb.ctx.RequestLayout()
	}
}

func (mb *MenuBar) handleDropdownKey(ke tui.KeyEvent, ctx *tui.Context) bool {
	cat := mb.menu.categories[mb.menu.selectedCategory]
	itemCount := len(cat.Items)

	switch ke.Code {
	case tui.KeyEscape:
		mb.menu.dropdownOpen = false
		mb.requestLayout()
		mb.markDirty()
		return true
	case tui.KeyUp, 'k':
		if itemCount > 0 {
			mb.menu.selectedItem = (mb.menu.selectedItem - 1 + itemCount) % itemCount
			mb.markDirty()
		}
		return true
	case tui.KeyDown, 'j':
		if itemCount > 0 {
			mb.menu.selectedItem = (mb.menu.selectedItem + 1) % itemCount
			mb.markDirty()
		}
		return true
	case tui.KeyLeft, 'h':
		mb.menu.selectedCategory = (mb.menu.selectedCategory - 1 + len(mb.menu.categories)) % len(mb.menu.categories)
		mb.menu.selectedItem = 0
		mb.requestLayout()
		mb.markDirty()
		return true
	case tui.KeyRight, 'l':
		mb.menu.selectedCategory = (mb.menu.selectedCategory + 1) % len(mb.menu.categories)
		mb.menu.selectedItem = 0
		mb.requestLayout()
		mb.markDirty()
		return true
	case tui.KeyEnter, ' ':
		if itemCount > 0 {
			mb.menu.executeItem(mb.menu.selectedCategory, mb.menu.selectedItem, ctx)
		}
		return true
	}

	return false
}

func (mb *MenuBar) handleModalKey(ke tui.KeyEvent, ctx *tui.Context) bool {
	switch mb.menu.modal {
	case modalExit:
		switch ke.Code {
		case tui.KeyEscape, 'n', 'N':
			mb.menu.Deactivate(ctx)
			return true
		case tui.KeyLeft, tui.KeyRight, tui.KeyTab, 'h', 'l':
			mb.menu.exitChoice = 1 - mb.menu.exitChoice
			mb.markDirty()
			return true
		case 'y', 'Y':
			mb.menu.modal = modalNone
			if mb.menu.cb.OnQuit != nil {
				mb.menu.cb.OnQuit()
			}
			return true
		case tui.KeyEnter:
			if mb.menu.exitChoice == 0 { // Yes
				mb.menu.modal = modalNone
				if mb.menu.cb.OnQuit != nil {
					mb.menu.cb.OnQuit()
				}
			} else { // No
				mb.menu.Deactivate(ctx)
			}
			return true
		}
	case modalKeymaps:
		switch ke.Code {
		case tui.KeyEscape:
			mb.menu.Deactivate(ctx)
			return true
		case tui.KeyUp, tui.KeyDown, tui.KeyTab, 'j', 'k':
			mb.menu.selectedKeymap = 1 - mb.menu.selectedKeymap
			mb.markDirty()
			return true
		case '1':
			mb.menu.selectedKeymap = 0
			mb.commitKeymap(ctx)
			return true
		case '2':
			mb.menu.selectedKeymap = 1
			mb.commitKeymap(ctx)
			return true
		case tui.KeyEnter:
			mb.commitKeymap(ctx)
			return true
		}
	case modalNotImplemented:
		switch ke.Code {
		case tui.KeyEscape, tui.KeyEnter, ' ':
			mb.menu.Deactivate(ctx)
			return true
		}
	}
	return false
}

func (mb *MenuBar) commitKeymap(ctx *tui.Context) {
	if mb.menu.selectedKeymap == 1 {
		mb.menu.currentKeyset = widget.KeysetNano
		if mb.menu.cb.OnSetKeyset != nil {
			mb.menu.cb.OnSetKeyset(widget.KeysetNano)
		}
		if mb.menu.cb.OnStatusMessage != nil {
			mb.menu.cb.OnStatusMessage("switched keymap to Nano (modeless)")
		}
	} else {
		mb.menu.currentKeyset = widget.KeysetVim
		if mb.menu.cb.OnSetKeyset != nil {
			mb.menu.cb.OnSetKeyset(widget.KeysetVim)
		}
		if mb.menu.cb.OnStatusMessage != nil {
			mb.menu.cb.OnStatusMessage("switched keymap to Vim (modal)")
		}
	}
	mb.menu.Deactivate(ctx)
}

// AcceptsFocus implements tui.Focusable.
func (mb *MenuBar) AcceptsFocus() bool {
	return true
}

// MenuOverlay renders the dropdown menu cards and modal dialogs on OverlayHost.
type MenuOverlay struct {
	ctx  *tui.Context
	menu *TopMenu
}

// Init mounts the overlay into the TUI context.
func (mo *MenuOverlay) Init(ctx *tui.Context) {
	mo.ctx = ctx
}

// Layout sizes the overlay layer to match container bounds when active.
func (mo *MenuOverlay) Layout(c tui.Constraints) tui.Size {
	if !mo.menu.dropdownOpen && mo.menu.modal == modalNone {
		return tui.Size{}
	}
	return c.Constrain(tui.Size{W: c.MaxW, H: c.MaxH})
}

// Render paints dropdowns or active modals on the overlay surface.
func (mo *MenuOverlay) Render(s tui.Surface) {
	sz := s.Size()
	if sz.W <= 0 || sz.H <= 0 {
		return
	}

	if mo.menu.modal != modalNone {
		mo.renderModal(s, sz)
		return
	}

	if mo.menu.dropdownOpen {
		mo.renderDropdown(s, sz)
	}
}

// renderDropdown paints the active menu category's dropdown card.
func (mo *MenuOverlay) renderDropdown(s tui.Surface, sz tui.Size) {
	catIdx := mo.menu.selectedCategory
	if catIdx < 0 || catIdx >= len(mo.menu.categories) {
		return
	}
	cat := mo.menu.categories[catIdx]

	// Determine X coordinate for dropdown box based on category.
	var x int
	switch catIdx {
	case 0: // File
		x = 1
	case 1: // Option
		x = 7
	case 2: // Help (pegged to right)
		x = sz.W - 14
		if x < 1 {
			x = 1
		}
	}

	boxW := 15
	boxH := len(cat.Items) + 2

	// Render framed dropdown box
	boxRect := tui.Rect{X: x, Y: 1, W: boxW, H: boxH}
	renderBoxFrame(s, boxRect, cat.Name, dropdownBorder)

	// Render items inside
	for i, it := range cat.Items {
		itemY := 2 + i
		isSel := i == mo.menu.selectedItem
		itemSt := menuBarStyle
		accSt := menuAccentStyle
		if isSel {
			itemSt = menuHighlightStyle
			accSt = menuHighlightAccentStyle
		}

		// Clear row inside box
		s.Fill(tui.Rect{X: x + 1, Y: itemY, W: boxW - 2, H: 1}, " ", itemSt)

		// First char hotkey accent
		first := string([]rune(it.Name)[0])
		s.SetCell(x+2, itemY, first, accSt)

		// Remaining name
		rest := string([]rune(it.Name)[1:])
		drawText(s, x+3, itemY, rest, itemSt)
	}
}

// renderModal paints centered modals with a dimmed scrim background.
func (mo *MenuOverlay) renderModal(s tui.Surface, sz tui.Size) {
	// 1. Scrim the background
	s.Fill(tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H}, "░", scrimStyle)

	switch mo.menu.modal {
	case modalExit:
		cardW, cardH := 36, 7
		cx := (sz.W - cardW) / 2
		cy := (sz.H - cardH) / 2
		rect := tui.Rect{X: cx, Y: cy, W: cardW, H: cardH}
		renderBoxFrame(s, rect, "Exit", modalCardStyle)

		s.Fill(tui.Rect{X: cx + 1, Y: cy + 1, W: cardW - 2, H: cardH - 2}, " ", modalCardStyle)
		text := "Are you sure to quit?"
		drawText(s, cx+(cardW-len(text))/2, cy+2, text, modalCardStyle.Bold(true))

		// Buttons: [ Yes ]   [ No ]
		yesSt := buttonNormalStyle
		noSt := buttonNormalStyle
		if mo.menu.exitChoice == 0 {
			yesSt = buttonFocusedStyle
		} else {
			noSt = buttonFocusedStyle
		}
		drawText(s, cx+8, cy+4, "[ Yes ]", yesSt)
		drawText(s, cx+20, cy+4, "[ No ]", noSt)

	case modalKeymaps:
		cardW, cardH := 38, 9
		cx := (sz.W - cardW) / 2
		cy := (sz.H - cardH) / 2
		rect := tui.Rect{X: cx, Y: cy, W: cardW, H: cardH}
		renderBoxFrame(s, rect, "Keymaps", modalCardStyle)

		s.Fill(tui.Rect{X: cx + 1, Y: cy + 1, W: cardW - 2, H: cardH - 2}, " ", modalCardStyle)
		drawText(s, cx+3, cy+1, "Select Editor Keymap:", modalTitleStyle)

		opt1 := " 1. Vim  (modal editing)   "
		opt2 := " 2. Nano (modeless editing)"
		st1 := modalCardStyle
		st2 := modalCardStyle
		if mo.menu.selectedKeymap == 0 {
			st1 = buttonFocusedStyle
		} else {
			st2 = buttonFocusedStyle
		}
		drawText(s, cx+4, cy+3, opt1, st1)
		drawText(s, cx+4, cy+4, opt2, st2)

		footer := "[Enter] Select    [Esc] Cancel"
		drawText(s, cx+(cardW-len(footer))/2, cy+6, footer, modalCardStyle)

	case modalNotImplemented:
		cardW, cardH := 36, 7
		cx := (sz.W - cardW) / 2
		cy := (sz.H - cardH) / 2
		rect := tui.Rect{X: cx, Y: cy, W: cardW, H: cardH}
		renderBoxFrame(s, rect, "Not Implemented", modalCardStyle)

		s.Fill(tui.Rect{X: cx + 1, Y: cy + 1, W: cardW - 2, H: cardH - 2}, " ", modalCardStyle)
		msg := mo.menu.modalMsg
		if len(msg) > cardW-4 {
			msg = msg[:cardW-7] + "..."
		}
		fullMsg := msg + " is not implemented"
		if len(fullMsg) > cardW-4 {
			fullMsg = msg
		}
		drawText(s, cx+(cardW-len(fullMsg))/2, cy+2, fullMsg, modalCardStyle)

		drawText(s, cx+(cardW-6)/2, cy+4, "[ OK ]", buttonFocusedStyle)
	}
}

// renderBoxFrame draws a titled box frame on the given rectangle.
func renderBoxFrame(s tui.Surface, r tui.Rect, title string, st style.Style) {
	if r.W < 2 || r.H < 2 {
		return
	}

	// Corners
	s.SetCell(r.X, r.Y, "┌", st)
	s.SetCell(r.X+r.W-1, r.Y, "┐", st)
	s.SetCell(r.X, r.Y+r.H-1, "└", st)
	s.SetCell(r.X+r.W-1, r.Y+r.H-1, "┘", st)

	// Top and bottom borders
	for x := r.X + 1; x < r.X+r.W-1; x++ {
		s.SetCell(x, r.Y, "─", st)
		s.SetCell(x, r.Y+r.H-1, "─", st)
	}

	// Left and right borders
	for y := r.Y + 1; y < r.Y+r.H-1; y++ {
		s.SetCell(r.X, y, "│", st)
		s.SetCell(r.X+r.W-1, y, "│", st)
	}

	// Title
	if title != "" && r.W > len(title)+4 {
		titleText := " " + title + " "
		drawText(s, r.X+2, r.Y, titleText, st.Bold(true))
	}
}

// drawText renders a plain string horizontally starting at (x, y).
func drawText(s tui.Surface, x, y int, text string, st style.Style) {
	col := x
	for _, r := range text {
		s.SetCell(col, y, string(r), st)
		col++
	}
}

// HandleEvent forwards events to MenuBar when active.
func (mo *MenuOverlay) HandleEvent(ev tui.Event) bool {
	if !mo.menu.Active() {
		return false
	}
	return mo.menu.bar.HandleEvent(ev)
}
