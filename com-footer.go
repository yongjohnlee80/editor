package editor

import (
	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// Footer is the single-row strip pinned to the bottom of the Dock. It renders
// the three-segment status bar:
//
//	┌─────────────────────────────────────────────────────┐
//	│ NORMAL          path/to/file              14:22     │  ← status
//	└─────────────────────────────────────────────────────┘
//
// Its sole responsibility is statistics reporting (mode, path/transient message,
// time). Command line input is hosted separately by EditorPane.
type Footer struct {
	// ctx is the mounted framework context retained from Init. Valid for the
	// component's lifetime; used to lay out and place the child widget.
	ctx *tui.Context

	// status is the three-segment status bar that lives in the footer:
	// mode (NORMAL / INSERT) on the left, file path or transient message in
	// the centre, and a wall clock on the right.
	status *widget.StatusBar
}

// newFooter constructs the Footer and its child StatusBar widget.
func newFooter() *Footer {
	return &Footer{
		status: widget.NewStatusBar(),
	}
}

// Init mounts the status bar child widget into the TUI context and retains ctx
// for use in Layout.
func (f *Footer) Init(ctx *tui.Context) {
	f.ctx = ctx
	ctx.Mount(f.status)
}

// Layout computes the footer's geometry for the current frame, sizing and
// positioning the status bar child, and returning the footer's size to the parent (Dock).
func (f *Footer) Layout(c tui.Constraints) tui.Size {
	sz := f.ctx.LayoutChild(f.status, c)
	f.ctx.PlaceChild(f.status, tui.Rect{X: 0, Y: 0, W: c.MaxW, H: sz.H})
	return c.Constrain(tui.Size{W: c.MaxW, H: sz.H})
}

// Render is a no-op because Footer is a container component whose visual output is
// produced by the mounted StatusBar child.
func (f *Footer) Render(tui.Surface) {}

// HandleEvent returns false because Footer does not handle keyboard input.
// Key events and command line input are handled locally within EditorPane.
func (f *Footer) HandleEvent(tui.Event) bool {
	return false
}

// SetStatus updates the three segments of the status bar (mode on the left,
// center text, and time on the right).
func (f *Footer) SetStatus(left, center, right string) {
	f.status.SetLeft(left)
	f.status.SetCenter(center)
	f.status.SetRight(right)
}
