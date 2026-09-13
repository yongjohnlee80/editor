package editor

import (
	"strings"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// Modal is a standalone dialog widget component. It takes a title, a text body,
// and a slice of Button widgets, managing navigation across buttons and rendering
// a centered framed card with scrim backdrop.
type Modal struct {
	widget.Base

	title   string
	body    string
	buttons []*Button

	selected  int
	onDismiss func()

	style *ModalStyle
}

// NewModal constructs a standalone Modal widget taking title, body text, and buttons.
func NewModal(title, body string, buttons ...*Button) *Modal {
	m := &Modal{
		title:   title,
		body:    body,
		buttons: buttons,
	}
	m.updateButtonFocus()
	return m
}

// Init mounts the modal into the TUI context and initializes its child buttons.
func (m *Modal) Init(ctx *tui.Context) {
	m.Base.Init(ctx)
	for _, b := range m.buttons {
		b.Init(ctx)
	}
}

// Title returns the modal dialog's title.
func (m *Modal) Title() string {
	return m.title
}

// SetTitle updates the modal dialog's title.
func (m *Modal) SetTitle(title string) {
	m.title = title
	m.MarkDirty()
}

// Body returns the modal dialog's body text.
func (m *Modal) Body() string {
	return m.body
}

// SetBody updates the modal dialog's body text.
func (m *Modal) SetBody(body string) {
	m.body = body
	m.MarkDirty()
}

// Buttons returns the modal dialog's slice of buttons.
func (m *Modal) Buttons() []*Button {
	return m.buttons
}

// SetButtons replaces the modal dialog's buttons.
func (m *Modal) SetButtons(buttons ...*Button) {
	m.buttons = buttons
	if m.selected >= len(m.buttons) {
		m.selected = 0
	}
	m.updateButtonFocus()
	m.MarkDirty()
}

// SelectedButton returns the index of the currently focused button.
func (m *Modal) SelectedButton() int {
	return m.selected
}

// SetSelectedButton sets the index of the currently focused button.
func (m *Modal) SetSelectedButton(idx int) {
	if idx >= 0 && idx < len(m.buttons) {
		m.selected = idx
		m.updateButtonFocus()
		m.MarkDirty()
	}
}

// SelectNext focuses the next button, wrapping around to the first.
func (m *Modal) SelectNext() {
	if len(m.buttons) == 0 {
		return
	}
	m.selected = (m.selected + 1) % len(m.buttons)
	m.updateButtonFocus()
	m.MarkDirty()
}

// SelectPrev focuses the previous button, wrapping around to the last.
func (m *Modal) SelectPrev() {
	if len(m.buttons) == 0 {
		return
	}
	m.selected = (m.selected - 1 + len(m.buttons)) % len(m.buttons)
	m.updateButtonFocus()
	m.MarkDirty()
}

// TriggerFocused triggers the currently focused button's action.
func (m *Modal) TriggerFocused() {
	if m.selected >= 0 && m.selected < len(m.buttons) {
		m.buttons[m.selected].Trigger()
	}
}

// OnDismiss sets a callback function invoked when the modal is dismissed via Escape.
func (m *Modal) OnDismiss(fn func()) {
	m.onDismiss = fn
}

// Dismiss invokes the onDismiss callback if configured.
func (m *Modal) Dismiss() {
	if m.onDismiss != nil {
		m.onDismiss()
	}
}

// ModalStyle returns the active or fallback style configuration for this modal.
func (m *Modal) ModalStyle() *ModalStyle {
	if m.style == nil {
		return defaultModalStyle
	}
	return m.style
}

// SetStyle configures a custom ModalStyle for this modal.
func (m *Modal) SetStyle(s *ModalStyle) *Modal {
	m.style = s
	m.MarkDirty()
	return m
}

// SetStyles configures the styling attributes of the modal card, title, body, and scrim.
func (m *Modal) SetStyles(card, title, body, scrim style.Style) *Modal {
	m.style = NewModalStyle(card, title, body, scrim)
	m.MarkDirty()
	return m
}

func (m *Modal) updateButtonFocus() {
	for i, b := range m.buttons {
		b.SetFocused(i == m.selected)
	}
}

// AcceptsFocus implements tui.Focusable.
func (m *Modal) AcceptsFocus() bool {
	return true
}

// Layout sizes the modal to match container constraints.
func (m *Modal) Layout(c tui.Constraints) tui.Size {
	return c.Constrain(tui.Size{W: c.MaxW, H: c.MaxH})
}

// Render paints the scrim background and centered modal dialog card.
func (m *Modal) Render(s tui.Surface) {
	sz := s.Size()
	if sz.W <= 0 || sz.H <= 0 {
		return
	}

	// 1. Scrim background
	s.Fill(tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H}, "░", m.style.Scrim())

	// 2. Compute card geometry
	cardW := 36
	if len(m.title)+6 > cardW {
		cardW = len(m.title) + 6
	}
	if len(m.body)+6 > cardW {
		cardW = len(m.body) + 6
	}

	spacing := 5
	totalButtonsWidth := 0
	for i, b := range m.buttons {
		totalButtonsWidth += b.Width()
		if i > 0 {
			totalButtonsWidth += spacing
		}
	}
	if totalButtonsWidth+6 > cardW {
		cardW = totalButtonsWidth + 6
	}
	if cardW > sz.W-2 {
		cardW = sz.W - 2
	}
	if cardW < 20 {
		cardW = 20
	}

	lines := strings.Split(m.body, "\n")
	cardH := 7
	if len(lines) > 1 {
		cardH = len(lines) + 6
	}
	if cardH > sz.H-2 {
		cardH = sz.H - 2
	}
	if cardH < 5 {
		cardH = 5
	}

	cx := (sz.W - cardW) / 2
	cy := (sz.H - cardH) / 2
	rect := tui.Rect{X: cx, Y: cy, W: cardW, H: cardH}

	// 3. Render framed card
	renderBoxFrame(s, rect, m.title, m.style.Card())
	s.Fill(tui.Rect{X: cx + 1, Y: cy + 1, W: cardW - 2, H: cardH - 2}, " ", m.style.Card())

	// 4. Render body text
	if len(lines) == 1 {
		bodyText := m.body
		if len(bodyText) > cardW-4 {
			bodyText = bodyText[:cardW-7] + "..."
		}
		drawText(s, cx+(cardW-len(bodyText))/2, cy+2, bodyText, m.style.Body().Bold(true))
	} else {
		for i, line := range lines {
			lineY := cy + 2 + i
			if lineY < cy+cardH-3 {
				if len(line) > cardW-4 {
					line = line[:cardW-7] + "..."
				}
				drawText(s, cx+(cardW-len(line))/2, lineY, line, m.style.Body())
			}
		}
	}

	// 5. Render buttons
	m.updateButtonFocus()
	btnCount := len(m.buttons)
	if btnCount == 0 {
		return
	}

	btnY := cy + cardH - 3
	startX := cx + (cardW-totalButtonsWidth)/2
	currX := startX
	for _, b := range m.buttons {
		b.RenderAt(s, currX, btnY)
		currX += b.Width() + spacing
	}
}

