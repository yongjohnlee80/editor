package editor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
)

func TestModal_StandaloneWidget(t *testing.T) {
	var yesClicked, noClicked, dismissed bool
	yesBtn := NewButton("Yes", func() { yesClicked = true }).SetRole(ButtonRoleDefault).SetMnemonic('y')
	noBtn := NewButton("No", func() { noClicked = true }).SetRole(ButtonRoleCancel).SetMnemonic('n')

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
	if !modal.TrapsFocus() {
		t.Fatal("Modal must implement FocusScope with TrapsFocus() == true")
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

	// Mnemonic 'n' triggers No button
	modal.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: 'n'})
	if !noClicked {
		t.Fatal("mnemonic 'n' did not trigger No button")
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

	// SetButtons defensive copy and nil filtering
	okBtn := NewButton("OK", nil)
	modal.SetButtons(okBtn, nil)
	if len(modal.Buttons()) != 1 || modal.Buttons()[0].Label() != "OK" {
		t.Fatal("SetButtons failed or did not filter nils")
	}
	if !okBtn.Focused() {
		t.Fatal("new button not focused after SetButtons")
	}

	// Mutating returned slice should not affect modal internals
	retButtons := modal.Buttons()
	retButtons[0] = nil
	if modal.Buttons()[0] == nil {
		t.Fatal("modal.Buttons() must return a defensive copy")
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
		style.New().Foreground(style.ANSI(14)),
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
}

func TestModal_ChildMountingAndDistinctNodeIDs(t *testing.T) {
	tb := tui.NewTestBackend(80, 24)
	btn1 := NewButton("First", nil)
	btn2 := NewButton("Second", nil)
	modal := NewModal("Title", "Body", btn1, btn2)

	ctx, cancel := context.WithCancel(context.Background())
	app := tui.NewApp(modal, tui.WithBackend(tb))
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("App.Run did not return within 3s after cancel")
		}
	}()

	var modalID, btn1ID, btn2ID tui.NodeID
	doneUpdate := make(chan struct{})
	app.Update(func() {
		modalID = modal.NodeID()
		btn1ID = btn1.NodeID()
		btn2ID = btn2.NodeID()
		close(doneUpdate)
	})
	select {
	case <-doneUpdate:
	case <-time.After(3 * time.Second):
		t.Fatal("app.Update did not execute within 3s")
	}

	if modalID == 0 || btn1ID == 0 || btn2ID == 0 {
		t.Fatalf("expected non-zero NodeIDs for all mounted components: modal=%d, btn1=%d, btn2=%d", modalID, btn1ID, btn2ID)
	}

	if btn1ID == modalID || btn2ID == modalID {
		t.Fatalf("child buttons must not reuse parent Modal NodeID %d; got btn1=%d, btn2=%d", modalID, btn1ID, btn2ID)
	}

	if btn1ID == btn2ID {
		t.Fatalf("child buttons must each have distinct NodeIDs; got btn1=%d, btn2=%d", btn1ID, btn2ID)
	}
}

func TestModal_TinySurfaceSafety(t *testing.T) {
	btn := NewButton("OK", nil)
	modal := NewModal("Very Long Title That Exceeds Viewport", "Very long body text that also exceeds viewport width and height", btn)

	// Test tiny surface (10x3)
	tinySurf := newMockSurface(10, 3)
	// Must not panic or produce out-of-bounds coordinates
	modal.Render(tinySurf)
	s := tinySurf.String()
	if len(s) == 0 {
		t.Fatal("tiny surface rendering produced empty string")
	}
}

func TestModal_UnicodeDisplayCells(t *testing.T) {
	btn := NewButton("确定", nil)
	modal := NewModal("确认退出", "你确定要退出吗？", btn)

	surf := newMockSurface(40, 10)
	modal.Render(surf)
	screen := surf.String()
	if !strings.Contains(screen, "确认退出") {
		t.Fatalf("rendered modal missing CJK title; screen:\n%s", screen)
	}
	if !strings.Contains(screen, "你确定要退出吗？") {
		t.Fatalf("rendered modal missing CJK body; screen:\n%s", screen)
	}
}

func TestModalStyle_Standalone(t *testing.T) {
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
