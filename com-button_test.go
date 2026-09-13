package editor

import (
	"strings"
	"testing"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
)

type mockSurface struct {
	w, h  int
	cells map[[2]int]string
}

func newMockSurface(w, h int) *mockSurface {
	return &mockSurface{w: w, h: h, cells: make(map[[2]int]string)}
}

func (m *mockSurface) SetCell(x, y int, content string, st style.Style) {
	if x >= 0 && x < m.w && y >= 0 && y < m.h {
		m.cells[[2]int{x, y}] = content
	}
}

func (m *mockSurface) Fill(r tui.Rect, content string, st style.Style) {
	for y := r.Y; y < r.Y+r.H && y < m.h; y++ {
		for x := r.X; x < r.X+r.W && x < m.w; x++ {
			m.SetCell(x, y, content, st)
		}
	}
}

func (m *mockSurface) Sub(r tui.Rect) tui.Surface { return m }
func (m *mockSurface) Size() tui.Size             { return tui.Size{W: m.w, H: m.h} }
func (m *mockSurface) StringWidth(s string) int   { return len(s) }
func (m *mockSurface) Theme() *style.Theme        { return nil }
func (m *mockSurface) Caps() tui.Capabilities     { return tui.Capabilities{} }

func (m *mockSurface) String() string {
	var sb strings.Builder
	for y := 0; y < m.h; y++ {
		for x := 0; x < m.w; x++ {
			if c, ok := m.cells[[2]int{x, y}]; ok {
				sb.WriteString(c)
			} else {
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func TestButton_StandaloneWidget(t *testing.T) {
	var clicked bool
	btn := NewButton("Confirm", func() {
		clicked = true
	})

	if btn.Label() != "Confirm" {
		t.Fatalf("Label() = %q, want %q", btn.Label(), "Confirm")
	}
	if btn.FormattedLabel() != "[ Confirm ]" {
		t.Fatalf("FormattedLabel() = %q, want %q", btn.FormattedLabel(), "[ Confirm ]")
	}
	if btn.Width() != 11 {
		t.Fatalf("Width() = %d, want 11", btn.Width())
	}
	if !btn.AcceptsFocus() {
		t.Fatal("AcceptsFocus() should return true")
	}

	btn.SetLabel("Save")
	if btn.Label() != "Save" {
		t.Fatalf("after SetLabel: Label() = %q, want %q", btn.Label(), "Save")
	}

	// Trigger callback directly
	btn.Trigger()
	if !clicked {
		t.Fatal("Trigger() did not invoke callback action")
	}

	// Test SetAction
	var altClicked bool
	btn.SetAction(func() { altClicked = true })
	btn.Trigger()
	if !altClicked {
		t.Fatal("Trigger() did not invoke updated callback action")
	}

	// HandleEvent when unfocused -> returns false, does not click
	altClicked = false
	handled := btn.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if handled || altClicked {
		t.Fatal("unfocused button must not handle KeyEnter or invoke callback")
	}

	// Focus button -> handles KeyEnter and Space
	btn.SetFocused(true)
	if !btn.Focused() {
		t.Fatal("Focused() should return true after SetFocused(true)")
	}
	handled = btn.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if !handled || !altClicked {
		t.Fatal("focused button must handle KeyEnter and invoke callback")
	}

	altClicked = false
	handled = btn.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: ' '})
	if !handled || !altClicked {
		t.Fatal("focused button must handle Space and invoke callback")
	}

	// Non-activation keys should not be handled
	handled = btn.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: 'x'})
	if handled {
		t.Fatal("focused button should not handle unrelated key 'x'")
	}

	// Layout constraints
	sz := btn.Layout(tui.Constraints{MinW: 0, MaxW: 80, MinH: 0, MaxH: 24})
	if sz.W != btn.Width() || sz.H != 1 {
		t.Fatalf("Layout() = %+v, want (%d, 1)", sz, btn.Width())
	}

	// Custom styles
	customNorm := style.New().Foreground(style.ANSI(2))
	customFoc := style.New().Foreground(style.ANSI(3))
	btn.SetStyles(customNorm, customFoc)
	if btn.ButtonStyle(false) != customNorm {
		t.Fatal("ButtonStyle(false) did not match custom normal style")
	}
	if btn.ButtonStyle(true) != customFoc {
		t.Fatal("ButtonStyle(true) did not match custom focused style")
	}

	// Render onto surface
	surf := newMockSurface(20, 3)
	btn.RenderAt(surf, 2, 1)
	rendered := surf.String()
	if !strings.Contains(rendered, "[ Save ]") {
		t.Fatalf("RenderAt did not paint formatted label; screen:\n%s", rendered)
	}

	// Render default origin
	surf2 := newMockSurface(20, 3)
	btn.Render(surf2)
	rendered2 := surf2.String()
	if !strings.Contains(rendered2, "[ Save ]") {
		t.Fatalf("Render did not paint formatted label at (0,0); screen:\n%s", rendered2)
	}
}

func TestButtonStyle_Standalone(t *testing.T) {
	// Nil receiver tests fallback to defaultButtonStyle
	var nilStyle *ButtonStyle
	if nilStyle.Normal() != defaultButtonStyle.normal {
		t.Fatal("nilStyle.Normal() did not return defaultButtonStyle.normal")
	}
	if nilStyle.Focused() != defaultButtonStyle.focused {
		t.Fatal("nilStyle.Focused() did not return defaultButtonStyle.focused")
	}
	if nilStyle.Style(false) != defaultButtonStyle.normal {
		t.Fatal("nilStyle.Style(false) did not return defaultButtonStyle.normal")
	}
	if nilStyle.Style(true) != defaultButtonStyle.focused {
		t.Fatal("nilStyle.Style(true) did not return defaultButtonStyle.focused")
	}

	// Custom ButtonStyle
	norm := style.New().Foreground(style.ANSI(1))
	foc := style.New().Foreground(style.ANSI(2))
	bs := NewButtonStyle(norm, foc)
	if bs.Normal() != norm {
		t.Fatal("bs.Normal() did not return custom norm")
	}
	if bs.Focused() != foc {
		t.Fatal("bs.Focused() did not return custom foc")
	}
	if bs.Style(false) != norm {
		t.Fatal("bs.Style(false) did not return custom norm")
	}
	if bs.Style(true) != foc {
		t.Fatal("bs.Style(true) did not return custom foc")
	}
}