// HandleEvent processes keyboard navigation and activation for the modal.
func (m *Modal) HandleEvent(ev tui.Event) bool {
	ke, ok := ev.(tui.KeyEvent)
	if !ok || ke.Kind != tui.KeyPress {
		return false
	}

	switch ke.Code {
	case tui.KeyEscape:
		if m.onDismiss != nil {
			m.onDismiss()
			return true
		}
		for _, b := range m.buttons {
			lbl := strings.ToLower(b.label)
			if lbl == "no" || lbl == "cancel" || lbl == "close" {
				b.Trigger()
				return true
			}
		}
		if len(m.buttons) > 0 {
			m.buttons[len(m.buttons)-1].Trigger()
			return true
		}
		return false

	case tui.KeyLeft, tui.KeyUp, 'h', 'k':
		if len(m.buttons) > 1 {
			m.SelectPrev()
			return true
		}

	case tui.KeyRight, tui.KeyDown, 'l', 'j':
		if len(m.buttons) > 1 {
			m.SelectNext()
			return true
		}

	case tui.KeyTab:
		if len(m.buttons) > 1 {
			if ke.Mods&tui.ModShift != 0 {
				m.SelectPrev()
			} else {
				m.SelectNext()
			}
			return true
		}

	case tui.KeyEnter, ' ':
		m.TriggerFocused()
		return true

	default:
		// Check for mnemonic hotkey matching first letter of button label
		codeLower := strings.ToLower(string(ke.Code))
		if len(codeLower) > 0 {
			r := []rune(codeLower)[0]
			for i, b := range m.buttons {
				lblLower := strings.ToLower(b.label)
				if len(lblLower) > 0 && []rune(lblLower)[0] == r {
					m.SetSelectedButton(i)
					b.Trigger()
					return true
				}
			}
		}
	}
	return false
}
