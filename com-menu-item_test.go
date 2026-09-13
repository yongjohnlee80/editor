package editor

import (
	"strings"
	"testing"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
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

func TestMenuItem_SubmenuAndKeyset(t *testing.T) {
	var vimTriggered bool
	sub1 := NewMenuItemWithKeyset("1. Vim  (modal)", '1', 0, widget.KeysetVim, func() {
		vimTriggered = true
	})
	sub2 := NewMenuItemWithKeyset("2. Nano (modeless)", '2', 0, widget.KeysetNano, nil)

	parent := NewMenuItemWithSubmenu("Keymaps", 'k', 0, sub1, sub2)

	if !parent.HasSubmenu() {
		t.Fatal("parent.HasSubmenu() should return true")
	}
	if len(parent.Submenu) != 2 {
		t.Fatalf("len(parent.Submenu) = %d, want 2", len(parent.Submenu))
	}
	if !sub1.Checkable() {
		t.Fatal("sub1 should be checkable")
	}
	if sub1.Keyset != widget.KeysetVim {
		t.Fatalf("sub1.Keyset = %v, want KeysetVim", sub1.Keyset)
	}

	sub1.Trigger()
	if !vimTriggered {
		t.Fatal("sub1.Trigger() did not invoke action")
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
	sub1.SetChecked(true)
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
