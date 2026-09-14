package editor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
)

func TestMenuItem_StandaloneWidget(t *testing.T) {
	var triggered bool
	item := NewMenuItem("Open", 'o', 0, func() {
		triggered = true
	})

	if item.Name != "Open" {
		t.Fatalf("Name = %q, want %q", item.Name, "Open")
	}
	if item.Hotkey != 'o' || item.HotkeyIdx != 0 {
		t.Fatalf("Hotkey = %c, Idx = %d, want 'o', 0", item.Hotkey, item.HotkeyIdx)
	}

	// Trigger action
	item.Trigger()
	if !triggered {
		t.Fatal("Trigger did not invoke action callback")
	}

	// Status flags
	if item.Selected() {
		t.Fatal("initial Selected() should be false")
	}
	item.SetSelected(true)
	if !item.Selected() {
		t.Fatal("after SetSelected(true): Selected() should be true")
	}

	if item.Disabled() {
		t.Fatal("initial Disabled() should be false")
	}
	item.SetDisabled(true)
	if !item.Disabled() {
		t.Fatal("after SetDisabled(true): Disabled() should be true")
	}
	if item.AcceptsFocus() {
		t.Fatal("disabled item must not accept focus")
	}

	// Trigger on disabled item must not invoke action
	triggered = false
	item.Trigger()
	if triggered {
		t.Fatal("disabled item must not trigger action")
	}

	// Re-enable item
	item.SetDisabled(false)

	// Checked / Checkable flags
	if item.Checked() || item.Checkable() {
		t.Fatal("initial Checked / Checkable should be false")
	}
	item.SetCheckable(true)
	item.SetChecked(true)
	if !item.Checked() || !item.Checkable() {
		t.Fatal("SetCheckable / SetChecked failed")
	}

	// Status snapshot
	st := item.Status()
	if !st.Selected || st.Disabled || !st.Checked || !st.Checkable {
		t.Fatalf("Status() snapshot mismatch: %+v", st)
	}
	item.SetStatus(MenuItemStatus{Selected: false, Disabled: false, Checked: false, Checkable: false})
	if item.Selected() || item.Checked() {
		t.Fatal("SetStatus failed to update flags")
	}

	// Keyboard event handling
	item.SetSelected(true)
	handled := item.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if !handled || !triggered {
		t.Fatal("selected item must handle KeyEnter and invoke action")
	}

	triggered = false
	handled = item.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: ' '})
	if !handled || !triggered {
		t.Fatal("selected item must handle Space and invoke action")
	}

	// Hotkey mnemonic triggering
	triggered = false
	item.SetSelected(false)
	handled = item.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: 'o'})
	if !handled || !triggered {
		t.Fatal("item must handle hotkey 'o' mnemonic even when unselected")
	}

	// Custom styles
	customStyle := NewMenuStyle(
		style.New().Background(style.ANSI(0)),
		style.New().Foreground(style.ANSI(1)),
		style.New().Background(style.ANSI(7)),
		style.New().Foreground(style.ANSI(1)),
		style.New().Border(style.BorderNormal),
	)
	item.SetStyle(customStyle)
	if item.MenuStyle() != customStyle {
		t.Fatal("SetStyle did not update item MenuStyle")
	}
}

func TestMenuItem_SubmenuAndChildMounting(t *testing.T) {
	var sub1Triggered bool
	sub1 := NewCheckableMenuItem("1. Vim  (modal)", '1', 0, true, func() {
		sub1Triggered = true
	})
	sub2 := NewCheckableMenuItem("2. Nano (modeless)", '2', 0, false, nil)

	parent := NewMenuItemWithSubmenu("Keymaps", 'k', 0, sub1, sub2)

	if !parent.HasSubmenu() {
		t.Fatal("parent.HasSubmenu() should return true")
	}
	if len(parent.Submenu()) != 2 {
		t.Fatalf("len(parent.Submenu()) = %d, want 2", len(parent.Submenu()))
	}
	if !sub1.Checkable() {
		t.Fatal("sub1 should be checkable")
	}
	if !sub1.Checked() {
		t.Fatal("sub1 should be checked initially")
	}

	sub1.Trigger()
	if !sub1Triggered {
		t.Fatal("sub1.Trigger() did not invoke action")
	}

	// Defensive copy verification
	subs := parent.Submenu()
	subs[0] = nil
	if parent.Submenu()[0] == nil {
		t.Fatal("parent.Submenu() must return a defensive copy")
	}

	// Real App mounting test to verify distinct NodeIDs
	tb := tui.NewTestBackend(80, 24)
	ctx, cancel := context.WithCancel(context.Background())
	app := tui.NewApp(parent, tui.WithBackend(tb))
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

	var parentID, sub1ID, sub2ID tui.NodeID
	doneUpdate := make(chan struct{})
	app.Update(func() {
		parentID = parent.NodeID()
		sub1ID = sub1.NodeID()
		sub2ID = sub2.NodeID()
		close(doneUpdate)
	})
	select {
	case <-doneUpdate:
	case <-time.After(3 * time.Second):
		t.Fatal("app.Update did not execute within 3s")
	}

	if parentID == 0 || sub1ID == 0 || sub2ID == 0 {
		t.Fatalf("expected non-zero NodeIDs for all mounted items: parent=%d, sub1=%d, sub2=%d", parentID, sub1ID, sub2ID)
	}
	if sub1ID == parentID || sub2ID == parentID {
		t.Fatalf("child submenu items must not reuse parent NodeID %d; got sub1=%d, sub2=%d", parentID, sub1ID, sub2ID)
	}
	if sub1ID == sub2ID {
		t.Fatalf("submenu items must each have distinct NodeIDs; got sub1=%d, sub2=%d", sub1ID, sub2ID)
	}

	// Rendering parent with submenu arrow ►
	surf := newMockSurface(20, 3)
	parent.RenderAt(surf, 1, 1, 18)
	screen := surf.String()
	if !strings.Contains(screen, "Keymaps") {
		t.Fatalf("rendered parent missing Name; screen:\n%s", screen)
	}
	if !strings.Contains(screen, "►") {
		t.Fatalf("rendered parent missing submenu arrow ►; screen:\n%s", screen)
	}

	// Rendering checkable child with bullet •
	subSurf := newMockSurface(24, 3)
	sub1.RenderAt(subSurf, 1, 1, 22)
	subScreen := subSurf.String()
	if !strings.Contains(subScreen, "•") {
		t.Fatalf("rendered checkable child missing bullet •; screen:\n%s", subScreen)
	}
	if !strings.Contains(subScreen, "1. Vim") {
		t.Fatalf("rendered checkable child missing text; screen:\n%s", subScreen)
	}
}

func TestMenuItem_UnicodeDisplayWidth(t *testing.T) {
	cjkItem := NewMenuItem("文件", 'f', 0, nil)
	// "文件" = 4 cells + 4 = 8
	if w := cjkItem.Width(); w != 8 {
		t.Fatalf("Width() for CJK item = %d, want 8", w)
	}
}
