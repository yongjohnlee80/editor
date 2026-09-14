package editor

import "github.com/yongjohnlee80/golib/tui/style"

// defaultModalStyle provides fallback styling for modal dialogs:
// rounded high-contrast card, cyan title, white body text, and stippled scrim.
var defaultModalStyle = &ModalStyle{
	card: style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(7)).
		Border(style.BorderRounded),
	title: style.New().
		Foreground(style.ANSI(14)).
		Bold(true),
	body: style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(7)),
	scrim: style.New().
		Foreground(style.ANSI(8)).
		Faint(true),
}

var (
	scrimStyle      = defaultModalStyle.scrim
	modalCardStyle  = defaultModalStyle.card
	modalTitleStyle = defaultModalStyle.title
)

// ModalStyle encapsulates the visual presentation attributes for a Modal widget,
// including its dialog card container, title header, body text, and background scrim.
type ModalStyle struct {
	card  style.Style
	title style.Style
	body  style.Style
	scrim style.Style
}

// NewModalStyle constructs a custom ModalStyle with the specified card, title, body, and scrim styles.
func NewModalStyle(card, title, body, scrim style.Style) *ModalStyle {
	return &ModalStyle{
		card:  card,
		title: title,
		body:  body,
		scrim: scrim,
	}
}

// Card returns the style used for the modal dialog box card.
// If the receiver is nil, it falls back to defaultModalStyle.
func (m *ModalStyle) Card() style.Style {
	if m == nil {
		return defaultModalStyle.card
	}
	return m.card
}

// Title returns the style used for the modal dialog title.
// If the receiver is nil, it falls back to defaultModalStyle.
func (m *ModalStyle) Title() style.Style {
	if m == nil {
		return defaultModalStyle.title
	}
	return m.title
}

// Body returns the style used for the modal dialog body text.
// If the receiver is nil, it falls back to defaultModalStyle.
func (m *ModalStyle) Body() style.Style {
	if m == nil {
		return defaultModalStyle.body
	}
	return m.body
}

// Scrim returns the style used for the backdrop scrim.
// If the receiver is nil, it falls back to defaultModalStyle.
func (m *ModalStyle) Scrim() style.Style {
	if m == nil {
		return defaultModalStyle.scrim
	}
	return m.scrim
}
