package editor

import "github.com/yongjohnlee80/golib/tui/style"

// defaultMenuStyle provides the fallback Borland / high-contrast menu styling.
var defaultMenuStyle = &MenuStyle{
	bar: style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(0)),
	accent: style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(1)).
		Bold(true),
	highlight: style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(7)).
		Bold(true),
	highlightAccent: style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(1)).
		Bold(true),
	border: style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(0)).
		Border(style.BorderNormal),
}

var (
	menuBarStyle             = defaultMenuStyle.bar
	menuAccentStyle          = defaultMenuStyle.accent
	menuHighlightStyle       = defaultMenuStyle.highlight
	menuHighlightAccentStyle = defaultMenuStyle.highlightAccent
	dropdownBorder           = defaultMenuStyle.border
)

// MenuStyle encapsulates the visual presentation attributes for the TopMenu subsystem,
// including the menu bar fill, hotkey accents, selection highlights, and dropdown/submenu borders.
type MenuStyle struct {
	bar             style.Style
	accent          style.Style
	highlight       style.Style
	highlightAccent style.Style
	border          style.Style
}

// NewMenuStyle constructs a custom MenuStyle with the specified styling attributes.
func NewMenuStyle(bar, accent, highlight, highlightAccent, border style.Style) *MenuStyle {
	return &MenuStyle{
		bar:             bar,
		accent:          accent,
		highlight:       highlight,
		highlightAccent: highlightAccent,
		border:          border,
	}
}

// Bar returns the base style for the menu bar and unselected items.
// If the receiver is nil, it falls back to defaultMenuStyle.
func (m *MenuStyle) Bar() style.Style {
	if m == nil {
		return defaultMenuStyle.bar
	}
	return m.bar
}

// Accent returns the style for unselected hotkey accelerator letters.
// If the receiver is nil, it falls back to defaultMenuStyle.
func (m *MenuStyle) Accent() style.Style {
	if m == nil {
		return defaultMenuStyle.accent
	}
	return m.accent
}

// Highlight returns the style for selected/highlighted items.
// If the receiver is nil, it falls back to defaultMenuStyle.
func (m *MenuStyle) Highlight() style.Style {
	if m == nil {
		return defaultMenuStyle.highlight
	}
	return m.highlight
}

// HighlightAccent returns the style for hotkey accelerator letters on selected items.
// If the receiver is nil, it falls back to defaultMenuStyle.
func (m *MenuStyle) HighlightAccent() style.Style {
	if m == nil {
		return defaultMenuStyle.highlightAccent
	}
	return m.highlightAccent
}

// Border returns the style for dropdown and cascading submenu box borders.
// If the receiver is nil, it falls back to defaultMenuStyle.
func (m *MenuStyle) Border() style.Style {
	if m == nil {
		return defaultMenuStyle.border
	}
	return m.border
}

// ItemStyle returns either the highlight or bar style depending on selection state.
func (m *MenuStyle) ItemStyle(selected bool) style.Style {
	if selected {
		return m.Highlight()
	}
	return m.Bar()
}

// AccentStyle returns either the highlightAccent or accent style depending on selection state.
func (m *MenuStyle) AccentStyle(selected bool) style.Style {
	if selected {
		return m.HighlightAccent()
	}
	return m.Accent()
}
