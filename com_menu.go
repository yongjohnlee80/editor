package editor

import (
	"unicode"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
)

// MenuPlacement configures where the menu bar is docked within the application.
type MenuPlacement string

const (
	PlacementTop    MenuPlacement = "top"
	PlacementBottom MenuPlacement = "bottom"
	PlacementLeft   MenuPlacement = "left"
	PlacementRight  MenuPlacement = "right"
)

// MenuCategory represents a top-level category on the menu bar (e.g. File, Option, Help).
type MenuCategory struct {
	Name string
	// Hotkey is the mnemonic rune character for category activation.
	Hotkey rune
	// HotkeyIdx is the 0-based grapheme index within Name of the mnemonic character to highlight.
	HotkeyIdx int
	RightPeg  bool
	Items     []*MenuItem
}

// TopMenuCallbacks holds the external actions triggered by menu items and modals.
type TopMenuCallbacks struct {
	OnQuit          func()
	OnStatusMessage func(string)
	OnRestoreFocus  func()
}

// submenuLevel represents one level of an active cascading submenu in the navigation stack.
type submenuLevel struct {
	parent *MenuItem
	items  []*MenuItem
	sel    int
	rect   tui.Rect
}

// TopMenu is the Borland Turbo Vision / classic Macintosh-style menu bar subsystem.
// It supports configurable docking placement (Top, Bottom, Left, Right), Alt hotkeys,
// mnemonic navigation, and arbitrary-depth cascading submenus.
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

	// Cascading submenu navigation stack supporting arbitrary depth
	submenuStack []submenuLevel

	// Active modal dialog
	activeModal *Modal

	bar     *MenuBar
	overlay *MenuOverlay

	style *MenuStyle

	resolver KeyResolver
}

func sanitizeCategories(cats []MenuCategory) []MenuCategory {
	out := make([]MenuCategory, 0, len(cats))
	for _, c := range cats {
		cleanItems := make([]*MenuItem, 0, len(c.Items))
		for _, it := range c.Items {
			if it != nil {
				cleanItems = append(cleanItems, it)
			}
		}
		c.Items = cleanItems
		out = append(out, c)
	}
	return out
}

// NewTopMenu constructs a generic, application-neutral TopMenu subsystem with caller-supplied
// categories and optional external callbacks and key resolver.
func NewTopMenu(categories []MenuCategory, cb TopMenuCallbacks, resolver ...KeyResolver) *TopMenu {
	var res KeyResolver
	if len(resolver) > 0 && resolver[0] != nil {
		res = resolver[0]
	} else {
		res = NewDefaultKeyResolver(" ")
	}
	tm := &TopMenu{
		cb:         cb,
		resolver:   res,
		placement:  PlacementTop,
		categories: sanitizeCategories(categories),
	}
	tm.bar = &MenuBar{menu: tm}
	tm.overlay = &MenuOverlay{menu: tm}
	tm.selectedItem = tm.firstEnabledItem(tm.selectedCategory)
	tm.syncItemSelection()
	return tm
}

// Categories returns a defensive copy of the configured menu categories.
func (tm *TopMenu) Categories() []MenuCategory {
	out := make([]MenuCategory, len(tm.categories))
	for i, c := range tm.categories {
		itemsCopy := make([]*MenuItem, len(c.Items))
		copy(itemsCopy, c.Items)
		c.Items = itemsCopy
		out[i] = c
	}
	return out
}

// SetCategories updates the configured menu categories, filtering nils and invalidating layout.
func (tm *TopMenu) SetCategories(categories []MenuCategory) {
	tm.categories = sanitizeCategories(categories)
	if tm.selectedCategory >= len(tm.categories) {
		tm.selectedCategory = max(0, len(tm.categories)-1)
	}
	tm.selectedItem = tm.firstEnabledItem(tm.selectedCategory)
	tm.syncItemSelection()
	if tm.bar != nil && tm.bar.ctx != nil {
		tm.bar.ctx.RequestLayout()
		tm.bar.ctx.MarkDirty()
	}
	if tm.overlay != nil && tm.overlay.ctx != nil {
		tm.overlay.ctx.RequestLayout()
		tm.overlay.ctx.MarkDirty()
	}
}

