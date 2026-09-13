package editor

import (
	"fmt"
	"strings"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// MenuPlacement configures where the menu bar is docked within the application.
type MenuPlacement string

const (
	PlacementTop    MenuPlacement = "top"
	PlacementBottom MenuPlacement = "bottom"
	PlacementLeft   MenuPlacement = "left"
	PlacementRight  MenuPlacement = "right"
)

// Active modal types managed by TopMenu.
type menuModalType uint8

const (
	modalNone menuModalType = iota
	modalExit
	modalKeymaps
	modalNotImplemented
)

// MenuCategory represents a top-level category on the menu bar (e.g. File, Option, Help).
type MenuCategory struct {
	Name      string
	Hotkey    rune
	HotkeyIdx int
	Items     []*MenuItem
}

// TopMenuCallbacks holds the external actions triggered by menu items and modals.
type TopMenuCallbacks struct {
	OnQuit          func()
	OnSetKeyset     func(widget.Keyset)
	OnStatusMessage func(string)
	OnRestoreFocus  func()
}

// TopMenu is the Borland Turbo Vision / classic Macintosh-style menu bar subsystem.
// It supports configurable docking placement (Top, Bottom, Left, Right), Alt hotkeys,
// mnemonic navigation, and cascading submenus.
//
// # Architectural Layout & Hierarchy
//
//	┌─ OverlayHost (Modal Layer) ──────────────────────────────────┐
//	│  ┌─ MenuOverlay (Dropdown Popups & Cascading Submenus) ─────┐│
//	│  │  ┌─ Dropdown Card ──┐┌─ Cascading Submenu Card ────────┐ ││
//	│  │  │  Option          ││  • 1. Vim   (modal)             │ ││
//	│  │  │  Keymaps       ► ││    2. Nano  (modeless)          │ ││
//	│  │  └──────────────────┘└─────────────────────────────────┘ ││
//	│  └──────────────────────────────────────────────────────────┘│
//	│  ┌─ Dock ──────────────────────────────────────────────────┐ │
//	│  │  MenuBar: [ File ] [ Option ]                   [ Help ]│ │ ← Configurable Edge
//	│  │─────────────────────────────────────────────────────────│ │
//	│  │  EditorPane (Buffer + Floating Command Line)            │ │
//	│  │─────────────────────────────────────────────────────────│ │
//	│  │  Footer (Mode, Status, Clock)                           │ │ ← Pinned DockBottom
//	│  └─────────────────────────────────────────────────────────┘ │
//	└──────────────────────────────────────────────────────────────┘
type TopMenu struct {
	cb TopMenuCallbacks

	// placement sets the dock edge: Top, Bottom, Left, or Right.
	placement MenuPlacement

	// categories holds the menu definition.
	categories []MenuCategory

	// Menu bar state
	active           bool
	selectedCategory int

	// Dropdown popup state
	dropdownOpen bool
	selectedItem int

	// Cascading submenu state
	submenuOpen     bool
	selectedSubItem int

	// Modal state
	modal          menuModalType
	modalMsg       string // message for modalNotImplemented
	exitChoice     int    // 0 = Yes, 1 = No
	selectedKeymap int    // 0 = Vim, 1 = Nano
	currentKeyset  widget.Keyset
	activeModal    *Modal

	bar     *MenuBar
	overlay *MenuOverlay

	style *MenuStyle

	resolver KeyResolver
}

// newTopMenu constructs the TopMenu subsystem with its categories and child components.
func newTopMenu(cb TopMenuCallbacks, resolver ...KeyResolver) *TopMenu {
	var res KeyResolver
	if len(resolver) > 0 && resolver[0] != nil {
		res = resolver[0]
	} else {
		res = NewDefaultKeyResolver(" ")
	}
	tm := &TopMenu{
		cb:            cb,
		resolver:      res,
		placement:     PlacementTop,
		currentKeyset: widget.KeysetVim,
	}
	tm.categories = []MenuCategory{
		{
			Name:      "File",
			Hotkey:    'f',
			HotkeyIdx: 0,
			Items: []*MenuItem{
				NewMenuItem("New", 'n', 0, func() {
					tm.openNotImplemented("File -> New", nil)
				}),
				NewMenuItem("Open", 'o', 0, func() {
					tm.openNotImplemented("File -> Open", nil)
				}),
				NewMenuItem("Save", 's', 0, func() {
					tm.openNotImplemented("File -> Save", nil)
				}),
				NewMenuItem("Exit", 'x', 1, func() {
					tm.openExitModal(nil)
				}),
			},
		},
		{
			Name:      "Option",
			Hotkey:    'o',
			HotkeyIdx: 0,
			Items: []*MenuItem{
				NewMenuItemWithSubmenu("Keymaps", 'k', 0,
					NewMenuItemWithKeyset("1. Vim  (modal)", '1', 0, widget.KeysetVim, func() {
						tm.commitKeymap(widget.KeysetVim, nil)
					}),
					NewMenuItemWithKeyset("2. Nano (modeless)", '2', 0, widget.KeysetNano, func() {
						tm.commitKeymap(widget.KeysetNano, nil)
					}),
				),
			},
		},
		{
			Name:      "Help",
			Hotkey:    'h',
			HotkeyIdx: 0,
			Items: []*MenuItem{
				NewMenuItem("About", 'a', 0, func() {
					tm.openNotImplemented("Help -> About", nil)
				}),
			},
		},
	}

	tm.bar = &MenuBar{menu: tm}
	tm.overlay = &MenuOverlay{menu: tm}
	return tm
}

// SetPlacement configures where the menu bar is mounted (top, bottom, left, right).
func (tm *TopMenu) SetPlacement(p MenuPlacement) {
	tm.placement = p
}

// Placement returns the active dock placement.
func (tm *TopMenu) Placement() MenuPlacement {
	return tm.placement
}

// MenuStyle returns the active or fallback style configuration for the menu.
func (tm *TopMenu) MenuStyle() *MenuStyle {
	if tm.style == nil {
		return defaultMenuStyle
	}
	return tm.style
}

// SetStyle configures a custom MenuStyle for the menu subsystem.
func (tm *TopMenu) SetStyle(s *MenuStyle) *TopMenu {
	tm.style = s
	if tm.bar != nil && tm.bar.ctx != nil {
		tm.bar.ctx.RequestLayout()
		tm.bar.ctx.MarkDirty()
	}
	return tm
}

// SetStyles configures individual styling attributes for the menu subsystem.
func (tm *TopMenu) SetStyles(bar, accent, highlight, highlightAccent, border style.Style) *TopMenu {
	tm.style = NewMenuStyle(bar, accent, highlight, highlightAccent, border)
	if tm.bar != nil && tm.bar.ctx != nil {
		tm.bar.ctx.RequestLayout()
		tm.bar.ctx.MarkDirty()
	}
	return tm
}

// Bar returns the menu bar component for mounting in Dock.
func (tm *TopMenu) Bar() *MenuBar {
	return tm.bar
}

// Overlay returns the overlay layer component for mounting on OverlayHost.
func (tm *TopMenu) Overlay() *MenuOverlay {
	return tm.overlay
}

// Active reports whether the menu bar, an open dropdown, or an open modal has focus.
func (tm *TopMenu) Active() bool {
	return tm.active || tm.modal != modalNone || tm.activeModal != nil
}

// DropdownOpen reports whether a dropdown menu is currently visible.
func (tm *TopMenu) DropdownOpen() bool {
	return tm.dropdownOpen
}

// SubmenuOpen reports whether a cascading submenu is currently open.
func (tm *TopMenu) SubmenuOpen() bool {
	return tm.submenuOpen
}

// ModalActive reports whether a modal is currently displayed.
func (tm *TopMenu) ModalActive() bool {
	return tm.modal != modalNone || tm.activeModal != nil
}

// ActiveModal returns the currently active standalone modal widget, if any.
func (tm *TopMenu) ActiveModal() *Modal {
	return tm.activeModal
}

// Activate engages the menu bar, setting the cursor on the File menu.
func (tm *TopMenu) Activate(ctx *tui.Context) {
	tm.active = true
	tm.dropdownOpen = false
	tm.submenuOpen = false
	tm.selectedCategory = 0
	tm.selectedItem = 0
	if ctx != nil {
		ctx.FocusComponent(tm.bar)
		ctx.RequestLayout()
		ctx.MarkDirty()
	}
}

// OpenCategory engages the menu bar and immediately drops down the requested category index.
func (tm *TopMenu) OpenCategory(catIdx int, ctx *tui.Context) {
	if catIdx < 0 || catIdx >= len(tm.categories) {
		catIdx = 0
	}
	tm.active = true
	tm.selectedCategory = catIdx
	tm.dropdownOpen = true
	tm.submenuOpen = false
	tm.selectedItem = 0
	if ctx != nil {
		ctx.FocusComponent(tm.bar)
		ctx.RequestLayout()
		ctx.MarkDirty()
	}
}

// Deactivate disengages the menu bar and any open dropdown/submenu, restoring focus to the editor.
func (tm *TopMenu) Deactivate(ctx *tui.Context) {
	if ctx == nil && tm.bar != nil {
		ctx = tm.bar.ctx
	}
	tm.active = false
	tm.dropdownOpen = false
	tm.submenuOpen = false
	tm.modal = modalNone
	tm.activeModal = nil
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

// openSubmenu opens the cascading submenu for the current item if one exists.
func (tm *TopMenu) openSubmenu(ctx *tui.Context) bool {
	cat := tm.categories[tm.selectedCategory]
	if tm.selectedItem < 0 || tm.selectedItem >= len(cat.Items) {
		return false
	}
	item := cat.Items[tm.selectedItem]
	if len(item.Submenu) == 0 {
		return false
	}
	tm.submenuOpen = true
	if tm.currentKeyset == widget.KeysetNano {
		tm.selectedSubItem = 1
	} else {
		tm.selectedSubItem = 0
	}
	if ctx != nil {
		ctx.RequestLayout()
		ctx.MarkDirty()
	}
	return true
}

// executeItem dispatches the semantic action for the chosen dropdown item.
func (tm *TopMenu) executeItem(catIdx, itemIdx int, ctx *tui.Context) {
	cat := tm.categories[catIdx]
	item := cat.Items[itemIdx]

	if item.HasSubmenu() {
		tm.openSubmenu(ctx)
		return
	}

	tm.dropdownOpen = false
	tm.submenuOpen = false

	if item.Action() != nil {
		item.Trigger()
		return
	}

	switch cat.Name {
	case "File":
		if item.Name == "Exit" {
			tm.openExitModal(ctx)
			return
		}
		tm.openNotImplemented(fmt.Sprintf("File -> %s", item.Name), ctx)
	case "Option":
		tm.openNotImplemented(fmt.Sprintf("Option -> %s", item.Name), ctx)
	case "Help":
		tm.openNotImplemented(fmt.Sprintf("Help -> %s", item.Name), ctx)
	default:
		tm.openNotImplemented(item.Name, ctx)
	}
}

func (tm *TopMenu) openExitModal(ctx *tui.Context) {
	if ctx == nil && tm.bar != nil {
		ctx = tm.bar.ctx
	}
	tm.modal = modalExit
	tm.exitChoice = 0 // default: Yes

	yesBtn := NewButton("Yes", func() {
		tm.modal = modalNone
		tm.activeModal = nil
		if tm.cb.OnQuit != nil {
			tm.cb.OnQuit()
		}
	})
	noBtn := NewButton("No", func() {
		tm.Deactivate(ctx)
	})

	tm.activeModal = NewModal("Exit", "Are you sure to quit?", yesBtn, noBtn)
	tm.activeModal.OnDismiss(func() {
		tm.Deactivate(ctx)
	})

	if ctx != nil {
		ctx.RequestLayout()
		ctx.MarkDirty()
	}
}

func (tm *TopMenu) openKeymapsModal(ctx *tui.Context) {
	if ctx == nil && tm.bar != nil {
		ctx = tm.bar.ctx
	}
	tm.modal = modalKeymaps

	vimBtn := NewButton("1. Vim (modal)", func() {
		tm.commitKeymap(widget.KeysetVim, ctx)
	})
	nanoBtn := NewButton("2. Nano (modeless)", func() {
		tm.commitKeymap(widget.KeysetNano, ctx)
	})

	initialSel := 0
	if tm.currentKeyset == widget.KeysetNano {
		initialSel = 1
	}
	tm.selectedKeymap = initialSel

	tm.activeModal = NewModal("Keymaps", "Select Editor Keymap:", vimBtn, nanoBtn)
	tm.activeModal.SetSelectedButton(initialSel)
	tm.activeModal.OnDismiss(func() {
		tm.Deactivate(ctx)
	})

	if ctx != nil {
		ctx.RequestLayout()
		ctx.MarkDirty()
	}
}

func (tm *TopMenu) commitKeymap(ks widget.Keyset, ctx *tui.Context) {
	if ctx == nil && tm.bar != nil {
		ctx = tm.bar.ctx
	}
	tm.currentKeyset = ks
	if tm.cb.OnSetKeyset != nil {
		tm.cb.OnSetKeyset(ks)
	}
	if tm.cb.OnStatusMessage != nil {
		if ks == widget.KeysetNano {
			tm.cb.OnStatusMessage("switched keymap to Nano (modeless)")
		} else {
			tm.cb.OnStatusMessage("switched keymap to Vim (modal)")
		}
	}
	tm.Deactivate(ctx)
}

func (tm *TopMenu) openNotImplemented(msg string, ctx *tui.Context) {
	if ctx == nil && tm.bar != nil {
		ctx = tm.bar.ctx
	}
	tm.modal = modalNotImplemented
	tm.modalMsg = msg

	okBtn := NewButton("OK", func() {
		tm.Deactivate(ctx)
	})

	fullMsg := msg + " is not implemented"
	tm.activeModal = NewModal("Not Implemented", fullMsg, okBtn)
	tm.activeModal.OnDismiss(func() {
		tm.Deactivate(ctx)
	})

	if ctx != nil {
		ctx.RequestLayout()
		ctx.MarkDirty()
	}
}

// MenuBar is the docked menu strip (horizontal on Top/Bottom, vertical on Left/Right).
type MenuBar struct {
	ctx  *tui.Context
	menu *TopMenu
}

// Init mounts the menu bar into the TUI context.
func (mb *MenuBar) Init(ctx *tui.Context) {
	mb.ctx = ctx
}

// Layout sizes the menu bar based on dock placement.
// Horizontal (Top/Bottom): 1 row tall, fills width.
// Vertical (Left/Right): 16 columns wide, fills height (like autodb explorer).
func (mb *MenuBar) Layout(c tui.Constraints) tui.Size {
	if mb.menu.placement == PlacementLeft || mb.menu.placement == PlacementRight {
		return c.Constrain(tui.Size{W: 16, H: c.MaxH})
	}
	return c.Constrain(tui.Size{W: c.MaxW, H: 1})
}

// MenuStyle returns the active or fallback style configuration for the menu.
func (mb *MenuBar) MenuStyle() *MenuStyle {
	return mb.menu.MenuStyle()
}

// SetStyle configures a custom MenuStyle for the menu bar and overlay.
func (mb *MenuBar) SetStyle(s *MenuStyle) *MenuBar {
	mb.menu.SetStyle(s)
	return mb
}

// SetStyles configures individual styling attributes for the menu bar and overlay.
func (mb *MenuBar) SetStyles(bar, accent, highlight, highlightAccent, border style.Style) *MenuBar {
	mb.menu.SetStyles(bar, accent, highlight, highlightAccent, border)
	return mb
}

// Render paints the menu bar.
func (mb *MenuBar) Render(s tui.Surface) {
	sz := s.Size()
	if sz.W <= 0 || sz.H <= 0 {
		return
	}

	if mb.menu.placement == PlacementLeft || mb.menu.placement == PlacementRight {
		mb.renderVertical(s, sz)
	} else {
		mb.renderHorizontal(s, sz)
	}
}

// renderHorizontal paints the classic horizontal Borland menu bar.
func (mb *MenuBar) renderHorizontal(s tui.Surface, sz tui.Size) {
	// 1. Fill entire bar background with light gray.
	s.Fill(tui.Rect{X: 0, Y: 0, W: sz.W, H: 1}, " ", mb.menu.style.Bar())

	// 2. Render Left menus: File, Option
	x := 1
	for i := 0; i < 2 && i < len(mb.menu.categories); i++ {
		cat := mb.menu.categories[i]
		isSel := mb.menu.active && mb.menu.selectedCategory == i
		w := mb.renderCategoryItemHorizontal(s, x, cat, isSel)
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
		mb.renderCategoryItemHorizontal(s, helpX, helpCat, isSel)
	}
}

// renderCategoryItemHorizontal draws a single category with hotkey accent on horizontal bar.
func (mb *MenuBar) renderCategoryItemHorizontal(s tui.Surface, x int, cat MenuCategory, isSel bool) int {
	label := " " + cat.Name + " "
	st := mb.menu.style.ItemStyle(isSel)
	accSt := mb.menu.style.AccentStyle(isSel)

	s.Fill(tui.Rect{X: x, Y: 0, W: len(label), H: 1}, " ", st)

	// Leading space
	s.SetCell(x, 0, " ", st)

	runes := []rune(cat.Name)
	for i, r := range runes {
		cellSt := st
		if i == cat.HotkeyIdx {
			cellSt = accSt
		}
		s.SetCell(x+1+i, 0, string(r), cellSt)
	}

	// Trailing space
	s.SetCell(x+len(label)-1, 0, " ", st)

	return len(label)
}

// renderVertical paints an autodb explorer-style vertical sidemenu.
func (mb *MenuBar) renderVertical(s tui.Surface, sz tui.Size) {
	// Fill background
	s.Fill(tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H}, " ", mb.menu.style.Bar())

	// Render frame header
	renderBoxFrame(s, tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H}, "Menu", mb.menu.style.Border())

	for i, cat := range mb.menu.categories {
		y := 2 + i
		if y >= sz.H-1 {
			break
		}
		isSel := mb.menu.active && mb.menu.selectedCategory == i
		st := mb.menu.style.ItemStyle(isSel)
		accSt := mb.menu.style.AccentStyle(isSel)

		s.Fill(tui.Rect{X: 1, Y: y, W: sz.W - 2, H: 1}, " ", st)

		runes := []rune(cat.Name)
		for j, r := range runes {
			cellSt := st
			if j == cat.HotkeyIdx {
				cellSt = accSt
			}
			s.SetCell(2+j, y, string(r), cellSt)
		}
	}
}

