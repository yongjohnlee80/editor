package editor

import (
	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// commandPrompt is the static label shown to the left of the command input
// when the user enters command mode by pressing ":". It gives the footer the
// familiar vim look: ":" appears at the left edge and the typed text follows.
const commandPrompt = "COMMAND: "

// commandPromptStyle renders the command prompt in reverse-video blue/white
// (ANSI colour 6 background, colour 0 foreground) so it stands out visually
// from the plain text of the command input to its right.
var commandPromptStyle = style.New().
	Background(style.ANSI(6)).
	Foreground(style.ANSI(0))

// footer is the single-row strip pinned to the bottom of the Dock. It renders
// in one of two visual modes, toggled by the commanding flag:
//
// Normal mode — shows the three-segment status bar:
//
//	┌─────────────────────────────────────────────────────┐
//	│ NORMAL          path/to/file              14:22     │  ← status
//	└─────────────────────────────────────────────────────┘
//
// Command mode — replaces the status bar with a prompt + text input:
//
//	┌─────────────────────────────────────────────────────┐
//	│ COMMAND: █                                          │  ← prompt + input
//	└─────────────────────────────────────────────────────┘
//
// # Always-mounted design
//
// All three child widgets (status, prompt, input) are mounted once in Init
// and remain mounted for the lifetime of the footer, regardless of which
// mode is active. When a widget is not in use its Layout constraints are set
// to zero (tui.Constraints{}) and its placement rect is also zero (tui.Rect{}).
// A zero-sized rect tells the framework not to paint the widget, so it
// effectively disappears without being unmounted.
//
// This avoids the alternative of unmounting/remounting children on every ":"
// press. Unmounting assigns a new NodeID on the next mount, which would
// invalidate the SubmitEvent subscription that App.Init wired once to the
// original NodeID of the TextInput — causing submitted commands to be silently
// dropped.
//
// # Why App.Init owns the SubmitEvent subscription (not footer.Init)
//
// footer.Init *could* subscribe to events using its own ctx — the API is
// identical: tui.SubscribeScoped(ctx, func(ev SomeEvent) { … }). Any
// component that has a *tui.Context in hand can call SubscribeScoped and
// the subscription will be torn down automatically when that component is
// unmounted.
//
// The SubmitEvent subscription deliberately lives in App.Init instead because
// the handler needs to call a.runCommand(ev.Value), which is App-level logic
// (command parsing, file writes, quit). Putting that handler in footer.Init
// would require passing a callback into footer at construction time, or giving
// footer a reference back to App — both of which tangle the dependency graph
// unnecessarily. Keeping the subscription in App.Init keeps footer a pure
// layout shell with no knowledge of command semantics.
type footer struct {
	ctx    *tui.Context
	status *widget.StatusBar
	prompt *widget.Text
	input  *widget.TextInput

	// commanding is true while the user is typing an ex command (":w", ":q",
	// etc.). App.openCommand flips it to true and App.closeCommand flips it
	// back. Layout reads this flag on every pass to decide which widgets to
	// show and which to suppress.
	commanding bool
}

// Init mounts all three child widgets into the TUI context and retains ctx
// for use in Layout.
//
// # Lifecycle
//
// Init is called exactly once by the TUI framework when the footer is first
// mounted as a child of the Dock (via ctx.Mount in App.Init). It runs before
// any Layout, Render, or HandleEvent call. The received ctx is the footer's
// own component context — distinct from the App's ctx — and is valid for the
// footer's entire mounted lifetime.
//
// Call chain: tui.NewApp.Run → mount(nil, root) → App.Init → ctx.Mount(host)
// → … → ctx.Mount(footer) → footer.Init.
func (f *footer) Init(ctx *tui.Context) {
	f.ctx = ctx

	// Mount all children unconditionally. The framework assigns each a stable
	// NodeID here; that ID must not change for the lifetime of the footer or
	// event subscriptions break (see type-level doc above).
	ctx.Mount(f.status)
	if f.prompt == nil {
		// Guard: New() normally passes a pre-built prompt, but if footer is
		// constructed without one (e.g. in tests) build a default here.
		f.prompt = widget.NewText(commandPrompt, widget.WithTextStyle(commandPromptStyle))
	}
	ctx.Mount(f.prompt)
	ctx.Mount(f.input)
}

// Layout computes the footer's geometry for the current frame, sizes and
// positions its visible children, and returns the footer's own size to the
// parent (Dock).
//
// # Lifecycle
//
// Layout is called by the TUI framework on every layout pass — at startup,
// whenever a component calls ctx.RequestLayout(), and after any terminal
// resize event. The framework calls it top-down: Dock.Layout → footer.Layout.
// After Layout returns, the framework calls Render on the same component tree
// bottom-up (leaves first), so the sizes computed here are already committed
// when Render runs.
//
// c (tui.Constraints) carries the maximum width and height the Dock is
// offering the footer. Because the footer is pinned to DockBottom, the Dock
// gives it the full terminal width and at most one row of height (MaxH=1 in
// practice).
//
// # Normal mode layout
//
// The status bar fills the full width; prompt and input get zero size and an
// empty rect so they don't paint:
//
//	X=0                                             X=MaxW
//	┌─────────────────────────────────────────────────┐
//	│ status (W=MaxW, H=sz.H)                         │
//	└─────────────────────────────────────────────────┘
//	  prompt (W=0, H=0)  input (W=0, H=0)  ← invisible
//
// # Command mode layout
//
// The status bar is suppressed and the remaining width is split between the
// fixed-width prompt label and a stretching text input:
//
//	X=0        X=promptSz.W                        X=MaxW
//	┌──────────┬──────────────────────────────────────┐
//	│ prompt   │ input (W = MaxW − promptSz.W)        │
//	│ (fixed)  │                                      │
//	└──────────┴──────────────────────────────────────┘
//	  status (W=0, H=0)  ← invisible
func (f *footer) Layout(c tui.Constraints) tui.Size {
	if !f.commanding {
		// ── Normal mode ──────────────────────────────────────────────────────
		//
		// Step 1: measure the status bar at the full available width.
		sz := f.ctx.LayoutChild(f.status, c)

		// Step 2: place the status bar flush to the top-left corner, spanning
		// the full width. The height comes from the status bar's own preferred
		// height (typically 1 row).
		//
		// W is c.MaxW (the full terminal width) rather than sz.W (the width
		// the status bar asked for). The distinction matters because
		// LayoutChild returns the size the widget *wants*, which may be
		// narrower than the available space if, for example, the status bar
		// has short content. PlaceChild's rect is the *drawing surface* given
		// to the widget — if we passed sz.W here, the renderer would clip the
		// status bar to only that many columns and leave the remainder of the
		// footer row unpainted. Those columns would then show whatever was
		// drawn there in the previous frame (ghost pixels from the old status
		// text, or from a wider editor window before a resize). Using c.MaxW
		// forces the status bar to fill the entire row, which StatusBar
		// handles by stretching its rightmost segment to cover the gap.
		//
		// General rule — LayoutChild vs PlaceChild are separate concerns:
		//   LayoutChild → "measure the child" (what does it want?)
		//   PlaceChild  → "allocate its drawing surface" (what do we give it?)
		// For any full-width strip (status bar, footer, header) the surface
		// should always be c.MaxW regardless of what the child measured,
		// so that no column on that row is left unpainted.
		f.ctx.PlaceChild(f.status, tui.Rect{X: 0, Y: 0, W: c.MaxW, H: sz.H})

		// Step 3: give prompt and input zero constraints and a zero rect.
		// LayoutChild with an empty Constraints{} lets the widget know it has
		// no space; PlaceChild with an empty Rect{} tells the renderer to skip
		// it entirely. Both calls are required on every frame — omitting either
		// leaves stale geometry from a previous frame visible on screen.
		f.ctx.LayoutChild(f.prompt, tui.Constraints{})
		f.ctx.PlaceChild(f.prompt, tui.Rect{})
		f.ctx.LayoutChild(f.input, tui.Constraints{})
		f.ctx.PlaceChild(f.input, tui.Rect{})

		return c.Constrain(tui.Size{W: c.MaxW, H: sz.H})
	}

	// ── Command mode ─────────────────────────────────────────────────────────
	//
	// Step 1: suppress the status bar with zero constraints and a zero rect
	// (same reasoning as the idle children in normal mode above).
	f.ctx.LayoutChild(f.status, tui.Constraints{})
	f.ctx.PlaceChild(f.status, tui.Rect{})

	// Step 2: measure the prompt label. MaxH=1 keeps it to a single row.
	// The prompt's width is dictated by the length of the commandPrompt
	// constant ("COMMAND: "), so it will always be narrow and fixed.
	promptSz := f.ctx.LayoutChild(f.prompt, tui.Constraints{MaxW: c.MaxW, MaxH: 1})

	// Step 3: place the prompt flush to the left edge (X=0, Y=0).
	f.ctx.PlaceChild(f.prompt, tui.Rect{X: 0, Y: 0, W: promptSz.W, H: promptSz.H})

	// Step 4: the input occupies whatever width remains after the prompt.
	// max(0, …) guards against a degenerate terminal that is narrower than
	// the prompt itself, which would produce a negative width and panic.
	inputW := max(0, c.MaxW-promptSz.W)
	inputSz := f.ctx.LayoutChild(f.input, tui.Constraints{MaxW: inputW, MaxH: 1})

	// Step 5: place the input immediately to the right of the prompt (X=promptSz.W).
	f.ctx.PlaceChild(f.input, tui.Rect{X: promptSz.W, Y: 0, W: inputW, H: inputSz.H})

	// Step 6: return the footer's own height as the taller of the two widgets
	// (they should both be 1, but we clamp to at least 1 to avoid reporting
	// a zero-height row, which would cause the Dock to collapse the footer).
	h := max(promptSz.H, inputSz.H)
	if h < 1 {
		h = 1
	}
	return c.Constrain(tui.Size{W: c.MaxW, H: h})
}

// Render paints the footer's own background, then lets the framework paint the
// mounted children on top.
//
// # Lifecycle
//
// Render is called by the TUI framework after Layout has completed for the
// entire tree. It is called bottom-up: children render before parents so that
// parents can paint over children when needed. The surface s is pre-clipped to
// the rect that PlaceChild assigned to the footer, so all coordinates here are
// footer-local (origin at 0, 0).
//
// In normal mode Render does nothing: the status bar (a child widget) paints
// its own background and content, so there is no gap to fill.
//
// In command mode Render fills the footer row with a blank slate before the
// prompt and input children paint. This ensures any leftover status-bar pixels
// from the previous frame are erased — particularly important when the terminal
// does not support full background-colour clearing.
func (f *footer) Render(s tui.Surface) {
	if f.commanding {
		sz := s.Size()
		if sz.W > 0 && sz.H > 0 {
			// Flood-fill the footer's bounding rect with a plain space in the
			// default style. This clears any residual pixels from the status
			// bar that was visible in the previous frame, giving the prompt
			// and input a clean surface to paint on top of.
			s.Fill(tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H}, " ", style.Style{})
		}
	}
}

// HandleEvent lets all events bubble up to the parent (App). The footer is a
// pure layout shell: its children (StatusBar, Text, TextInput) handle their
// own input, and App.handleKey owns the ":" keystroke that opens the command
// line. Returning false here ensures the framework continues the event-bubbling
// walk rather than stopping at the footer.
func (f *footer) HandleEvent(tui.Event) bool { return false }
