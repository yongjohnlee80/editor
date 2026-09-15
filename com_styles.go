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
	// Surface: the bar and dropdown background. BOLD, matching golib's own
	// default — menu labels are chrome sitting against the document, and
	// weight is what separates the two.
	style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(0)).
		Bold(true),
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

// defaultModalStyle is the dialog card: BLACK TEXT ON WHITE, the same inversion
// the menu bar uses, so the two read as one chrome rather than two themes.
//
// The body inherits the card's background rather than setting one of its own,
// which is what keeps the message from sitting in a differently-coloured patch
// inside the card.
var defaultModalStyle = widget.NewModalStyleFull(
	// Card, and therefore the body behind the text.
	style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(0)),
	// Title, on the frame's top rule. Bold rather than coloured: the position
	// already says it is the title.
	style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(0)).
		Bold(true),
	// Border.
	style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(0)).
		Border(style.BorderRounded),
	// Scrim over the content behind the dialog.
	style.New().
		Foreground(style.ANSI(8)).
		Faint(true),
)

// bodyStyle dresses a dialog's message to match the card it sits on. A Text
// with no style of its own paints on the terminal default, which shows as a
// rectangle of the wrong colour in the middle of the card.
var bodyStyle = style.New().
	Background(style.ANSI(7)).
	Foreground(style.ANSI(0))

// defaultButtonStyle dresses a dialog's controls to match the card they sit on.
//
// UNFOCUSED IS THE CARD'S OWN LOOK — black on white — so a button that is not
// the one the keyboard will act on reads as part of the dialog rather than as a
// second thing competing for attention. FOCUSED INVERTS IT, white on black,
// which is the same inversion the menu bar uses for its selection: one visual
// language for "this is where the keyboard is", in both places.
//
// Without this the buttons fall back to golib's token defaults, which resolve
// against the terminal's theme and rendered as a blue fill that belonged to
// neither the card nor the bar.
var defaultButtonStyle = widget.NewButtonStyleFull(
	// Normal: the card's own colours.
	style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(0)),
	// Focused: inverted, and bold so the brackets read as heavier too.
	style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(7)).
		Bold(true),
	// Disabled: still on the card, greyed rather than removed — a control that
	// exists but is unavailable says more than one that vanished.
	style.New().
		Background(style.ANSI(7)).
		Foreground(style.ANSI(8)),
	// Armed: pressed and not yet released. The focused look without the bold,
	// so a press reads as a change without the label jumping weight.
	style.New().
		Background(style.ANSI(0)).
		Foreground(style.ANSI(7)),
)
