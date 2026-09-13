package editor

import "github.com/yongjohnlee80/golib/tui/style"

// defaultButtonStyle provides the fallback high-contrast styling for buttons
// (black background with white text when unfocused; inverted with bold text when focused).
var defaultButtonStyle = &ButtonStyle{
	normal: style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(7)),
	focused: style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(0)).
		Bold(true),
}

var (
	buttonNormalStyle  = defaultButtonStyle.normal
	buttonFocusedStyle = defaultButtonStyle.focused
)

// ButtonStyle encapsulates the visual presentation attributes for a Button widget
// across its normal (unfocused) and focused interaction states.
type ButtonStyle struct {
	normal  style.Style
	focused style.Style
}

// NewButtonStyle constructs a custom ButtonStyle with the specified normal and focused styles.
func NewButtonStyle(normal, focused style.Style) *ButtonStyle {
	return &ButtonStyle{
		normal:  normal,
		focused: focused,
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

// Style returns either the focused or normal style depending on the given focus state.
func (b *ButtonStyle) Style(focused bool) style.Style {
	switch focused {
	case true:
		return b.Focused()
	default:
		return b.Normal()
	}
}
