package editor

import (
	"fmt"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// ButtonRole indicates the semantic role of a button within dialogs and forms.
type ButtonRole int

const (
	// ButtonRoleNormal is a standard interactive button.
	ButtonRoleNormal ButtonRole = iota
	// ButtonRoleDefault is the primary/default button activated on Enter in dialogs.
	ButtonRoleDefault
	// ButtonRoleCancel is the cancel/dismiss button activated on Escape in dialogs.
	ButtonRoleCancel
)

// Button is a standalone clickable/focusable button widget component.
// It encapsulates a label, an action callback, disabled state, semantic role,
// mnemonic hotkey, and component-level styles for normal, focused, and disabled states.
type Button struct {
	widget.Base

	label    string
	action   func()
	focused  bool
	disabled bool
	role     ButtonRole
	mnemonic rune

	style *ButtonStyle
}

// NewButton creates a new standalone Button widget with the given label and action.
// Styling is applied at the component level using high-contrast themes.
func NewButton(label string, action func()) *Button {
	return &Button{
		label:  label,
		action: action,
		role:   ButtonRoleNormal,
	}
}

// Init mounts the button into the TUI context.
func (b *Button) Init(ctx *tui.Context) {
	b.Base.Init(ctx)
}

// Label returns the button's display label.
func (b *Button) Label() string {
	return b.label
}

// SetLabel updates the button's display label and invalidates layout.
func (b *Button) SetLabel(l string) {
	if b.label != l {
		b.label = l
		b.RequestLayout()
		b.MarkDirty()
	}
}

// Action returns the button's callback action.
func (b *Button) Action() func() {
	return b.action
}

// SetAction updates the button's callback action.
func (b *Button) SetAction(fn func()) {
	b.action = fn
}

// Focused reports whether the button currently has focus.
func (b *Button) Focused() bool {
	return b.focused
}

// SetFocused sets whether the button is currently focused.
func (b *Button) SetFocused(f bool) {
	if b.disabled && f {
		return
	}
	if b.focused != f {
		b.focused = f
		b.MarkDirty()
	}
}

// Disabled reports whether the button is disabled.
func (b *Button) Disabled() bool {
	return b.disabled
}

// SetDisabled updates the button's disabled state. Disabled buttons cannot accept focus.
func (b *Button) SetDisabled(d bool) {
	if b.disabled != d {
		b.disabled = d
		if d && b.focused {
			b.focused = false
		}
		b.RequestLayout()
		b.MarkDirty()
	}
}

// Role returns the semantic role of the button.
func (b *Button) Role() ButtonRole {
	return b.role
}

// SetRole configures the semantic role of the button.
func (b *Button) SetRole(r ButtonRole) *Button {
	b.role = r
	return b
}

// Mnemonic returns the explicit hotkey rune configured for this button, if any.
func (b *Button) Mnemonic() rune {
	return b.mnemonic
}

// SetMnemonic sets an explicit hotkey mnemonic for this button.
func (b *Button) SetMnemonic(m rune) *Button {
	b.mnemonic = m
	return b
}

// ButtonStyle returns the computed style for the given focus state.
func (b *Button) ButtonStyle(focused bool) style.Style {
	if b.disabled {
		return b.style.Disabled()
	}
	return b.style.Style(focused)
}

// SetStyle configures a custom ButtonStyle for this button.
func (b *Button) SetStyle(s *ButtonStyle) *Button {
	b.style = s
	b.MarkDirty()
	return b
}

// SetStyles configures custom normal and focused styles for this button.
func (b *Button) SetStyles(normal, focused style.Style) *Button {
	b.style = NewButtonStyle(normal, focused)
	b.MarkDirty()
	return b
}

// FormattedLabel returns the bracketed label string: "[ <label> ]".
func (b *Button) FormattedLabel() string {
	return fmt.Sprintf("[ %s ]", b.label)
}

func (b *Button) measure(s string) int {
	if ctx := b.Context(); ctx != nil {
		return ctx.StringWidth(s)
	}
	return tui.StringWidth(s)
}

// Width returns the rendered display-cell width of the button: "[ label ]".
func (b *Button) Width() int {
	return b.measure(b.FormattedLabel())
}

// Trigger executes the button's callback action if defined and not disabled.
func (b *Button) Trigger() {
	if !b.disabled && b.action != nil {
		b.action()
	}
}

// AcceptsFocus implements tui.Focusable. Disabled buttons cannot accept focus.
func (b *Button) AcceptsFocus() bool {
	return !b.disabled
}

// Layout calculates the size constraints for the button.
func (b *Button) Layout(c tui.Constraints) tui.Size {
	return c.Constrain(tui.Size{W: b.Width(), H: 1})
}

// Render paints the button at origin (0, 0) onto the surface.
func (b *Button) Render(s tui.Surface) {
	b.RenderAt(s, 0, 0)
}

// RenderAt paints the button at specific (x, y) coordinates on the surface.
func (b *Button) RenderAt(s tui.Surface, x, y int) {
	st := b.style.Style(b.focused)
	if b.disabled {
		st = b.style.Disabled()
	}
	drawText(s, x, y, b.FormattedLabel(), st)
}

// HandleEvent processes framework focus events and keyboard activation.
func (b *Button) HandleEvent(ev tui.Event) bool {
	switch e := ev.(type) {
	case tui.FocusEvent:
		if !e.Terminal {
			b.focused = e.Gained && !b.disabled
			b.MarkDirty()
			return true
		}
	case tui.KeyEvent:
		if !b.focused || b.disabled {
			return false
		}
		if e.Kind == tui.KeyPress {
			if e.Code == tui.KeyEnter || e.Code == ' ' {
				b.Trigger()
				return true
			}
		}
	}
	return false
}