// DefaultMenuCategories constructs the default File, Option, Help categories for the editor.
func DefaultMenuCategories(tm *TopMenu, cb TopMenuCallbacks) []MenuCategory {
	var vimItem, nanoItem *MenuItem
	vimItem = NewCheckableMenuItem("1. Vim  (modal)", '1', 0, true, func() {
		vimItem.SetChecked(true)
		nanoItem.SetChecked(false)
		if cb.OnStatusMessage != nil {
			cb.OnStatusMessage("switched keymap to Vim (modal)")
		}
	})
	nanoItem = NewCheckableMenuItem("2. Nano (modeless)", '2', 0, false, func() {
		nanoItem.SetChecked(true)
		vimItem.SetChecked(false)
		if cb.OnStatusMessage != nil {
			cb.OnStatusMessage("switched keymap to Nano (modeless)")
		}
	})

	return []MenuCategory{
		{
			Name:      "File",
			Hotkey:    'f',
			HotkeyIdx: 0,
			Items: []*MenuItem{
				NewMenuItem("New", 'n', 0, func() {
					tm.OpenNotImplemented("File -> New", nil)
				}),
				NewMenuItem("Open", 'o', 0, func() {
					tm.OpenNotImplemented("File -> Open", nil)
				}),
				NewMenuItem("Save", 's', 0, func() {
					tm.OpenNotImplemented("File -> Save", nil)
				}),
				NewMenuItem("Exit", 'x', 1, func() {
					tm.OpenExitModal(nil)
				}),
			},
		},
		{
			Name:      "Option",
			Hotkey:    'o',
			HotkeyIdx: 0,
			Items: []*MenuItem{
				NewMenuItemWithSubmenu("Keymaps", 'k', 0, vimItem, nanoItem),
			},
		},
		{
			Name:      "Help",
			Hotkey:    'h',
			HotkeyIdx: 0,
			RightPeg:  true,
			Items: []*MenuItem{
				NewMenuItem("About", 'a', 0, func() {
					tm.OpenNotImplemented("Help -> About", nil)
				}),
			},
		},
	}
}

// newTopMenu constructs the TopMenu subsystem with default editor categories.
func newTopMenu(cb TopMenuCallbacks, resolver ...KeyResolver) *TopMenu {
	tm := NewTopMenu(nil, cb, resolver...)
	tm.SetCategories(DefaultMenuCategories(tm, cb))
	return tm
}

