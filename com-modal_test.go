package editor

import (
	"strings"
	"testing"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
)

func TestModal_StandaloneWidget(t *testing.T) {
	var yesClicked, noClicked, dismissed bool
	yesBtn := NewButton("Yes", func() { yesClicked = true })
	noBtn := NewButton("No", func() { noClicked = true })

	modal := NewModal("Confirmation", "Are you sure?", yesBtn, noBtn)
	modal.OnDismiss(func() { dismissed = true })

	if modal.Title() != "Confirmation" {
		t.Fatalf("Title() = %q, want %q", modal.Title(), "Confirmation")
	}
	if modal.Body() != "Are you sure?" {
		t.Fatalf("Body() = %q, want %q", modal.Body(), "Are you sure?")
	}
	if len(modal.Buttons()) != 2 {
		t.Fatalf("len(Buttons()) = %d, want 2", len(modal.Buttons()))
	}
	if modal.SelectedButton() != 0 {
		t.Fatalf("initial SelectedButton() = %d, want 0", modal.SelectedButton())
	}
	if !yesBtn.Focused() || noBtn.Focused() {
		t.Fatal("button focus state not initialized properly")
	}

	// SetTitle and SetBody
	modal.SetTitle("New Title")
	modal.SetBody("New Body")
	if modal.Title() != "New Title" || modal.Body() != "New Body" {
		t.Fatal("SetTitle or SetBody failed")
	}
	modal.SetTitle("Confirmation")
	modal.SetBody("Are you sure?")

	// Navigation: right -> No, left -> Yes
	modal.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyRight})
	if modal.SelectedButton() != 1 || !noBtn.Focused() || yesBtn.Focused() {
		t.Fatalf("after KeyRight: SelectedButton() = %d, want 1", modal.SelectedButton())
	}

	modal.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyLeft})
	if modal.SelectedButton() != 0 || !yesBtn.Focused() || noBtn.Focused() {
		t.Fatalf("after KeyLeft: SelectedButton() = %d, want 0", modal.SelectedButton())
	}

	// Navigation with vim keys (l, h, j, k)
	modal.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: 'l'})
	if modal.SelectedButton() != 1 {
		t.Fatalf("after 'l': SelectedButton() = %d, want 1", modal.SelectedButton())
	}
	modal.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: 'h'})
	if modal.SelectedButton() != 0 {
		t.Fatalf("after 'h': SelectedButton() = %d, want 0", modal.SelectedButton())
	}

	// Tab and Shift+Tab navigation
	modal.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyTab})
	if modal.SelectedButton() != 1 {
		t.Fatalf("after KeyTab: SelectedButton() = %d, want 1", modal.SelectedButton())
	}
	modal.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyTab, Mods: tui.ModShift})
	if modal.SelectedButton() != 0 {
		t.Fatalf("after Shift+KeyTab: SelectedButton() = %d, want 0", modal.SelectedButton())
	}

	// KeyEnter triggers focused button (Yes)
	modal.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if !yesClicked {
		t.Fatal("KeyEnter did not trigger focused Yes button")
	}

	// Hotkey 'n' triggers No button
	modal.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: 'n'})
	if !noClicked {
		t.Fatal("hotkey 'n' did not trigger No button")
	}

	// KeyEscape triggers onDismiss
	modal.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEscape})
	if !dismissed {
		t.Fatal("KeyEscape did not invoke onDismiss callback")
	}

	// Dismiss() direct invocation
	dismissed = false
	modal.Dismiss()
	if !dismissed {
		t.Fatal("Dismiss() did not invoke onDismiss callback")
	}

	// SetSelectedButton
	modal.SetSelectedButton(1)
	if modal.SelectedButton() != 1 || !noBtn.Focused() {
		t.Fatal("SetSelectedButton(1) failed to focus button 1")
	}

	// SetButtons
	okBtn := NewButton("OK", nil)
	modal.SetButtons(okBtn)
	if len(modal.Buttons()) != 1 || modal.Buttons()[0].Label() != "OK" {
		t.Fatal("SetButtons failed")
	}
	if !okBtn.Focused() {
		t.Fatal("new button not focused after SetButtons")
	}

	// AcceptsFocus and Layout
	if !modal.AcceptsFocus() {
		t.Fatal("AcceptsFocus() should return true")
	}
	sz := modal.Layout(tui.Constraints{MaxW: 80, MaxH: 24})
	if sz.W != 80 || sz.H != 24 {
		t.Fatalf("Layout() = %+v, want (80, 24)", sz)
	}

	// SetStyles
	modal.SetStyles(
		style.New().Background(style.ANSI(0)),
		style.New().Foreground(style.ANSI(7)),
		style.New().Foreground(style.ANSI(7)),
		style.New().Foreground(style.ANSI(8)),
	)

	// Render onto surface
	surf := newMockSurface(40, 10)
	modal.Render(surf)
	screen := surf.String()
	if !strings.Contains(screen, "Confirmation") {
		t.Fatalf("rendered modal missing title; screen:\n%s", screen)
	}
	if !strings.Contains(screen, "Are you sure?") {
		t.Fatalf("rendered modal missing body text; screen:\n%s", screen)
	}
	if !strings.Contains(screen, "[ OK ]") {
		t.Fatalf("rendered modal missing OK button; screen:\n%s", screen)
	}

	// Multiline body rendering
	multiModal := NewModal("Multi", "Line 1\nLine 2\nLine 3", okBtn)
	multiSurf := newMockSurface(40, 12)
	multiModal.Render(multiSurf)
	multiScreen := multiSurf.String()
	if !strings.Contains(multiScreen, "Line 1") || !strings.Contains(multiScreen, "Line 2") {
		t.Fatalf("multiline modal missing lines; screen:\n%s", multiScreen)
	}
}

func TestModalStyle_Standalone(t *testing.T) {
	// Nil receiver tests fallback to defaultModalStyle
	var nilStyle *ModalStyle
	if nilStyle.Card() != defaultModalStyle.card {
		t.Fatal("nilStyle.Card() did not return defaultModalStyle.card")
	}
	if nilStyle.Title() != defaultModalStyle.title {
		t.Fatal("nilStyle.Title() did not return defaultModalStyle.title")
	}
	if nilStyle.Body() != defaultModalStyle.body {
		t.Fatal("nilStyle.Body() did not return defaultModalStyle.body")
	}
	if nilStyle.Scrim() != defaultModalStyle.scrim {
		t.Fatal("nilStyle.Scrim() did not return defaultModalStyle.scrim")
	}

	// Custom style
	c := style.New().Foreground(style.ANSI(1))
	ti := style.New().Foreground(style.ANSI(2))
	b := style.New().Foreground(style.ANSI(3))
	s := style.New().Foreground(style.ANSI(4))
	custom := NewModalStyle(c, ti, b, s)

	if custom.Card() != c || custom.Title() != ti || custom.Body() != b || custom.Scrim() != s {
		t.Fatal("custom ModalStyle getters did not return expected styles")
	}

	modal := NewModal("Custom", "Body")
	if modal.ModalStyle() != defaultModalStyle {
		t.Fatal("modal.ModalStyle() should return defaultModalStyle when unset")
	}

	modal.SetStyle(custom)
	if modal.ModalStyle() != custom {
		t.Fatal("modal.ModalStyle() did not return custom style after SetStyle")
	}
}
