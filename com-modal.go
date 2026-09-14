package editor

import (
	"strings"
	"unicode"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// Modal is a standalone dialog widget component. It takes a title, a text body,
// and a slice of Button widgets, managing navigation across buttons and rendering
// a centered framed card with scrim backdrop. It implements tui.FocusScope to trap
// focus within the dialog during interaction.
type Modal struct {
	widget.Base

	title     string
	body      string
	buttons   []*Button
	selected  int
	onDismiss func()

	style *ModalStyle
}

// NewModal constructs a standalone Modal widget taking title, body text, and buttons.
func NewModal(title, body string, buttons ...*Button) *Modal {
	var filtered []*Button
	for _, b := range buttons {
		if b != nil {
			filtered = append(filtered, b)
		}
	}
	m := &Modal{
		title:   title,
		body:    body,
		buttons: filtered,
	}
	m.syncFocus()
	return m
}

// Init mounts the modal into the TUI context and framework-mounts its child buttons.
func (m *Modal) Init(ctx *tui.Context) {
	m.Base.Init(ctx)
	for _, b := range m.buttons {
		if b != nil {
			ctx.Mount(b)
		}
	}
	m.syncFocus()
}

// TrapsFocus implements tui.FocusScope. Keyboard tab navigation is confined to this dialog.
func (m *Modal) TrapsFocus() bool {
	return true
}

// Title returns the modal dialog's title.
func (m *Modal) Title() string {
	return m.title
}

// SetTitle updates the modal dialog's title and invalidates layout.
func (m *Modal) SetTitle(title string) {
	if m.title != title {
		m.title = title
		m.RequestLayout()
		m.MarkDirty()
	}
}

// Body returns the modal dialog's body text.
func (m *Modal) Body() string {
	return m.body
}

// SetBody updates the modal dialog's body text and invalidates layout.
func (m *Modal) SetBody(body string) {
	if m.body != body {
		m.body = body
		m.RequestLayout()
		m.MarkDirty()
	}
}

// Buttons returns a defensive copy of the modal dialog's buttons.
func (m *Modal) Buttons() []*Button {
	out := make([]*Button, len(m.buttons))
	copy(out, m.buttons)
	return out
}

// SetButtons replaces the modal dialog's buttons, dynamically mounting new children.
func (m *Modal) SetButtons(buttons ...*Button) {
	var filtered []*Button
	for _, b := range buttons {
		if b != nil {
			filtered = append(filtered, b)
		}
	}

	if ctx := m.Context(); ctx != nil {
		for _, old := range m.buttons {
			ctx.Unmount(old)
		}
		for _, b := range filtered {
			ctx.Mount(b)
		}
	}

	m.buttons = filtered
	if m.selected >= len(m.buttons) {
		m.selected = max(0, len(m.buttons)-1)
	}
	m.syncFocus()
	m.RequestLayout()
	m.MarkDirty()
}

// SelectedButton returns the index of the currently focused button.
func (m *Modal) SelectedButton() int {
	return m.selected
}

// SetSelectedButton sets the index of the currently focused button.
func (m *Modal) SetSelectedButton(idx int) {
	if idx >= 0 && idx < len(m.buttons) {
		if m.buttons[idx].Disabled() {
			return
		}
		m.selected = idx
		m.syncFocus()
		m.MarkDirty()
	}
}

// SelectNext focuses the next enabled button, wrapping around.
func (m *Modal) SelectNext() {
	n := len(m.buttons)
	if n <= 1 {
		return
	}
	for step := 1; step < n; step++ {
		idx := (m.selected + step) % n
		if !m.buttons[idx].Disabled() {
			m.selected = idx
			m.syncFocus()
			m.MarkDirty()
			return
		}
	}
}

// SelectPrev focuses the previous enabled button, wrapping around.
func (m *Modal) SelectPrev() {
	n := len(m.buttons)
	if n <= 1 {
		return
	}
	for step := 1; step < n; step++ {
		idx := (m.selected - step + n) % n
		if !m.buttons[idx].Disabled() {
			m.selected = idx
			m.syncFocus()
			m.MarkDirty()
			return
		}
	}
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

func (m *Modal) syncFocus() {
	for i, b := range m.buttons {
		isSel := (i == m.selected)
		b.SetFocused(isSel)
		if isSel && b.Context() != nil {
			b.Context().RequestFocus()
		}
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

func (m *Modal) measure(s string) int {
	if ctx := m.Context(); ctx != nil {
		return ctx.StringWidth(s)
	}
	return tui.StringWidth(s)
}

// Render paints the scrim background and centered modal dialog card with pure rendering.
func (m *Modal) Render(s tui.Surface) {
	sz := s.Size()
	if sz.W <= 0 || sz.H <= 0 {
		return
	}

	st := m.ModalStyle()

	// 1. Scrim background
	s.Fill(tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H}, "░", st.Scrim())

	// 2. Compute card geometry using surface string widths
	cardW := 36
	titleW := s.StringWidth(m.title) + 6
	if titleW > cardW {
		cardW = titleW
	}

	lines := strings.Split(m.body, "\n")
	for _, l := range lines {
		lw := s.StringWidth(l) + 6
		if lw > cardW {
			cardW = lw
		}
	}

	spacing := 3
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

	// Clamp to available surface dimensions
	if cardW > sz.W {
		cardW = sz.W
	}
	if cardW < 20 && sz.W >= 20 {
		cardW = 20
	} else if cardW < 20 {
		cardW = sz.W
	}

	cardH := 7
	if len(lines) > 1 {
		cardH = len(lines) + 6
	}
	if cardH > sz.H {
		cardH = sz.H
	}
	if cardH < 5 && sz.H >= 5 {
		cardH = 5
	} else if cardH < 5 {
		cardH = sz.H
	}

	cx := max(0, (sz.W-cardW)/2)
	cy := max(0, (sz.H-cardH)/2)
	rect := tui.Rect{X: cx, Y: cy, W: cardW, H: cardH}

	// 3. Render framed card with Title style applied
	renderBoxFrame(s, rect, m.title, st.Card(), st.Title())
	if cardW > 2 && cardH > 2 {
		s.Fill(tui.Rect{X: cx + 1, Y: cy + 1, W: cardW - 2, H: cardH - 2}, " ", st.Card())
	}

	// 4. Render body text safely truncated with Graphemes
	maxTextW := max(1, cardW-4)
	if len(lines) == 1 {
		bodyText := truncateGraphemes(m.body, maxTextW, s.StringWidth)
		textW := s.StringWidth(bodyText)
		drawText(s, cx+max(1, (cardW-textW)/2), cy+2, bodyText, st.Body().Bold(true))
	} else {
		for i, line := range lines {
			lineY := cy + 2 + i
			if lineY < cy+cardH-3 {
				tLine := truncateGraphemes(line, maxTextW, s.StringWidth)
				textW := s.StringWidth(tLine)
				drawText(s, cx+max(1, (cardW-textW)/2), lineY, tLine, st.Body())
			}
		}
	}

	// 5. Render buttons
	btnCount := len(m.buttons)
	if btnCount == 0 || cardH < 4 {
		return
	}

	btnY := cy + cardH - 2
	if btnY <= cy+2 {
		btnY = cy + cardH - 1
	}
	startX := cx + max(1, (cardW-totalButtonsWidth)/2)
	currX := startX
	for _, b := range m.buttons {
		if currX+b.Width() <= cx+cardW {
			b.RenderAt(s, currX, btnY)
		}
		currX += b.Width() + spacing
	}
}

// truncateGraphemes safely truncates s to maxW terminal columns using grapheme clusters.
func truncateGraphemes(s string, maxW int, widthFn func(string) int) string {
	if widthFn(s) <= maxW {
		return s
	}
	targetW := maxW
	if targetW > 3 {
		targetW -= 3
	} else {
		targetW = 1
	}

	var sb strings.Builder
	currW := 0
	for g := range tui.Graphemes(s) {
		gw := widthFn(g)
		if currW+gw > targetW {
			break
		}
		sb.WriteString(g)
		currW += gw
	}
	if maxW > 3 {
		sb.WriteString("...")
	}
	return sb.String()
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
		// Search for explicit cancel button
		for _, b := range m.buttons {
			if b.Role() == ButtonRoleCancel && !b.Disabled() {
				b.Trigger()
				return true
			}
		}
		// Fallback to button labeled Cancel/No/Close or last button
		for _, b := range m.buttons {
			lbl := strings.ToLower(b.Label())
			if (lbl == "cancel" || lbl == "no" || lbl == "close") && !b.Disabled() {
				b.Trigger()
				return true
			}
		}
		if len(m.buttons) > 0 && !m.buttons[len(m.buttons)-1].Disabled() {
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
		// Check for explicit button mnemonic
		keyRune := rune(ke.Code)
		for i, b := range m.buttons {
			if b.Disabled() {
				continue
			}
			if b.Mnemonic() != 0 && unicode.ToLower(b.Mnemonic()) == unicode.ToLower(keyRune) {
				m.SetSelectedButton(i)
				b.Trigger()
				return true
			}
		}
	}
	return false
}