// SetPlacement configures where the menu bar is mounted (top, bottom, left, right).
func (tm *TopMenu) SetPlacement(p MenuPlacement) {
	tm.placement = p
	if tm.bar != nil && tm.bar.ctx != nil {
		tm.bar.ctx.RequestLayout()
		tm.bar.ctx.MarkDirty()
	}
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

// Bar returns the MenuBar component for docking into layout.
func (tm *TopMenu) Bar() *MenuBar {
	return tm.bar
}

// Overlay returns the MenuOverlay component for mounting on OverlayHost.
func (tm *TopMenu) Overlay() *MenuOverlay {
	return tm.overlay
}

// Active reports whether the menu subsystem currently has focus.
func (tm *TopMenu) Active() bool {
	return tm.active
}

// DropdownOpen reports whether a dropdown menu is currently visible.
func (tm *TopMenu) DropdownOpen() bool {
	return tm.dropdownOpen
}

// SubmenuOpen reports whether one or more cascading submenus are open.
func (tm *TopMenu) SubmenuOpen() bool {
	return len(tm.submenuStack) > 0
}

// ModalActive reports whether a modal dialog is currently active.
func (tm *TopMenu) ModalActive() bool {
	return tm.activeModal != nil
}

// ActiveModal returns the currently active modal dialog, if any.
func (tm *TopMenu) ActiveModal() *Modal {
	return tm.activeModal
}

// Activate opens the menu bar on the first category.
func (tm *TopMenu) Activate(ctx *tui.Context) {
	tm.active = true
	tm.selectedCategory = 0
	tm.dropdownOpen = false
	tm.selectedItem = tm.firstEnabledItem(0)
	tm.submenuStack = nil
	tm.activeModal = nil
	tm.syncItemSelection()
	if ctx != nil {
		ctx.FocusComponent(tm.bar)
		ctx.RequestLayout()
		ctx.MarkDirty()
	} else if tm.bar != nil && tm.bar.ctx != nil {
		tm.bar.ctx.RequestFocus()
		tm.bar.ctx.RequestLayout()
		tm.bar.ctx.MarkDirty()
	}
}

// OpenCategory activates the menu and opens the specified category dropdown.
func (tm *TopMenu) OpenCategory(catIdx int, ctx *tui.Context) {
	if catIdx < 0 || catIdx >= len(tm.categories) {
		return
	}
	tm.active = true
	tm.selectedCategory = catIdx
	tm.dropdownOpen = true
	tm.selectedItem = tm.firstEnabledItem(catIdx)
	tm.submenuStack = nil
	tm.activeModal = nil
	tm.syncItemSelection()
	if ctx != nil {
		ctx.FocusComponent(tm.bar)
		ctx.RequestLayout()
		ctx.MarkDirty()
	} else if tm.bar != nil && tm.bar.ctx != nil {
		tm.bar.ctx.RequestFocus()
		tm.bar.ctx.RequestLayout()
		tm.bar.ctx.MarkDirty()
	}
}

// Deactivate closes the menu bar and restores focus to the editor.
func (tm *TopMenu) Deactivate(ctx *tui.Context) {
	if tm.activeModal != nil {
		tm.CloseModal(ctx)
	}

	tm.active = false
	tm.dropdownOpen = false
	tm.submenuStack = nil
	tm.syncItemSelection()

	if ctx != nil {
		ctx.RequestLayout()
		ctx.MarkDirty()
	} else if tm.bar != nil && tm.bar.ctx != nil {
		tm.bar.ctx.RequestLayout()
		tm.bar.ctx.MarkDirty()
	}

	if tm.cb.OnRestoreFocus != nil {
		tm.cb.OnRestoreFocus()
	}
}

// Toggle flips the active state of the menu bar.
func (tm *TopMenu) Toggle(ctx *tui.Context) {
	if tm.active {
		tm.Deactivate(ctx)
	} else {
		tm.Activate(ctx)
	}
}

func (tm *TopMenu) firstEnabledItem(catIdx int) int {
	if catIdx < 0 || catIdx >= len(tm.categories) {
		return -1
	}
	for i, it := range tm.categories[catIdx].Items {
		if it != nil && !it.Disabled() {
			return i
		}
	}
	return -1
}

func (tm *TopMenu) syncItemSelection() {
	for cIdx, cat := range tm.categories {
		for iIdx, it := range cat.Items {
			if it == nil {
				continue
			}
			isSel := tm.dropdownOpen && cIdx == tm.selectedCategory && iIdx == tm.selectedItem && len(tm.submenuStack) == 0
			it.SetSelected(isSel)
		}
	}
	for lvlIdx, lvl := range tm.submenuStack {
		isTop := (lvlIdx == len(tm.submenuStack)-1)
		for j, subIt := range lvl.items {
			if subIt == nil {
				continue
			}
			isSel := isTop && j == lvl.sel
			subIt.SetSelected(isSel)
		}
	}
}

func (tm *TopMenu) selectNextItem() {
	if tm.selectedCategory < 0 || tm.selectedCategory >= len(tm.categories) {
		return
	}
	cat := tm.categories[tm.selectedCategory]
	n := len(cat.Items)
	if n <= 1 {
		return
	}
	start := tm.selectedItem
	if start < 0 {
		start = 0
	}
	for step := 1; step < n; step++ {
		idx := (start + step) % n
		if cat.Items[idx] != nil && !cat.Items[idx].Disabled() {
			tm.selectedItem = idx
			tm.syncItemSelection()
			return
		}
	}
}

func (tm *TopMenu) selectPrevItem() {
	if tm.selectedCategory < 0 || tm.selectedCategory >= len(tm.categories) {
		return
	}
	cat := tm.categories[tm.selectedCategory]
	n := len(cat.Items)
	if n <= 1 {
		return
	}
	start := tm.selectedItem
	if start < 0 {
		start = 0
	}
	for step := 1; step < n; step++ {
		idx := (start - step + n) % n
		if cat.Items[idx] != nil && !cat.Items[idx].Disabled() {
			tm.selectedItem = idx
			tm.syncItemSelection()
			return
		}
	}
}

func (tm *TopMenu) selectNextSubmenuItem(lvl *submenuLevel) {
	n := len(lvl.items)
	if n <= 1 {
		return
	}
	start := lvl.sel
	if start < 0 {
		start = 0
	}
	for step := 1; step < n; step++ {
		idx := (start + step) % n
		if lvl.items[idx] != nil && !lvl.items[idx].Disabled() {
			lvl.sel = idx
			tm.syncItemSelection()
			return
		}
	}
}

func (tm *TopMenu) selectPrevSubmenuItem(lvl *submenuLevel) {
	n := len(lvl.items)
	if n <= 1 {
		return
	}
	start := lvl.sel
	if start < 0 {
		start = 0
	}
	for step := 1; step < n; step++ {
		idx := (start - step + n) % n
		if lvl.items[idx] != nil && !lvl.items[idx].Disabled() {
			lvl.sel = idx
			tm.syncItemSelection()
			return
		}
	}
}

func (tm *TopMenu) openSubmenu(ctx *tui.Context) bool {
	var currentItem *MenuItem
	if len(tm.submenuStack) == 0 {
		if tm.selectedCategory < 0 || tm.selectedCategory >= len(tm.categories) {
			return false
		}
		cat := tm.categories[tm.selectedCategory]
		if tm.selectedItem < 0 || tm.selectedItem >= len(cat.Items) {
			return false
		}
		currentItem = cat.Items[tm.selectedItem]
	} else {
		top := tm.submenuStack[len(tm.submenuStack)-1]
		if top.sel < 0 || top.sel >= len(top.items) {
			return false
		}
		currentItem = top.items[top.sel]
	}

	if currentItem == nil || currentItem.Disabled() || !currentItem.HasSubmenu() {
		return false
	}

	subItems := currentItem.Submenu()
	firstEnabled := -1
	for i, subIt := range subItems {
		if subIt != nil && !subIt.Disabled() {
			firstEnabled = i
			break
		}
	}

	tm.submenuStack = append(tm.submenuStack, submenuLevel{
		parent: currentItem,
		items:  subItems,
		sel:    firstEnabled,
	})

	tm.syncItemSelection()
	if ctx != nil {
		ctx.RequestLayout()
		ctx.MarkDirty()
	} else if tm.bar != nil && tm.bar.ctx != nil {
		tm.bar.ctx.RequestLayout()
		tm.bar.ctx.MarkDirty()
	}
	return true
}

func (tm *TopMenu) executeItem(catIdx, itemIdx int, ctx *tui.Context) {
	if catIdx < 0 || catIdx >= len(tm.categories) {
		return
	}
	cat := tm.categories[catIdx]
	if itemIdx < 0 || itemIdx >= len(cat.Items) {
		return
	}
	item := cat.Items[itemIdx]
	if item == nil || item.Disabled() {
		return // Inert on disabled items; keeps dropdown open
	}

	if item.HasSubmenu() {
		tm.openSubmenu(ctx)
		return
	}

	tm.dropdownOpen = false
	tm.submenuStack = nil
	tm.syncItemSelection()
	item.Trigger()
}

// OpenModal displays a modal dialog on the overlay layer.
func (tm *TopMenu) OpenModal(m *Modal, ctx *tui.Context) {
	if m == nil {
		return
	}
	if tm.activeModal != nil {
		tm.CloseModal(ctx)
	}
	tm.activeModal = m
	if tm.overlay != nil && tm.overlay.ctx != nil {
		tm.overlay.ctx.Mount(m)
		tm.overlay.ctx.RequestLayout()
		tm.overlay.ctx.MarkDirty()
	} else if ctx != nil {
		ctx.Mount(m)
		ctx.RequestLayout()
		ctx.MarkDirty()
	}
}

// CloseModal unmounts and closes the currently active modal dialog.
func (tm *TopMenu) CloseModal(ctx *tui.Context) {
	if tm.activeModal == nil {
		return
	}
	if ctx != nil {
		ctx.Unmount(tm.activeModal)
	} else if tm.overlay != nil && tm.overlay.ctx != nil {
		tm.overlay.ctx.Unmount(tm.activeModal)
	}
	tm.activeModal = nil
	if ctx != nil {
		ctx.RequestLayout()
		ctx.MarkDirty()
	} else if tm.overlay != nil && tm.overlay.ctx != nil {
		tm.overlay.ctx.RequestLayout()
		tm.overlay.ctx.MarkDirty()
	}
}

// OpenExitModal displays the exit confirmation modal dialog.
func (tm *TopMenu) OpenExitModal(ctx *tui.Context) {
	btnYes := NewButton("Yes", func() {
		tm.CloseModal(ctx)
		tm.Deactivate(ctx)
		if tm.cb.OnQuit != nil {
			tm.cb.OnQuit()
		}
	}).SetRole(ButtonRoleDefault).SetMnemonic('y')

	btnNo := NewButton("No", func() {
		tm.CloseModal(ctx)
		tm.Deactivate(ctx)
	}).SetRole(ButtonRoleCancel).SetMnemonic('n')

	modal := NewModal("Exit Confirmation", "Are you sure to quit?", btnYes, btnNo)
	modal.SetStyle(defaultModalStyle)
	modal.OnDismiss(func() {
		tm.CloseModal(ctx)
		tm.Deactivate(ctx)
	})

	tm.OpenModal(modal, ctx)
}

// OpenNotImplemented displays a not-implemented notification modal dialog.
func (tm *TopMenu) OpenNotImplemented(msg string, ctx *tui.Context) {
	btnOK := NewButton("OK", func() {
		tm.CloseModal(ctx)
		tm.Deactivate(ctx)
	}).SetRole(ButtonRoleDefault).SetMnemonic('o')

	modal := NewModal("Not Implemented", msg+" is not implemented yet.", btnOK)
	modal.SetStyle(defaultModalStyle)
	modal.OnDismiss(func() {
		tm.CloseModal(ctx)
		tm.Deactivate(ctx)
	})

	tm.OpenModal(modal, ctx)
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
func (mb *MenuBar) Layout(c tui.Constraints) tui.Size {
	if mb.menu.placement == PlacementLeft || mb.menu.placement == PlacementRight {
		return c.Constrain(tui.Size{W: 16, H: c.MaxH})
	}
	return c.Constrain(tui.Size{W: c.MaxW, H: 1})
}

// AcceptsFocus implements tui.Focusable.
func (mb *MenuBar) AcceptsFocus() bool {
	return true
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
	// 1. Fill entire bar background with bar style.
	s.Fill(tui.Rect{X: 0, Y: 0, W: sz.W, H: 1}, " ", mb.menu.MenuStyle().Bar())

	// Separate into normal (left) categories and right-pegged categories.
	var leftCats []int
	var rightCats []int
	for i, cat := range mb.menu.categories {
		if cat.RightPeg {
			rightCats = append(rightCats, i)
		} else {
			leftCats = append(leftCats, i)
		}
	}

	// 2. Render Left categories
	x := 1
	for _, idx := range leftCats {
		cat := mb.menu.categories[idx]
		isSel := mb.menu.active && mb.menu.selectedCategory == idx
		w := mb.renderCategoryItemHorizontal(s, x, cat, isSel)
		x += w + 1
	}

	// 3. Render Right-pegged categories
	rightX := sz.W - 1
	for rIdx := len(rightCats) - 1; rIdx >= 0; rIdx-- {
		idx := rightCats[rIdx]
		cat := mb.menu.categories[idx]
		catW := s.StringWidth(cat.Name) + 2
		posX := rightX - catW
		if posX < x {
			posX = x
		}
		isSel := mb.menu.active && mb.menu.selectedCategory == idx
		mb.renderCategoryItemHorizontal(s, posX, cat, isSel)
		rightX = posX - 1
	}
}

func (mb *MenuBar) renderCategoryItemHorizontal(s tui.Surface, x int, cat MenuCategory, isSel bool) int {
	label := " " + cat.Name + " "
	st := mb.menu.MenuStyle().ItemStyle(isSel)
	accSt := mb.menu.MenuStyle().AccentStyle(isSel)

	col := x
	runeIdx := 0
	for g := range tui.Graphemes(label) {
		gw := s.StringWidth(g)
		cellSt := st
		if runeIdx == cat.HotkeyIdx+1 {
			cellSt = accSt
		}
		s.SetCell(col, 0, g, cellSt)
		col += gw
		runeIdx++
	}
	return s.StringWidth(label)
}

func (mb *MenuBar) renderVertical(s tui.Surface, sz tui.Size) {
	st := mb.menu.MenuStyle().Bar()
	s.Fill(tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H}, " ", st)
	renderBoxFrame(s, tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H}, "Menu", mb.menu.MenuStyle().Border())

	y := 2
	for i, cat := range mb.menu.categories {
		if y >= sz.H-2 {
			break
		}
		isSel := mb.menu.active && mb.menu.selectedCategory == i
		itemSt := mb.menu.MenuStyle().ItemStyle(isSel)
		accSt := mb.menu.MenuStyle().AccentStyle(isSel)

		if isSel {
			cursorRowStyle := style.New().
				Background(style.ANSI(6)).
				Foreground(style.ANSI(0)).
				Bold(true)
			cursorAccentStyle := style.New().
				Background(style.ANSI(6)).
				Foreground(style.ANSI(1)).
				Bold(true)
			itemSt = cursorRowStyle
			accSt = cursorAccentStyle
		}

		s.Fill(tui.Rect{X: 1, Y: y, W: sz.W - 2, H: 1}, " ", itemSt)

		display := "  " + cat.Name
		col := 1
		runeIdx := 0
		for g := range tui.Graphemes(display) {
			gw := s.StringWidth(g)
			cellSt := itemSt
			if runeIdx == cat.HotkeyIdx+2 {
				cellSt = accSt
			}
			s.SetCell(col, y, g, cellSt)
			col += gw
			runeIdx++
		}
		y += 2
	}
}

