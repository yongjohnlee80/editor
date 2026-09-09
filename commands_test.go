package editor

import (
	"errors"
	"testing"

	"github.com/yongjohnlee80/golib/tui"
)

// Commands tests exercise the command layer in pure isolation: no TUI loop,
// no App, no file system. The registry is constructed inline, handlers are
// stubs, and Dispatch is driven directly.

// ─── Status ──────────────────────────────────────────────────────────────────

func TestCommandStatus_Values(t *testing.T) {
	cases := []struct {
		s    CommandStatus
		want string
	}{
		{StatusOK, "OK"},
		{StatusPromptNeeded, "PromptNeeded"},
		{StatusRefused, "Refused"},
		{StatusUnknown, "Unknown"},
		{CommandStatus("Custom"), "Custom"},
	}
	for _, tc := range cases {
		if string(tc.s) != tc.want {
			t.Errorf("CommandStatus %q != %q", string(tc.s), tc.want)
		}
	}
}

// ─── Response constructors ───────────────────────────────────────────────────

func TestOk_SetsStatusAndResult(t *testing.T) {
	r := Ok("hello")
	if r.Status() != StatusOK {
		t.Errorf("Status = %v, want OK", r.Status())
	}
	if r.Result() != "hello" {
		t.Errorf("Result = %q, want %q", r.Result(), "hello")
	}
	if r.Err() != nil {
		t.Errorf("Err = %v, want nil", r.Err())
	}
}

func TestPrompt_SetsStatusAndPrefill(t *testing.T) {
	r := Prompt("w ")
	if r.Status() != StatusPromptNeeded {
		t.Errorf("Status = %v, want PromptNeeded", r.Status())
	}
	if r.Result() != "w " {
		t.Errorf("Result = %q, want %q", r.Result(), "w ")
	}
}

func TestRefuse_SetsStatusAndError(t *testing.T) {
	err := errors.New("unsaved changes")
	r := Refuse[string](err)
	if r.Status() != StatusRefused {
		t.Errorf("Status = %v, want Refused", r.Status())
	}
	if r.Err() != err {
		t.Errorf("Err = %v, want %v", r.Err(), err)
	}
}

// ─── Registry ────────────────────────────────────────────────────────────────

// echoCmd is a trivial Command[string] that echoes its argument back.
var echoCmd Command[string] = func(_ *tui.Context, arg string) CommandResponse[string] {
	return Ok("echo: " + arg)
}

func TestRegistry_DispatchKnownVerb(t *testing.T) {
	reg := NewRegistry()
	reg.Add(Register(echoCmd, "echo the argument"), "echo")

	resp := reg.Dispatch(nil, "echo hello")
	if resp.Status() != StatusOK {
		t.Errorf("Status = %v, want OK", resp.Status())
	}
}

func TestRegistry_DispatchUnknownVerbReturnsStatusUnknown(t *testing.T) {
	reg := NewRegistry()

	resp := reg.Dispatch(nil, "zzz")
	if resp.Status() != StatusUnknown {
		t.Errorf("Status = %v, want Unknown", resp.Status())
	}
	// The error message must name the rejected verb.
	if resp.Err() == nil {
		t.Error("Err must be non-nil for an unknown verb")
	}
}

func TestRegistry_DispatchEmptyLineIsOK(t *testing.T) {
	reg := NewRegistry()
	resp := reg.Dispatch(nil, "")
	if resp.Status() != StatusOK {
		t.Errorf("Status = %v, want OK for empty line", resp.Status())
	}
}

func TestRegistry_DispatchStripsLeadingColon(t *testing.T) {
	reg := NewRegistry()
	reg.Add(Register(echoCmd, "echo"), "echo")

	// ":echo world" should route the same as "echo world".
	resp := reg.Dispatch(nil, ":echo world")
	if resp.Status() != StatusOK {
		t.Errorf("Status = %v, want OK", resp.Status())
	}
}

func TestRegistry_AddAliasesShareHandler(t *testing.T) {
	reg := NewRegistry()
	// Register "quit" under both "q" and "quit".
	reg.Add(Register(echoCmd, "quit"), "q", "quit")

	for _, verb := range []string{"q", "quit"} {
		resp := reg.Dispatch(nil, verb)
		if resp.Status() != StatusOK {
			t.Errorf("Dispatch %q: Status = %v, want OK", verb, resp.Status())
		}
	}
}

func TestRegistry_VerbsListsRegisteredVerbs(t *testing.T) {
	reg := NewRegistry()
	reg.Add(Register(echoCmd, "echo"), "echo", "e")

	verbs := reg.Verbs()
	have := make(map[string]bool, len(verbs))
	for _, v := range verbs {
		have[v] = true
	}
	for _, want := range []string{"echo", "e"} {
		if !have[want] {
			t.Errorf("Verbs() missing %q; got %v", want, verbs)
		}
	}
}

func TestRegister_HelpIsAccessible(t *testing.T) {
	h := Register(echoCmd, "echo the argument back")
	if h.Help() != "echo the argument back" {
		t.Errorf("Help() = %q, want %q", h.Help(), "echo the argument back")
	}
}

// A command that signals StatusRefused is propagated correctly through the
// registry without being converted to a Go error.
func TestRegistry_DispatchRefusedCommand(t *testing.T) {
	refuseCmd := Command[struct{}](func(_ *tui.Context, _ string) CommandResponse[struct{}] {
		return Refuse[struct{}](errors.New("not allowed"))
	})
	reg := NewRegistry()
	reg.Add(Register(refuseCmd, "always refuses"), "no")

	resp := reg.Dispatch(nil, "no")
	if resp.Status() != StatusRefused {
		t.Errorf("Status = %v, want Refused", resp.Status())
	}
}

// A command that signals StatusPromptNeeded is propagated correctly.
func TestRegistry_DispatchPromptNeededCommand(t *testing.T) {
	promptCmd := Command[string](func(_ *tui.Context, _ string) CommandResponse[string] {
		return Prompt("w ")
	})
	reg := NewRegistry()
	reg.Add(Register(promptCmd, "needs a filename"), "save-as")

	resp := reg.Dispatch(nil, "save-as")
	if resp.Status() != StatusPromptNeeded {
		t.Errorf("Status = %v, want PromptNeeded", resp.Status())
	}
}
