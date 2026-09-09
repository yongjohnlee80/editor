package editor

import (

	"github.com/yongjohnlee80/golib/errs"
	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// InputScope identifies the logical interaction mode for key resolution.
// ScopeUnknown is reserved as zero to ensure that an uninitialized or omitted
// scope value cannot accidentally be interpreted as ScopeEditorNormal.
type InputScope uint8

const (
	// ScopeUnknown represents an invalid or uninitialized input scope.
	ScopeUnknown InputScope = iota

	// ScopeEditorNormal applies when the editor buffer is in Normal mode.
	ScopeEditorNormal

	// ScopeCommandLine applies when the footer command line is active.
	ScopeCommandLine
)

// KeyAction represents an application-level semantic action triggered by
// keyboard input.
type KeyAction uint8

const (
	// ActionNone indicates no semantic action.
	ActionNone KeyAction = iota

	// ActionOpenCommandLine requests opening the ex command line input.
	ActionOpenCommandLine

	// ActionCancelCommandLine requests cancelling and closing the ex command line.
	ActionCancelCommandLine
)

// KeyResolver resolves physical key events into semantic actions within an input scope.
// It is a pure, stateless interface with no component references or focus ownership.
type KeyResolver interface {
	Resolve(scope InputScope, ev tui.KeyEvent) (KeyAction, bool)
}

// KeyActionSink is a synchronous callback for executing semantic key actions.
// App implements this to perform cross-component focus and visibility transitions.
type KeyActionSink func(KeyAction)

// DefaultKeyResolver is the standard KeyResolver implementation.
// It matches physical key events against configured application keybindings.
type DefaultKeyResolver struct {
	leaderKey string
}

// NewDefaultKeyResolver constructs a KeyResolver configured with the specified leader key.
func NewDefaultKeyResolver(leaderKey string) *DefaultKeyResolver {
	return &DefaultKeyResolver{
		leaderKey: leaderKey,
	}
}

// Resolve matches a key event under the provided scope and returns the corresponding
// KeyAction. Returns (ActionNone, false) if no binding matches or if the event is a release.
func (r *DefaultKeyResolver) Resolve(scope InputScope, ev tui.KeyEvent) (KeyAction, bool) {
	if ev.Kind == tui.KeyRelease {
		return ActionNone, false
	}

	// Key events with modifiers other than shift/ctrl are ignored by default
	// resolution so they can bubble to host or window-manager bindings.
	if ev.Mods&(tui.ModAlt|tui.ModSuper|tui.ModMeta|tui.ModHyper) != 0 {
		return ActionNone, false
	}

	switch scope {
	case ScopeEditorNormal:
		// In Normal mode: ":" or LeaderKey triggers opening the command line.
		// Note: widget.Editor handles vi movements and commands first;
		// unhandled keys bubble up to EditorPane, which invokes this resolver.
		if ev.Text == ":" {
			return ActionOpenCommandLine, true
		}
		if r.leaderKey != "" && ev.Text == r.leaderKey {
			return ActionOpenCommandLine, true
		}
	case ScopeCommandLine:
		// In Command mode: <Escape> cancels the command line.
		if ev.Code == tui.KeyEscape {
			return ActionCancelCommandLine, true
		}
	}

	return ActionNone, false
}

// ValidateLeaderKey checks if leaderKey collides with built-in vi Normal mode bindings.
// It queries widget.DefaultKeymap() directly to ensure alignment with the effective keymap.
func ValidateLeaderKey(leaderKey string) error {
	rs := []rune(leaderKey)
	if len(rs) != 1 {
		return errs.Wrap(errs.ErrInvalidArgument,
			"keyboard.LeaderKey = %q: want exactly one character, got %d",
			leaderKey, len(rs))
	}

	r := rs[0]
	km := widget.DefaultKeymap()
	chord := widget.KeyChord{
		Mode: widget.ModeNormal,
		Code: r,
		Ctrl: false,
	}

	if act, bound := km[chord]; bound && act != widget.ActUnbound {
		return errs.Wrap(errs.ErrInvalidArgument,
			"keyboard.LeaderKey = %q collides with built-in vi Normal mode binding (%v)",
			leaderKey, act)
	}

	return nil
}
