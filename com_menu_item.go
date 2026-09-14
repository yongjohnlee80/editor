package editor

import (
	"unicode"

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

// MenuItem is a standalone menu item widget component.
// It encapsulates display label, hotkey accelerator, semantic action callback,
// state/status flags (selected, disabled, checked), and optional submenu children.
// It is completely decoupled from application domain types or keysets.
type MenuItem struct {
	widget.Base

	Name string
	// Hotkey is the mnemonic rune character for shortcut activation.
	Hotkey rune
	// HotkeyIdx is the 0-based grapheme index within Name of the mnemonic character to highlight.
	HotkeyIdx int

	action func()

	// Status flags
	focused   bool
	selected  bool
	disabled  bool
	checked   bool
	checkable bool

	submenu []*MenuItem

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

// NewMenuItemWithSubmenu constructs a MenuItem that cascades into nested child submenu items.
func NewMenuItemWithSubmenu(name string, hotkey rune, hotkeyIdx int, submenu ...*MenuItem) *MenuItem {
	var filtered []*MenuItem
	for _, sub := range submenu {
		if sub != nil {
			filtered = append(filtered, sub)
		}
	}
	return &MenuItem{
		Name:      name,
		Hotkey:    hotkey,
		HotkeyIdx: hotkeyIdx,
		submenu:   filtered,
	}
}

// NewCheckableMenuItem constructs a checkable MenuItem with initial checked state and action.
func NewCheckableMenuItem(name string, hotkey rune, hotkeyIdx int, checked bool, action func()) *MenuItem {
	return &MenuItem{
		Name:      name,
		Hotkey:    hotkey,
		HotkeyIdx: hotkeyIdx,
		action:    action,
		checkable: true,
		checked:   checked,
	}
}

// Init mounts the menu item into the TUI context.
func (mi *MenuItem) Init(ctx *tui.Context) {
	mi.Base.Init(ctx)
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

// Focused reports whether the menu item currently has focus.
func (mi *MenuItem) Focused() bool {
	return mi.focused
}

// SetFocused sets whether the menu item currently has focus.
func (mi *MenuItem) SetFocused(f bool) *MenuItem {
	if mi.disabled && f {
		return mi
	}
	if mi.focused != f {
		mi.focused = f
		mi.MarkDirty()
	}
	return mi
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

// SetDisabled updates the disabled state of the menu item and invalidates layout.
func (mi *MenuItem) SetDisabled(dis bool) *MenuItem {
	if mi.disabled != dis {
		mi.disabled = dis
		if dis && mi.focused {
			mi.focused = false
		}
		mi.RequestLayout()
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

// SetCheckable sets whether the menu item displays a check/bullet indicator and invalidates layout.
func (mi *MenuItem) SetCheckable(chk bool) *MenuItem {
	if mi.checkable != chk {
		mi.checkable = chk
		mi.RequestLayout()
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
	mi.RequestLayout()
	mi.MarkDirty()
	return mi
}

// HasSubmenu reports whether the menu item cascades into child submenu items.
func (mi *MenuItem) HasSubmenu() bool {
	return len(mi.submenu) > 0
}

// Submenu returns a defensive copy of the child submenu items.
func (mi *MenuItem) Submenu() []*MenuItem {
	out := make([]*MenuItem, len(mi.submenu))
	copy(out, mi.submenu)
	return out
}

// SetSubmenu configures child cascading submenu items as an extensible data model.
func (mi *MenuItem) SetSubmenu(submenu ...*MenuItem) *MenuItem {
	var filtered []*MenuItem
	for _, sub := range submenu {
		if sub != nil {
			filtered = append(filtered, sub)
		}
	}
	mi.submenu = filtered
	mi.RequestLayout()
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

func (mi *MenuItem) measure(s string) int {
	if ctx := mi.Context(); ctx != nil {
		return ctx.StringWidth(s)
	}
	return tui.StringWidth(s)
}

// Width returns the display width needed for this menu item using policy-aware measurement.
func (mi *MenuItem) Width() int {
	w := mi.measure(mi.Name) + 4
	if mi.checkable {
		w += 2
	}
	if len(mi.submenu) > 0 {
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
	highlighted := (mi.selected || mi.focused) && !mi.disabled
	st := mi.MenuStyle().ItemStyle(highlighted)
	accSt := mi.MenuStyle().AccentStyle(highlighted)

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

	// 3. Render name with hotkey accent, honoring grapheme display cells
	col := textStartX
	runeIdx := 0
	for g := range tui.Graphemes(mi.Name) {
		gw := s.StringWidth(g)
		if col+gw > x+width-1 {
			break
		}
		cellSt := st
		if runeIdx == mi.HotkeyIdx {
			cellSt = accSt
		}
		s.SetCell(col, y, g, cellSt)
		col += gw
		runeIdx++
	}

	// 4. Render submenu arrow indicator ► if it has children
	if len(mi.submenu) > 0 && x+width-2 >= x {
		arrowX := x + width - 2
		s.SetCell(arrowX, y, "►", st)
	}
}

// HandleEvent processes framework focus events, keyboard activation, and hotkey mnemonic matching.
func (mi *MenuItem) HandleEvent(ev tui.Event) bool {
	switch e := ev.(type) {
	case tui.FocusEvent:
		if !e.Terminal {
			mi.focused = e.Gained && !mi.disabled
			mi.MarkDirty()
			return false // Bubble up to ancestor
		}
	case tui.KeyEvent:
		if mi.disabled || e.Kind != tui.KeyPress {
			return false
		}

		if mi.focused || mi.selected {
			if e.Code == tui.KeyEnter || e.Code == ' ' {
				mi.Trigger()
				return true
			}
		}

		// Match hotkey mnemonic
		if mi.Hotkey != 0 && unicode.ToLower(rune(e.Code)) == unicode.ToLower(mi.Hotkey) {
			mi.Trigger()
			return true
		}
	}

	return false
}