// HandleEvent handles global menu shortcuts and menu navigation.
func (mb *MenuBar) HandleEvent(ev tui.Event) bool {
	ke, ok := ev.(tui.KeyEvent)
	if !ok || ke.Kind != tui.KeyPress {
		return false
	}

	// 1. If modal is active, delegate entirely to modal
	if mb.menu.activeModal != nil {
		return mb.handleModalKey(ke, mb.ctx)
	}

	// 2. F10 toggles menu bar
	if ke.Code == tui.KeyF10 {
		mb.menu.Toggle(mb.ctx)
		return true
	}

	// 3. Alt shortcuts: Alt+f (File), Alt+o (Option), Alt+h (Help)
	if ke.Mods&tui.ModAlt != 0 {
		for i, cat := range mb.menu.categories {
			if unicode.ToLower(cat.Hotkey) == unicode.ToLower(rune(ke.Code)) {
				mb.menu.OpenCategory(i, mb.ctx)
				return true
			}
		}
		return false
	}

	// 4. If menu is not active, do not consume keys
	if !mb.menu.active {
		return false
	}

	// 5. If cascading submenu stack is active, route there
	if len(mb.menu.submenuStack) > 0 {
		return mb.handleSubmenuKey(ke, mb.ctx)
	}

	// 6. If dropdown is open, route to dropdown
	if mb.menu.dropdownOpen {
		return mb.handleDropdownKey(ke, mb.ctx)
	}

	// 7. Menu bar navigation
	return mb.handleBarKey(ke, mb.ctx)
}

