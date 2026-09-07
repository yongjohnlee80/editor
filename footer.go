package editor

import (
	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// commandPrompt is displayed on the left side of the command line cursor.
const commandPrompt = "COMMAND: "

// commandPromptStyle styles the command prompt with a blue background and white text.
var commandPromptStyle = style.New().
	Background(style.ANSI(6)).
	Foreground(style.ANSI(0))

// footer shows the status bar, or the command line while one is being typed.
//
// All children stay MOUNTED and only the active mode is laid out, rather than mounting and
// unmounting on every ":". Remounting would give the input a new NodeID each
// time, which invalidates the SubmitEvent subscription that Init set up once.
type footer struct {
	ctx        *tui.Context
	status     *widget.StatusBar
	prompt     *widget.Text
	input      *widget.TextInput
	commanding bool
}

func (f *footer) Init(ctx *tui.Context) {
	f.ctx = ctx
	ctx.Mount(f.status)
	if f.prompt == nil {
		f.prompt = widget.NewText(commandPrompt, widget.WithTextStyle(commandPromptStyle))
	}
	ctx.Mount(f.prompt)
	ctx.Mount(f.input)
}

func (f *footer) Layout(c tui.Constraints) tui.Size {
	if !f.commanding {
		sz := f.ctx.LayoutChild(f.status, c)
		f.ctx.PlaceChild(f.status, tui.Rect{X: 0, Y: 0, W: c.MaxW, H: sz.H})

		// The idle children are given zero height and unplaced so they do not paint.
		f.ctx.LayoutChild(f.prompt, tui.Constraints{})
		f.ctx.PlaceChild(f.prompt, tui.Rect{})
		f.ctx.LayoutChild(f.input, tui.Constraints{})
		f.ctx.PlaceChild(f.input, tui.Rect{})
		return c.Constrain(tui.Size{W: c.MaxW, H: sz.H})
	}

	f.ctx.LayoutChild(f.status, tui.Constraints{})
	f.ctx.PlaceChild(f.status, tui.Rect{})

	promptSz := f.ctx.LayoutChild(f.prompt, tui.Constraints{MaxW: c.MaxW, MaxH: 1})
	f.ctx.PlaceChild(f.prompt, tui.Rect{X: 0, Y: 0, W: promptSz.W, H: promptSz.H})

	inputW := max(0, c.MaxW-promptSz.W)
	inputSz := f.ctx.LayoutChild(f.input, tui.Constraints{MaxW: inputW, MaxH: 1})
	f.ctx.PlaceChild(f.input, tui.Rect{X: promptSz.W, Y: 0, W: inputW, H: inputSz.H})

	h := max(promptSz.H, inputSz.H)
	if h < 1 {
		h = 1
	}
	return c.Constrain(tui.Size{W: c.MaxW, H: h})
}

func (f *footer) Render(s tui.Surface) {
	if f.commanding {
		sz := s.Size()
		if sz.W > 0 && sz.H > 0 {
			s.Fill(tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H}, " ", style.Style{})
		}
	}
}

// HandleEvent lets everything through. The footer is a layout shell: its
// children handle their own keys, and the App owns the ":" that opens the
// command line.
func (f *footer) HandleEvent(tui.Event) bool { return false }
