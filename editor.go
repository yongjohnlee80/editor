package editor

import (
	"os"

	"github.com/yongjohnlee80/golib/errs"
	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// New builds the editor around an optional file. An empty path opens an unnamed
// buffer, exactly as "vim" with no argument does.
//
// A path that does not exist is NOT an error: it opens empty and ":w" creates
// it. A path that exists but cannot be read IS an error, because silently
// showing an empty buffer for a file that is there invites overwriting it.
func New(cfg Config, path string, quit func()) (*App, error) {
	a := &App{cfg: cfg, quit: quit, path: path}

	wrap := widget.WrapNone
	if cfg.Editor.HorizontalWrap {
		// Soft wrap has no horizontal extent, so the Editor also stops
		// drawing a horizontal scroll indicator — the hiding is a
		// consequence of the wrap, not a second setting.
		wrap = widget.WrapSoft
	}
	a.editor = widget.NewEditor(widget.WithEditorWrap(wrap))

	if path != "" {
		b, err := os.ReadFile(path)
		switch {
		case err == nil:
			a.editor.SetValue(string(b))
		case os.IsNotExist(err):
			// A new file. Nothing to load, and ":w" will create it.
		default:
			return nil, errs.WrapCause(errs.ErrInvalidArgument, err,
				"editor: opening %s", path)
		}
	}

	a.box = widget.NewBox(a.editor, widget.WithTitle(a.title()))
	a.status = widget.NewStatusBar()
	a.cmdPrompt = widget.NewText(commandPrompt)
	a.cmdIn = widget.NewTextInput()
	a.footer = &footer{status: a.status, prompt: a.cmdPrompt, input: a.cmdIn}

	dock := tui.NewDock()
	dock.Pin(tui.DockBottom, a.footer)
	dock.Add(a.box)
	a.host = widget.NewOverlayHost(dock)
	return a, nil
}