func (mb *MenuBar) handleBarKey(ke tui.KeyEvent, ctx *tui.Context) bool {
	// 1. Mnemonic hotkeys for categories take top precedence on bar
	for i, cat := range mb.menu.categories {
		if cat.Hotkey != 0 && unicode.ToLower(cat.Hotkey) == unicode.ToLower(rune(ke.Code)) {
			mb.menu.OpenCategory(i, ctx)
			return true
		}
	}

	catCount := len(mb.menu.categories)
	switch ke.Code {
	case tui.KeyEscape:
		mb.menu.Deactivate(ctx)
		return true

	case tui.KeyLeft, 'h':
		if catCount > 0 {
			mb.menu.selectedCategory = (mb.menu.selectedCategory - 1 + catCount) % catCount
			mb.markDirty()
		}
		return true

	case tui.KeyRight, 'l':
		if catCount > 0 {
			mb.menu.selectedCategory = (mb.menu.selectedCategory + 1) % catCount
			mb.markDirty()
		}
		return true

	case tui.KeyDown, tui.KeyEnter, 'j':
		if catCount > 0 {
			mb.menu.dropdownOpen = true
			mb.menu.selectedItem = mb.menu.firstEnabledItem(mb.menu.selectedCategory)
			mb.menu.syncItemSelection()
			mb.requestLayout()
			mb.markDirty()
			return true
		}
		return false
	}
	return false
}

