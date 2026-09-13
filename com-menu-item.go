package editor

import (
	"strings"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// SubMenuItem is an alias to MenuItem, representing an item in a menu or cascading submenu.
type SubMenuItem = MenuItem

// MenuItemStatus represents the interaction and visual state flags of a MenuItem.
type MenuItemStatus struct {
	Selected  bool
	Disabled  bool
	Checked   bool
	Checkable bool
}

// MenuItem is a standalone menu item widget component, similar to Button.
// It encapsulates the display name, hotkey accelerator, semantic action callback,
// state/status flags (selected, disabled, checked), and optional submenu children.
type MenuItem struct {
	widget.Base

	Name      string
	Hotkey    rune
	HotkeyIdx int
	Keyset    widget.Keyset

	action func()

	// Status flags
	selected  bool
	disabled  bool
	checked   bool
	checkable bool

	Submenu []*MenuItem

	style *MenuStyle
}

// NewMenuItem constructs a standalone MenuItem widget with the given label, hotkey accelerator,
// and semantic action callback.
func NewMenuItem(name string, hotkey rune, hotkeyIdx int, action func()) *MenuItem {
	return &MenuItem{
		Name:      name,
		Hotkey:    hotkey,
		HotkeyIdx: hotkeyIdx,
		action:    action,
	}
}

// NewMenuItemWithSubmenu constructs a MenuItem that cascades into a nested submenu.
func NewMenuItemWithSubmenu(name string, hotkey rune, hotkeyIdx int, submenu ...*MenuItem) *MenuItem {
	return &MenuItem{
		Name:      name,
		Hotkey:    hotkey,
		HotkeyIdx: hotkeyIdx,
		Submenu:   submenu,
	}
}

// NewMenuItemWithKeyset constructs a checkable MenuItem bound to an editor Keyset option.
func NewMenuItemWithKeyset(name string, hotkey rune, hotkeyIdx int, ks widget.Keyset, action func()) *MenuItem {
	return &MenuItem{
		Name:      name,
		Hotkey:    hotkey,
		HotkeyIdx: hotkeyIdx,
		Keyset:    ks,
		action:    action,
		checkable: true,
	}
}

// Init mounts the menu item into the TUI context and initializes child submenu items.
func (mi *MenuItem) Init(ctx *tui.Context) {
	mi.Base.Init(ctx)
	for _, sub := range mi.Submenu {
		sub.Init(ctx)
	}
}

// Action returns the item's callback action.
func (mi *MenuItem) Action() func() {
	return mi.action
}

// SetAction updates the item's callback action.
func (mi *MenuItem) SetAction(fn func()) *MenuItem {
	mi.action = fn
	return mi
}

// Trigger executes the item's action if defined and not disabled.
func (mi *MenuItem) Trigger() {
	if mi.disabled {
		return
	}
	if mi.action != nil {
		mi.action()
	}
}

// Selected reports whether the menu item is currently selected/highlighted.
func (mi *MenuItem) Selected() bool {
	return mi.selected
}

// SetSelected updates the selected/highlighted state of the menu item.
func (mi *MenuItem) SetSelected(sel bool) *MenuItem {
	if mi.selected != sel {
		mi.selected = sel
		mi.MarkDirty()
	}
	return mi
}

// Disabled reports whether the menu item is disabled.
func (mi *MenuItem) Disabled() bool {
	return mi.disabled
}

// SetDisabled updates the disabled state of the menu item.
func (mi *MenuItem) SetDisabled(dis bool) *MenuItem {
	if mi.disabled != dis {
		mi.disabled = dis
		mi.MarkDirty()
	}
	return mi
}

// Checked reports whether the checkable menu item is currently checked.
func (mi *MenuItem) Checked() bool {
	return mi.checked
}

// SetChecked updates the checked status of the menu item.
func (mi *MenuItem) SetChecked(chk bool) *MenuItem {
	if mi.checked != chk {
		mi.checked = chk
		mi.MarkDirty()
	}
	return mi
}

// Checkable reports whether the menu item displays a check/bullet indicator.
func (mi *MenuItem) Checkable() bool {
	return mi.checkable
}

// SetCheckable sets whether the menu item displays a check/bullet indicator.
func (mi *MenuItem) SetCheckable(chk bool) *MenuItem {
	if mi.checkable != chk {
		mi.checkable = chk
		mi.MarkDirty()
	}
	return mi
}

// Status returns a snapshot of the item's current state flags.
func (mi *MenuItem) Status() MenuItemStatus {
	return MenuItemStatus{
		Selected:  mi.selected,
		Disabled:  mi.disabled,
		Checked:   mi.checked,
		Checkable: mi.checkable,
	}
}

// SetStatus updates the item's state flags from a snapshot.
func (mi *MenuItem) SetStatus(st MenuItemStatus) *MenuItem {
	mi.selected = st.Selected
	mi.disabled = st.Disabled
	mi.checked = st.Checked
	mi.checkable = st.Checkable
	mi.MarkDirty()
	return mi
}

// HasSubmenu reports whether the menu item cascades into child submenu items.
func (mi *MenuItem) HasSubmenu() bool {
	return len(mi.Submenu) > 0
}

// SetSubmenu configures child cascading submenu items for this item.
func (mi *MenuItem) SetSubmenu(submenu ...*MenuItem) *MenuItem {
	mi.Submenu = submenu
	mi.MarkDirty()
	return mi
}

// MenuStyle returns the active or fallback style configuration for this menu item.
func (mi *MenuItem) MenuStyle() *MenuStyle {
	if mi.style == nil {
		return defaultMenuStyle
	}
	return mi.style
}

// SetStyle configures a custom MenuStyle for this menu item.
func (mi *MenuItem) SetStyle(s *MenuStyle) *MenuItem {
	mi.style = s
	mi.MarkDirty()
	return mi
}

// Width returns the display width needed for this menu item.
func (mi *MenuItem) Width() int {
	w := len(mi.Name) + 4
	if mi.checkable {
		w += 2
	}
	if len(mi.Submenu) > 0 {
		w += 2
	}
	return w
}

// AcceptsFocus implements tui.Focusable.
func (mi *MenuItem) AcceptsFocus() bool {
	return !mi.disabled
}

// Layout calculates size constraints for this menu item.
func (mi *MenuItem) Layout(c tui.Constraints) tui.Size {
	return c.Constrain(tui.Size{W: mi.Width(), H: 1})
}

// Render paints the menu item at origin (0, 0).
func (mi *MenuItem) Render(s tui.Surface) {
	sz := s.Size()
	w := sz.W
	if w <= 0 {
		w = mi.Width()
	}
	mi.RenderAt(s, 0, 0, w)
}

// RenderAt paints the menu item row on the surface at coordinates (x, y) with the given width.
func (mi *MenuItem) RenderAt(s tui.Surface, x, y, width int) {
	st := mi.style.ItemStyle(mi.selected)
	accSt := mi.style.AccentStyle(mi.selected)

	if mi.disabled {
		st = st.Faint(true)
		accSt = accSt.Faint(true)
	}

	// 1. Fill row background
	s.Fill(tui.Rect{X: x, Y: y, W: width, H: 1}, " ", st)

	// 2. Render bullet/check indicator if checkable
	textStartX := x + 1
	if mi.checkable {
		bullet := "  "
		if mi.checked {
			bullet = "• "
		}
		drawText(s, x+1, y, bullet, st)
		textStartX = x + 3
	}

	// 3. Render name with hotkey accent
	runes := []rune(mi.Name)
	for j, r := range runes {
		cellSt := st
		if j == mi.HotkeyIdx {
			cellSt = accSt
		}
		s.SetCell(textStartX+j, y, string(r), cellSt)
	}

	// 4. Render submenu arrow indicator ► if it has children
	if len(mi.Submenu) > 0 {
		arrowX := x + width - 2
		s.SetCell(arrowX, y, "►", st)
	}
}

// HandleEvent processes keyboard activation and hotkey mnemonic matching.
func (mi *MenuItem) HandleEvent(ev tui.Event) bool {
	if mi.disabled {
		return false
	}
	ke, ok := ev.(tui.KeyEvent)
	if !ok || ke.Kind != tui.KeyPress {
		return false
	}

	if mi.selected {
		if ke.Code == tui.KeyEnter || ke.Code == ' ' {
			mi.Trigger()
			return true
		}
	}

	// Match hotkey mnemonic
	codeLower := strings.ToLower(string(ke.Code))
	hotkeyLower := strings.ToLower(string(mi.Hotkey))
	if len(codeLower) > 0 && len(hotkeyLower) > 0 && codeLower == hotkeyLower {
		mi.Trigger()
		return true
	}

	return false
}
