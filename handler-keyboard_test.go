package editor

import (
	"testing"

	"github.com/yongjohnlee80/golib/tui"
)

func TestInputScope_Values(t *testing.T) {
	if ScopeUnknown != 0 {
		t.Errorf("ScopeUnknown = %d, want 0 (reserved as unknown)", ScopeUnknown)
	}
	if ScopeEditorNormal == ScopeUnknown || ScopeCommandLine == ScopeUnknown {
		t.Error("valid scopes must not equal ScopeUnknown")
	}
}

func TestDefaultKeyResolver_ScopeEditorNormal(t *testing.T) {
	res := NewDefaultKeyResolver(" ")

	// ":" in Normal mode resolves to ActionOpenCommandLine
	act, ok := res.Resolve(ScopeEditorNormal, tui.KeyEvent{
		Kind: tui.KeyPress,
		Text: ":",
	})
	if !ok || act != ActionOpenCommandLine {
		t.Errorf("got (%v, %v), want (ActionOpenCommandLine, true)", act, ok)
	}

	// Space (leader) in Normal mode resolves to ActionOpenCommandLine
	act, ok = res.Resolve(ScopeEditorNormal, tui.KeyEvent{
		Kind: tui.KeyPress,
		Text: " ",
	})
	if !ok || act != ActionOpenCommandLine {
		t.Errorf("got (%v, %v), want (ActionOpenCommandLine, true)", act, ok)
	}

	// KeyRelease is ignored
	act, ok = res.Resolve(ScopeEditorNormal, tui.KeyEvent{
		Kind: tui.KeyRelease,
		Text: ":",
	})
	if ok || act != ActionNone {
		t.Errorf("got (%v, %v), want (ActionNone, false) on release", act, ok)
	}

	// Super-modified key is ignored
	act, ok = res.Resolve(ScopeEditorNormal, tui.KeyEvent{
		Kind: tui.KeyPress,
		Text: ":",
		Mods: tui.ModSuper,
	})
	if ok || act != ActionNone {
		t.Errorf("got (%v, %v), want (ActionNone, false) with Super mod", act, ok)
	}

	// Unrelated key is ignored
	act, ok = res.Resolve(ScopeEditorNormal, tui.KeyEvent{
		Kind: tui.KeyPress,
		Text: "x",
	})
	if ok || act != ActionNone {
		t.Errorf("got (%v, %v), want (ActionNone, false) for unrelated key", act, ok)
	}
}

func TestDefaultKeyResolver_ScopeCommandLine(t *testing.T) {
	res := NewDefaultKeyResolver(" ")

	// Escape in command line resolves to ActionCancelCommandLine
	act, ok := res.Resolve(ScopeCommandLine, tui.KeyEvent{
		Kind: tui.KeyPress,
		Code: tui.KeyEscape,
	})
	if !ok || act != ActionCancelCommandLine {
		t.Errorf("got (%v, %v), want (ActionCancelCommandLine, true)", act, ok)
	}

	// Normal text in command line is not an application action
	act, ok = res.Resolve(ScopeCommandLine, tui.KeyEvent{
		Kind: tui.KeyPress,
		Text: "w",
	})
	if ok || act != ActionNone {
		t.Errorf("got (%v, %v), want (ActionNone, false) for normal text in cmd line", act, ok)
	}
}

func TestValidateLeaderKey_Collisions(t *testing.T) {
	// Valid leader keys: space, comma, semicolon, hash
	for _, safe := range []string{" ", ",", ";", "#"} {
		if err := ValidateLeaderKey(safe); err != nil {
			t.Errorf("ValidateLeaderKey(%q) failed unexpectedly: %v", safe, err)
		}
	}

	// Conflicting leader keys (bound in vi Normal mode: h, j, k, l, i, a, g, d)
	for _, conflict := range []string{"h", "j", "k", "l", "i", "a", "g", "d", "y"} {
		err := ValidateLeaderKey(conflict)
		if err == nil {
			t.Errorf("ValidateLeaderKey(%q) succeeded, want collision error", conflict)
		}
	}

	// Multi-character or empty
	if err := ValidateLeaderKey(""); err == nil {
		t.Error("ValidateLeaderKey(\"\") must fail")
	}
	if err := ValidateLeaderKey("leader"); err == nil {
		t.Error("ValidateLeaderKey(\"leader\") must fail")
	}
}

func TestComponent_NilWiringRejection(t *testing.T) {
	cfg := DefaultConfig()

	// EditorPane rejects nil resolver or sink
	if _, err := newEditorPane(cfg, "", nil, func(KeyAction) {}); err == nil {
		t.Error("newEditorPane with nil resolver must return error")
	}
	if _, err := newEditorPane(cfg, "", NewDefaultKeyResolver(" "), nil); err == nil {
		t.Error("newEditorPane with nil sink must return error")
	}
}

func TestDefaultKeyResolver_F10_TogglesMenuBar(t *testing.T) {
	res := NewDefaultKeyResolver(" ")

	for _, scope := range []InputScope{ScopeEditorNormal, ScopeCommandLine} {
		act, ok := res.Resolve(scope, tui.KeyEvent{
			Kind: tui.KeyPress,
			Code: tui.KeyF10,
		})
		if !ok || act != ActionToggleMenuBar {
			t.Errorf("scope %v: got (%v, %v), want (ActionToggleMenuBar, true)", scope, act, ok)
		}
	}
}

func TestDefaultKeyResolver_Alt_TriggersMenu(t *testing.T) {
	res := NewDefaultKeyResolver(" ")

	cases := []struct {
		code rune
		want KeyAction
	}{
		{'f', ActionOpenMenuFile},
		{'F', ActionOpenMenuFile},
		{'o', ActionOpenMenuOption},
		{'O', ActionOpenMenuOption},
		{'h', ActionOpenMenuHelp},
		{'H', ActionOpenMenuHelp},
		{'m', ActionToggleMenuBar},
		{' ', ActionToggleMenuBar},
	}

	for _, scope := range []InputScope{ScopeEditorNormal, ScopeCommandLine} {
		for _, tc := range cases {
			act, ok := res.Resolve(scope, tui.KeyEvent{
				Kind: tui.KeyPress,
				Code: tc.code,
				Mods: tui.ModAlt,
			})
			if !ok || act != tc.want {
				t.Errorf("scope %v, key %c with Alt: got (%v, %v), want (%v, true)", scope, tc.code, act, ok, tc.want)
			}
		}
	}
}
