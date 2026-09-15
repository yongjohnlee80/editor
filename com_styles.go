package editor

// THE EDITOR'S LOOK, expressed in golib's style types.
//
// This replaces the two style files that belonged to the editor's own menu and
// modal widgets. The palette is unchanged — the Borland high-contrast scheme,
// white-on-black inverted for selection — but it now configures the upstream
// widgets instead of local ones.
//
// ONE DELIBERATE DIFFERENCE. The old menu drew a row's hotkey letter in red
// (menuAccentStyle); golib underlines it instead, from MenuItemModel.HotkeyIdx,
// and offers no per-letter colour. Underlining is the more conventional
// treatment and it is the widget's, so the red letter is gone rather than
// reproduced by some other means.

import (
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// defaultMenuStyle is the Borland bar: dark text on a light strip, inverted
// where the selection sits.
var defaultMenuStyle = widget.NewMenuStyle(
	// Surface: the bar and dropdown background.
	style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(0)),
	// Selected: inverted, which is how this scheme has always shown the cursor.
	style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(7)).
		Bold(true),
).
	// Armed is the pressed-but-not-yet-released look. The old widget had no such
	// state — it activated on press — so this is new, and it matches Selected
	// with the bold dropped so a press reads as a change without a jump.
	WithArmed(style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(7))).
	// Disabled rows were previously just skipped. Showing them greyed is better:
	// a command that exists but is unavailable tells the user more than one that
	// vanishes.
	WithDisabled(style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(8))).
	// Accel is the right-aligned shortcut text. Nothing sets one yet; styled now
	// so the first row that does is not unreadable.
	WithAccel(style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(8))).
	WithBorder(style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(0)).
		Border(style.BorderNormal))

// defaultModalStyle is the dialog card: a rounded high-contrast box with a cyan
// title over a stippled scrim.
var defaultModalStyle = widget.NewModalStyleFull(
	// Card.
	style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(7)),
	// Title.
	style.New().
		Foreground(style.ANSI(14)).
		Bold(true),
	// Border.
	style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(7)).
		Border(style.BorderRounded),
	// Scrim.
	style.New().
		Foreground(style.ANSI(8)).
		Faint(true),
)