func (mb *MenuBar) handleDropdownKey(ke tui.KeyEvent, ctx *tui.Context) bool {
	catCount := len(mb.menu.categories)
	if catCount == 0 || mb.menu.selectedCategory < 0 || mb.menu.selectedCategory >= catCount {
		mb.menu.dropdownOpen = false
		mb.requestLayout()
		mb.markDirty()
		return true
	}

	cat := mb.menu.categories[mb.menu.selectedCategory]
	itemCount := len(cat.Items)

	// 1. Check for mnemonic hotkey within dropdown items first
	for i, it := range cat.Items {
		if it == nil || it.Disabled() {
			continue
		}
		if it.Hotkey != 0 && unicode.ToLower(it.Hotkey) == unicode.ToLower(rune(ke.Code)) {
			mb.menu.selectedItem = i
			mb.menu.syncItemSelection()
			mb.menu.executeItem(mb.menu.selectedCategory, i, ctx)
			return true
		}
	}

	switch ke.Code {
	case tui.KeyEscape:
		mb.menu.dropdownOpen = false
		mb.menu.syncItemSelection()
		mb.requestLayout()
		mb.markDirty()
		return true

	case tui.KeyUp, 'k':
		if itemCount > 0 {
			mb.menu.selectPrevItem()
			mb.markDirty()
		}
		return true

	case tui.KeyDown, 'j':
		if itemCount > 0 {
			mb.menu.selectNextItem()
			mb.markDirty()
		}
		return true

	case tui.KeyLeft, 'h':
		if catCount > 0 {
			mb.menu.selectedCategory = (mb.menu.selectedCategory - 1 + catCount) % catCount
			mb.menu.selectedItem = mb.menu.firstEnabledItem(mb.menu.selectedCategory)
			mb.menu.syncItemSelection()
			mb.requestLayout()
			mb.markDirty()
		}
		return true

	case tui.KeyRight, 'l':
		if mb.menu.selectedItem >= 0 && mb.menu.selectedItem < len(cat.Items) {
			currItem := cat.Items[mb.menu.selectedItem]
			if currItem != nil && currItem.HasSubmenu() && !currItem.Disabled() {
				mb.menu.openSubmenu(ctx)
				return true
			}
		}
		if catCount > 0 {
			mb.menu.selectedCategory = (mb.menu.selectedCategory + 1) % catCount
			mb.menu.selectedItem = mb.menu.firstEnabledItem(mb.menu.selectedCategory)
			mb.menu.syncItemSelection()
			mb.requestLayout()
			mb.markDirty()
		}
		return true

	case tui.KeyEnter:
		if mb.menu.selectedItem >= 0 && mb.menu.selectedItem < len(cat.Items) {
			mb.menu.executeItem(mb.menu.selectedCategory, mb.menu.selectedItem, ctx)
		}
		return true
	}
	return false
}

func (mb *MenuBar) handleSubmenuKey(ke tui.KeyEvent, ctx *tui.Context) bool {
	if len(mb.menu.submenuStack) == 0 {
		return false
	}
	topIdx := len(mb.menu.submenuStack) - 1
	top := &mb.menu.submenuStack[topIdx]
	subCount := len(top.items)

	// 1. Check for hotkey mnemonic in current submenu level first
	for i, it := range top.items {
		if it == nil || it.Disabled() {
			continue
		}
		if it.Hotkey != 0 && unicode.ToLower(it.Hotkey) == unicode.ToLower(rune(ke.Code)) {
			top.sel = i
			mb.menu.syncItemSelection()
			if it.HasSubmenu() {
				mb.menu.openSubmenu(ctx)
			} else {
				it.Trigger()
				mb.menu.Deactivate(ctx)
			}
			return true
		}
	}

	switch ke.Code {
	case tui.KeyEscape, tui.KeyLeft, 'h':
		mb.menu.submenuStack = mb.menu.submenuStack[:topIdx]
		mb.menu.syncItemSelection()
		mb.requestLayout()
		mb.markDirty()
		return true

	case tui.KeyUp, 'k':
		if subCount > 0 {
			mb.menu.selectPrevSubmenuItem(top)
			mb.markDirty()
		}
		return true

	case tui.KeyDown, 'j':
		if subCount > 0 {
			mb.menu.selectNextSubmenuItem(top)
			mb.markDirty()
		}
		return true

	case tui.KeyRight, 'l':
		if top.sel >= 0 && top.sel < subCount {
			item := top.items[top.sel]
			if item != nil && item.HasSubmenu() && !item.Disabled() {
				mb.menu.openSubmenu(ctx)
				return true
			}
		}
		return true

	case tui.KeyEnter, ' ':
		if top.sel >= 0 && top.sel < subCount {
			item := top.items[top.sel]
			if item != nil && !item.Disabled() {
				if item.HasSubmenu() {
					mb.menu.openSubmenu(ctx)
					return true
				}
				item.Trigger()
				mb.menu.Deactivate(ctx)
				return true
			}
		}
		return true // Inert on disabled item; keep open
	}
	return false
}