// HandleEvent manages keyboard navigation across the menu bar, dropdowns, and submenus.
func (mb *MenuBar) HandleEvent(ev tui.Event) bool {
	ke, ok := ev.(tui.KeyEvent)
	if !ok || ke.Kind == tui.KeyRelease {
		return false
	}

	ctx := mb.ctx

	// Route Alt accelerators and F10 toggle through the shared key resolver.
	// Unrecognized Alt chords bubble to host/editor bindings.
	if mb.menu.resolver != nil {
		if action, ok := mb.menu.resolver.Resolve(ScopeEditorNormal, ke); ok {
			switch action {
			case ActionToggleMenuBar:
				mb.menu.Toggle(ctx)
				return true
			case ActionOpenMenuFile:
				mb.menu.OpenCategory(0, ctx)
				return true
			case ActionOpenMenuOption:
				mb.menu.OpenCategory(1, ctx)
				return true
			case ActionOpenMenuHelp:
				mb.menu.OpenCategory(2, ctx)
				return true
			}
		}
	}

	// When a modal is open, delegate event to modal handler
	if mb.menu.modal != modalNone || mb.menu.activeModal != nil {
		return mb.handleModalKey(ke, ctx)
	}

	if !mb.menu.active {
		return false
	}

	// When a cascading submenu is open:
	if mb.menu.submenuOpen {
		return mb.handleSubmenuKey(ke, ctx)
	}

	// When a dropdown is open:
	if mb.menu.dropdownOpen {
		return mb.handleDropdownKey(ke, ctx)
	}

	// When menu bar is active (dropdown closed):
	return mb.handleBarKey(ke, ctx)
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

func (mb *MenuBar) handleBarKey(ke tui.KeyEvent, ctx *tui.Context) bool {
	// Hotkeys based on highlighted character ('f', 'o', 'h'):
	switch ke.Code {
	case 'f', 'F':
		mb.menu.OpenCategory(0, ctx)
		return true
	case 'o', 'O':
		mb.menu.OpenCategory(1, ctx)
		return true
	case 'h', 'H':
		mb.menu.OpenCategory(2, ctx)
		return true
	case tui.KeyEscape:
		mb.menu.Deactivate(ctx)
		return true
	}

	isVertical := mb.menu.placement == PlacementLeft || mb.menu.placement == PlacementRight

	if isVertical {
		switch ke.Code {
		case tui.KeyUp, 'k':
			mb.menu.selectedCategory = (mb.menu.selectedCategory - 1 + len(mb.menu.categories)) % len(mb.menu.categories)
			mb.markDirty()
			return true
		case tui.KeyDown, 'j':
			mb.menu.selectedCategory = (mb.menu.selectedCategory + 1) % len(mb.menu.categories)
			mb.markDirty()
			return true
		case tui.KeyRight, tui.KeyEnter, ' ', 'l':
			mb.menu.dropdownOpen = true
			mb.menu.selectedItem = 0
			mb.requestLayout()
			mb.markDirty()
			return true
		}
	} else {
		switch ke.Code {
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
	}

	return false
}

func (mb *MenuBar) handleDropdownKey(ke tui.KeyEvent, ctx *tui.Context) bool {
	cat := mb.menu.categories[mb.menu.selectedCategory]
	itemCount := len(cat.Items)

	// Check mnemonic hotkey for the active dropdown:
	codeLower := strings.ToLower(string(ke.Code))
	if len(codeLower) > 0 {
		r := []rune(codeLower)[0]
		for i, it := range cat.Items {
			if it.Hotkey == r {
				mb.menu.selectedItem = i
				if len(it.Submenu) > 0 {
					mb.menu.openSubmenu(ctx)
				} else {
					mb.menu.executeItem(mb.menu.selectedCategory, i, ctx)
				}
				return true
			}
		}
	}

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
	case tui.KeyRight, 'l':
		// If current item has a submenu, open the cascading submenu on the right
		if itemCount > 0 && len(cat.Items[mb.menu.selectedItem].Submenu) > 0 {
			mb.menu.openSubmenu(ctx)
			return true
		}
		// Otherwise in horizontal mode, move to next category dropdown
		if mb.menu.placement != PlacementLeft && mb.menu.placement != PlacementRight {
			mb.menu.selectedCategory = (mb.menu.selectedCategory + 1) % len(mb.menu.categories)
			mb.menu.selectedItem = 0
			mb.requestLayout()
			mb.markDirty()
			return true
		}
	case tui.KeyLeft, 'h':
		// In horizontal mode, move to previous category dropdown
		if mb.menu.placement != PlacementLeft && mb.menu.placement != PlacementRight {
			mb.menu.selectedCategory = (mb.menu.selectedCategory - 1 + len(mb.menu.categories)) % len(mb.menu.categories)
			mb.menu.selectedItem = 0
			mb.requestLayout()
			mb.markDirty()
			return true
		}
	case tui.KeyEnter, ' ':
		if itemCount > 0 {
			if len(cat.Items[mb.menu.selectedItem].Submenu) > 0 {
				mb.menu.openSubmenu(ctx)
			} else {
				mb.menu.executeItem(mb.menu.selectedCategory, mb.menu.selectedItem, ctx)
			}
		}
		return true
	}

	return false
}

func (mb *MenuBar) handleSubmenuKey(ke tui.KeyEvent, ctx *tui.Context) bool {
	cat := mb.menu.categories[mb.menu.selectedCategory]
	item := cat.Items[mb.menu.selectedItem]
	subCount := len(item.Submenu)

	switch ke.Code {
	case tui.KeyEscape, tui.KeyLeft, 'h':
		mb.menu.submenuOpen = false
		mb.requestLayout()
		mb.markDirty()
		return true
	case tui.KeyUp, 'k':
		if subCount > 0 {
			mb.menu.selectedSubItem = (mb.menu.selectedSubItem - 1 + subCount) % subCount
			mb.markDirty()
		}
		return true
	case tui.KeyDown, 'j':
		if subCount > 0 {
			mb.menu.selectedSubItem = (mb.menu.selectedSubItem + 1) % subCount
			mb.markDirty()
		}
		return true
	case '1', 'v', 'V':
		mb.menu.selectedSubItem = 0
		mb.commitSubmenu(ctx)
		return true
	case '2', 'n', 'N':
		mb.menu.selectedSubItem = 1
		mb.commitSubmenu(ctx)
		return true
	case tui.KeyEnter, ' ':
		mb.commitSubmenu(ctx)
		return true
	}
	return false
}

func (mb *MenuBar) commitSubmenu(ctx *tui.Context) {
	cat := mb.menu.categories[mb.menu.selectedCategory]
	item := cat.Items[mb.menu.selectedItem]
	if mb.menu.selectedSubItem >= 0 && mb.menu.selectedSubItem < len(item.Submenu) {
		subItem := item.Submenu[mb.menu.selectedSubItem]
		if subItem.Keyset != 0 {
			mb.menu.currentKeyset = subItem.Keyset
		}
		if subItem.Action() != nil {
			subItem.Trigger()
		} else {
			if mb.menu.cb.OnSetKeyset != nil && subItem.Keyset != 0 {
				mb.menu.cb.OnSetKeyset(subItem.Keyset)
			}
			if mb.menu.cb.OnStatusMessage != nil {
				if subItem.Keyset == widget.KeysetNano {
					mb.menu.cb.OnStatusMessage("switched keymap to Nano (modeless)")
				} else {
					mb.menu.cb.OnStatusMessage("switched keymap to Vim (modal)")
				}
			}
		}
	}
	mb.menu.Deactivate(ctx)
}

func (mb *MenuBar) handleModalKey(ke tui.KeyEvent, ctx *tui.Context) bool {
	if mb.menu.activeModal != nil {
		handled := mb.menu.activeModal.HandleEvent(ke)
		if mb.menu.activeModal != nil {
			if mb.menu.modal == modalExit {
				mb.menu.exitChoice = mb.menu.activeModal.SelectedButton()
			} else if mb.menu.modal == modalKeymaps {
				mb.menu.selectedKeymap = mb.menu.activeModal.SelectedButton()
			}
		}
		if handled {
			mb.markDirty()
			return true
		}
	}

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
			mb.commitKeymapModal(ctx)
			return true
		case '2':
			mb.menu.selectedKeymap = 1
			mb.commitKeymapModal(ctx)
			return true
		case tui.KeyEnter:
			mb.commitKeymapModal(ctx)
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

func (mb *MenuBar) commitKeymapModal(ctx *tui.Context) {
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

// MenuOverlay renders the dropdown menu cards, cascading submenus, and modal dialogs on OverlayHost.
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
	if !mo.menu.dropdownOpen && mo.menu.modal == modalNone && mo.menu.activeModal == nil {
		return tui.Size{}
	}
	return c.Constrain(tui.Size{W: c.MaxW, H: c.MaxH})
}

// Render paints dropdowns, cascading submenus, or active modals on the overlay surface.
func (mo *MenuOverlay) Render(s tui.Surface) {
	sz := s.Size()
	if sz.W <= 0 || sz.H <= 0 {
		return
	}

	if mo.menu.modal != modalNone || mo.menu.activeModal != nil {
		mo.renderModal(s, sz)
		return
	}

	if mo.menu.dropdownOpen {
		mo.renderDropdownAndSubmenu(s, sz)
	}
}

// renderDropdownAndSubmenu paints the active dropdown and any open cascading submenu.
func (mo *MenuOverlay) renderDropdownAndSubmenu(s tui.Surface, sz tui.Size) {
	catIdx := mo.menu.selectedCategory
	if catIdx < 0 || catIdx >= len(mo.menu.categories) {
		return
	}
	cat := mo.menu.categories[catIdx]

	boxW := 17
	boxH := len(cat.Items) + 2

	// Calculate (x, y) coordinates for dropdown box based on placement
	var x, y int
	switch mo.menu.placement {
	case PlacementLeft:
		x = 16
		y = catIdx*2 + 1
	case PlacementRight:
		x = sz.W - 16 - boxW
		y = catIdx*2 + 1
	case PlacementBottom:
		y = sz.H - 1 - boxH
		switch catIdx {
		case 0:
			x = 1
		case 1:
			x = 7
		case 2:
			x = sz.W - boxW - 1
		}
	default: // PlacementTop
		y = 1
		switch catIdx {
		case 0:
			x = 1
		case 1:
			x = 7
		case 2:
			x = sz.W - boxW - 1
		}
	}

	if x < 1 {
		x = 1
	}
	if y < 0 {
		y = 0
	}
	if y+boxH > sz.H {
		y = sz.H - boxH
	}

	// 1. Render framed dropdown box
	boxRect := tui.Rect{X: x, Y: y, W: boxW, H: boxH}
	renderBoxFrame(s, boxRect, cat.Name, mo.menu.style.Border())

	// 2. Render items inside dropdown
	for i, it := range cat.Items {
		itemY := y + 1 + i
		isSel := i == mo.menu.selectedItem
		it.SetSelected(isSel)
		it.SetStyle(mo.menu.style)
		it.RenderAt(s, x+1, itemY, boxW-2)
	}

	// 3. Render Cascading Submenu on the right if open
	if mo.menu.submenuOpen && mo.menu.selectedItem >= 0 && mo.menu.selectedItem < len(cat.Items) {
		item := cat.Items[mo.menu.selectedItem]
		if len(item.Submenu) > 0 {
			mo.renderCascadingSubmenu(s, sz, x, y, boxW, item)
		}
	}
}

// renderCascadingSubmenu renders the cascading submenu card next to the parent dropdown.
func (mo *MenuOverlay) renderCascadingSubmenu(s tui.Surface, sz tui.Size, parentX, parentY, parentW int, item *MenuItem) {
	subW := 24
	subH := len(item.Submenu) + 2

	// Cascade to the right of parent dropdown by default
	subX := parentX + parentW - 1
	subY := parentY + mo.menu.selectedItem + 1

	// If placed on the right or cascading extends past screen, cascade to the left
	if mo.menu.placement == PlacementRight || subX+subW > sz.W {
		subX = parentX - subW + 1
		if subX < 1 {
			subX = 1
		}
	}
	if subY+subH > sz.H {
		subY = sz.H - subH
	}
	if subY < 0 {
		subY = 0
	}

	subRect := tui.Rect{X: subX, Y: subY, W: subW, H: subH}
	renderBoxFrame(s, subRect, item.Name, mo.menu.style.Border())

	for i, subIt := range item.Submenu {
		rowY := subY + 1 + i
		isSel := i == mo.menu.selectedSubItem
		subIt.SetSelected(isSel)
		subIt.SetChecked(mo.menu.currentKeyset == subIt.Keyset)
		subIt.SetStyle(mo.menu.style)
		subIt.RenderAt(s, subX+1, rowY, subW-2)
	}
}

// renderModal paints centered modals with a dimmed scrim background.
func (mo *MenuOverlay) renderModal(s tui.Surface, sz tui.Size) {
	if mo.menu.activeModal != nil {
		mo.menu.activeModal.Render(s)
		return
	}

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
