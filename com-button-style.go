package editor

import "github.com/yongjohnlee80/golib/tui/style"

// defaultButtonStyle provides the fallback high-contrast styling for buttons
// (black background with white text when unfocused; inverted with bold text when focused; faint when disabled).
var defaultButtonStyle = &ButtonStyle{
	normal: style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(7)),
	focused: style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(0)).
		Bold(true),
	disabled: style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(8)).
		Faint(true),
}

var (
	buttonNormalStyle   = defaultButtonStyle.normal
	buttonFocusedStyle  = defaultButtonStyle.focused
	buttonDisabledStyle = defaultButtonStyle.disabled
)

// ButtonStyle encapsulates the visual presentation attributes for a Button widget
// across its normal (unfocused), focused, and disabled interaction states.
type ButtonStyle struct {
	normal   style.Style
	focused  style.Style
	disabled style.Style
}

// NewButtonStyle constructs a custom ButtonStyle with the specified normal and focused styles.
func NewButtonStyle(normal, focused style.Style) *ButtonStyle {
	return &ButtonStyle{
		normal:   normal,
		focused:  focused,
		disabled: normal.Faint(true),
	}
}

// NewButtonStyleWithDisabled constructs a custom ButtonStyle with normal, focused, and disabled styles.
func NewButtonStyleWithDisabled(normal, focused, disabled style.Style) *ButtonStyle {
	return &ButtonStyle{
		normal:   normal,
		focused:  focused,
		disabled: disabled,
	}
}

// Normal returns the style used when the button is unfocused.
// If the receiver is nil, it falls back to defaultButtonStyle.
func (b *ButtonStyle) Normal() style.Style {
	if b == nil {
		return defaultButtonStyle.normal
	}
	return b.normal
}

// Focused returns the style used when the button is focused.
// If the receiver is nil, it falls back to defaultButtonStyle.
func (b *ButtonStyle) Focused() style.Style {
	if b == nil {
		return defaultButtonStyle.focused
	}
	return b.focused
}

// Disabled returns the style used when the button is disabled.
// If the receiver is nil, it falls back to defaultButtonStyle.
func (b *ButtonStyle) Disabled() style.Style {
	if b == nil {
		return defaultButtonStyle.disabled
	}
	return b.disabled
}

// Style returns either the focused, disabled, or normal style depending on state.
func (b *ButtonStyle) Style(focused bool) style.Style {
	if focused {
		return b.Focused()
	}
	return b.Normal()
}
