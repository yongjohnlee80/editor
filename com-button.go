package editor

import (
	"fmt"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// Button is a standalone clickable/focusable button widget component.
// It encapsulates a label, an action callback, and component-level styles
// for normal and focused states.
type Button struct {
	widget.Base

	label   string
	action  func()
	focused bool

	style *ButtonStyle
}

// NewButton creates a new standalone Button widget with the given label and action.
// Styling is applied at the component level using high-contrast themes.
func NewButton(label string, action func()) *Button {
	return &Button{
		label:  label,
		action: action,
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

// SetLabel updates the button's display label.
func (b *Button) SetLabel(l string) {
	b.label = l
	b.MarkDirty()
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
	if b.focused != f {
		b.focused = f
		b.MarkDirty()
	}
}

func (b *Button) ButtonStyle(focused bool) style.Style {
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

// Width returns the rendered character width of the button: "[ label ]".
func (b *Button) Width() int {
	return len(b.label) + 4
}

// Trigger executes the button's callback action if defined.
func (b *Button) Trigger() {
	if b.action != nil {
		b.action()
	}
}

// AcceptsFocus implements tui.Focusable.
func (b *Button) AcceptsFocus() bool {
	return true
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
	drawText(s, x, y, b.FormattedLabel(), b.style.Style(b.focused))
}

// HandleEvent processes keyboard activation when focused.
func (b *Button) HandleEvent(ev tui.Event) bool {
	if !b.focused {
		return false
	}
	if ke, ok := ev.(tui.KeyEvent); ok && ke.Kind == tui.KeyPress {
		if ke.Code == tui.KeyEnter || ke.Code == ' ' {
			b.Trigger()
			return true
		}
	}
	return false
}