func (mb *MenuBar) handleModalKey(ke tui.KeyEvent, ctx *tui.Context) bool {
	if mb.menu.activeModal != nil {
		if mb.menu.activeModal.HandleEvent(ke) {
			mb.markDirty()
			return true
		}
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

// MenuOverlay renders the dropdown menu cards, cascading submenus, and modal dialogs on OverlayHost.
type MenuOverlay struct {
	ctx  *tui.Context
	menu *TopMenu

	dropdownRect tui.Rect
}

// Init mounts the overlay into the TUI context.
func (mo *MenuOverlay) Init(ctx *tui.Context) {
	mo.ctx = ctx
}

func (mo *MenuOverlay) measure(s string) int {
	if mo.ctx != nil {
		return mo.ctx.StringWidth(s)
	}
	return tui.StringWidth(s)
}

// Layout sizes the overlay layer to match container bounds when active and computes popup geometry.
func (mo *MenuOverlay) Layout(c tui.Constraints) tui.Size {
	if !mo.menu.dropdownOpen && len(mo.menu.submenuStack) == 0 && mo.menu.activeModal == nil {
		return tui.Size{}
	}
	sz := c.Constrain(tui.Size{W: c.MaxW, H: c.MaxH})
	if mo.menu.activeModal != nil && mo.ctx != nil {
		msz := mo.ctx.LayoutChild(mo.menu.activeModal, c)
		mo.ctx.PlaceChild(mo.menu.activeModal, tui.Rect{X: 0, Y: 0, W: msz.W, H: msz.H})
	}
	if mo.menu.dropdownOpen {
		mo.computeGeometry(sz)
	}
	return sz
}

func (mo *MenuOverlay) computeGeometry(sz tui.Size) {
	catIdx := mo.menu.selectedCategory
	if catIdx < 0 || catIdx >= len(mo.menu.categories) {
		mo.dropdownRect = tui.Rect{}
		return
	}
	cat := mo.menu.categories[catIdx]

	boxW := 18
	for _, it := range cat.Items {
		if it == nil {
			continue
		}
		w := mo.measure(it.Name) + 6
		if it.HasSubmenu() {
			w += 2
		}
		if w > boxW {
			boxW = w
		}
	}
	boxH := len(cat.Items) + 2

	var x, y int
	switch mo.menu.placement {
	case PlacementBottom:
		y = sz.H - boxH - 1
		x = mo.calculateDropdownX(catIdx, boxW, sz.W)
	case PlacementLeft:
		x = 16
		y = 2 + catIdx*2
	case PlacementRight:
		x = sz.W - 16 - boxW
		y = 2 + catIdx*2
	default:
		y = 1
		x = mo.calculateDropdownX(catIdx, boxW, sz.W)
	}

	if x < 0 {
		x = 0
	}
	if x+boxW > sz.W {
		x = max(0, sz.W-boxW)
	}
	if y < 0 {
		y = 0
	}
	if y+boxH > sz.H {
		boxH = max(2, sz.H-y)
	}

	mo.dropdownRect = tui.Rect{X: x, Y: y, W: boxW, H: boxH}

	// Compute cascading submenus for each stack level
	prevRect := mo.dropdownRect
	for lvlIdx := range mo.menu.submenuStack {
		lvl := &mo.menu.submenuStack[lvlIdx]
		subW := 22
		for _, subIt := range lvl.items {
			if subIt == nil {
				continue
			}
			w := mo.measure(subIt.Name) + 6
			if subIt.HasSubmenu() {
				w += 2
			}
			if w > subW {
				subW = w
			}
		}
		subH := len(lvl.items) + 2

		subX := prevRect.X + prevRect.W
		if subX+subW > sz.W {
			subX = prevRect.X - subW
			if subX < 0 {
				subX = max(0, sz.W-subW)
			}
		}

		selY := prevRect.Y + 1
		if lvlIdx == 0 {
			selY = prevRect.Y + 1 + mo.menu.selectedItem
		} else {
			prevLvl := mo.menu.submenuStack[lvlIdx-1]
			selY = prevRect.Y + 1 + prevLvl.sel
		}
		subY := selY
		if subY+subH > sz.H {
			subY = max(0, sz.H-subH)
		}

		subRect := tui.Rect{X: subX, Y: subY, W: subW, H: subH}
		lvl.rect = subRect
		prevRect = subRect
	}
}

// Render paints dropdowns and cascading submenus on the overlay surface without state mutation.
func (mo *MenuOverlay) Render(s tui.Surface) {
	sz := s.Size()
	if sz.W <= 0 || sz.H <= 0 {
		return
	}

	if mo.menu.activeModal != nil {
		if mo.ctx == nil {
			mo.menu.activeModal.Render(s)
		}
		return
	}

	if mo.menu.dropdownOpen {
		mo.renderDropdownAndSubmenus(s, sz)
	}
}

func (mo *MenuOverlay) renderDropdownAndSubmenus(s tui.Surface, sz tui.Size) {
	catIdx := mo.menu.selectedCategory
	if catIdx < 0 || catIdx >= len(mo.menu.categories) {
		return
	}
	cat := mo.menu.categories[catIdx]

	boxRect := mo.dropdownRect
	if boxRect.W <= 0 || boxRect.H <= 0 {
		mo.computeGeometry(sz)
		boxRect = mo.dropdownRect
	}
	if boxRect.W <= 0 || boxRect.H <= 0 {
		return
	}

	renderBoxFrame(s, boxRect, cat.Name, mo.menu.MenuStyle().Border())

	// Render items purely using already calculated positions
	for i, item := range cat.Items {
		if item == nil {
			continue
		}
		rowY := boxRect.Y + 1 + i
		if rowY >= boxRect.Y+boxRect.H-1 {
			break
		}
		item.RenderAt(s, boxRect.X+1, rowY, boxRect.W-2)
	}

	// Render cascading submenus for each stack level
	for _, lvl := range mo.menu.submenuStack {
		subRect := lvl.rect
		if subRect.W <= 0 || subRect.H <= 0 {
			continue
		}

		title := ""
		if lvl.parent != nil {
			title = lvl.parent.Name
		}
		renderBoxFrame(s, subRect, title, mo.menu.MenuStyle().Border())

		for j, subIt := range lvl.items {
			if subIt == nil {
				continue
			}
			rowY := subRect.Y + 1 + j
			if rowY >= subRect.Y+subRect.H-1 {
				break
			}
			subIt.RenderAt(s, subRect.X+1, rowY, subRect.W-2)
		}
	}
}

func (mo *MenuOverlay) calculateDropdownX(catIdx, boxW, screenW int) int {
	if catIdx < 0 || catIdx >= len(mo.menu.categories) {
		return 0
	}
	cat := mo.menu.categories[catIdx]
	if cat.RightPeg {
		return max(0, screenW-boxW-1)
	}

	x := 1
	for i := 0; i < catIdx; i++ {
		c := mo.menu.categories[i]
		if !c.RightPeg {
			x += mo.measure(c.Name) + 3
		}
	}
	return x
}

// renderBoxFrame draws a titled box frame on the given rectangle.
func renderBoxFrame(s tui.Surface, r tui.Rect, title string, frameSt style.Style, titleSt ...style.Style) {
	if r.W < 2 || r.H < 2 {
		return
	}

	tSt := frameSt.Bold(true)
	if len(titleSt) > 0 {
		tSt = titleSt[0]
	}

	// Corners
	s.SetCell(r.X, r.Y, "┌", frameSt)
	s.SetCell(r.X+r.W-1, r.Y, "┐", frameSt)
	s.SetCell(r.X, r.Y+r.H-1, "└", frameSt)
	s.SetCell(r.X+r.W-1, r.Y+r.H-1, "┘", frameSt)

	// Top and bottom borders
	for x := r.X + 1; x < r.X+r.W-1; x++ {
		s.SetCell(x, r.Y, "─", frameSt)
		s.SetCell(x, r.Y+r.H-1, "─", frameSt)
	}

	// Left and right borders
	for y := r.Y + 1; y < r.Y+r.H-1; y++ {
		s.SetCell(r.X, y, "│", frameSt)
		s.SetCell(r.X+r.W-1, y, "│", frameSt)
	}

	// Title drawn after borders to overwrite top border cells cleanly
	if title != "" {
		tw := s.StringWidth(title)
		if r.W > tw+4 {
			titleText := " " + title + " "
			drawText(s, r.X+2, r.Y, titleText, tSt)
		}
	}
}

// drawText renders a string horizontally starting at (x, y) using grapheme clusters.
func drawText(sur tui.Surface, x, y int, s string, st style.Style) int {
	w := sur.Size().W
	for c := range tui.Graphemes(s) {
		cw := sur.StringWidth(c)
		if x+cw > w {
			break
		}
		sur.SetCell(x, y, c, st)
		x += cw
	}
	return x
}

// HandleEvent forwards events to MenuBar when active.
func (mo *MenuOverlay) HandleEvent(ev tui.Event) bool {
	if !mo.menu.Active() {
		return false
	}
	return mo.menu.Bar().HandleEvent(ev)
}
