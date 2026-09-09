package editor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yongjohnlee80/golib/errs"
	"github.com/yongjohnlee80/golib/tui"
)

// This file implements the OS-level ex commands: reading a file into the
// buffer (":e", ":edit") and writing the buffer to disk (":w", ":write").
//
// # Layered design
//
// Each command is split into two layers:
//
//  1. A base function that works with io.Reader / io.Writer — no paths, no
//     os.* calls, fully testable without touching the file system.
//  2. A Command[R] constructor that handles path resolution, directory
//     creation, and the "no name" prompt case, then delegates I/O to the
//     base function.
//
// # Error handling
//
// Neither the base functions nor the Command constructors return a Go error.
// All failure outcomes are embedded in the response via Refuse[string](err),
// following the command layer's convention: callers switch on Status,
// they never check a second return value.
//
// Document interface is declared in document.go.

// InitOSCommands registers the standard OS file commands (:w, :write, :e, :edit)
// with the provided registry for the target document.
//
// This serves as the public registration entrypoint for all OS-level commands,
// allowing callers like App to mount the full set of file operations in one call.
func InitOSCommands(reg *Registry, doc Document) {
	reg.Add(Register(NewWriteFileCmd(doc), "write buffer to disk"), "w", "write")
	reg.Add(Register(NewReadFileCmd(doc), "edit (open) file from disk"), "e", "edit")
}

// ─── NewReadFileCmd ───────────────────────────────────────────────────────────

// NewReadFileCmd returns a Command[string] that reads a file from disk into
// the document, replacing its current content.
//
//   - arg is the path from the command line (e.g. ":e path/to/file").
//   - When arg is empty, the document's current Path() is used.
//   - When both are empty, StatusRefused is returned — nothing to open.
//
// On StatusOK, Result() is a vim-style summary ("path" NL lines) and the
// document's path is updated to the loaded file.
//
//	reg.Add(Register(NewReadFileCmd(doc), "edit (open) a file"), "e", "edit")
func NewReadFileCmd(doc Document) Command[string] {
	return func(_ *tui.Context, arg string) CommandResponse[string] {
		path := arg
		if path == "" {
			path = doc.Path()
		}
		if path == "" {
			return Refuse[string](errs.Wrap(errs.ErrInvalidArgument, "no file name"))
		}

		f, err := os.Open(path)
		if err != nil {
			return Refuse[string](errs.WrapCause(errs.ErrInvalidArgument, err,
				"opening %s", path))
		}
		defer f.Close()

		lines, err := readInto(doc, f)
		if err != nil {
			return Refuse[string](errs.WrapCause(errs.ErrInvalidArgument, err,
				"reading %s", path))
		}

		// Adopt the path so a later bare ":w" writes back to the same file.
		doc.SetPath(path)
		return Ok(fmt.Sprintf("%q %dL", path, lines))
	}
}

// ─── NewWriteFileCmd ──────────────────────────────────────────────────────────

// NewWriteFileCmd returns a Command[string] that writes the document's
// current text to disk.
//
//   - arg is the path from the command line (e.g. ":w path/to/file").
//   - When arg is empty, the document's current Path() is used.
//   - When both are empty, StatusPromptNeeded is returned with "w " as the
//     prefill — App reopens the command line so the user can type a name.
//
// Parent directories are created automatically (os.MkdirAll).
//
// On StatusOK, Result() is a vim-style summary ("path" NL written) and the
// document's path is updated and its dirty flag cleared.
//
//	reg.Add(Register(NewWriteFileCmd(doc), "write the buffer to disk"), "w", "write")
func NewWriteFileCmd(doc Document) Command[string] {
	return func(_ *tui.Context, arg string) CommandResponse[string] {
		path := arg
		if path == "" {
			path = doc.Path()
		}
		if path == "" {
			// No name anywhere: ask the user. The "w " prefill seeds the
			// command line so the user only needs to type the filename.
			return Prompt("w ")
		}

		// Create parent directories so ":w notes/2026/adr.md" works without
		// the user running mkdir first.
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return Refuse[string](errs.WrapCause(errs.ErrInvalidArgument, err,
					"creating %s", dir))
			}
		}

		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return Refuse[string](errs.WrapCause(errs.ErrInvalidArgument, err,
				"opening %s for write", path))
		}
		defer f.Close()

		lines, err := writeFrom(doc, f)
		if err != nil {
			return Refuse[string](errs.WrapCause(errs.ErrInvalidArgument, err,
				"writing %s", path))
		}

		// Adopt the path and clear dirty so ":w" and ":q" both behave
		// correctly on the next invocation.
		doc.SetPath(path)
		doc.MarkClean()
		return Ok(fmt.Sprintf("%q %dL written", path, lines))
	}
}

// ─── Base I/O functions ───────────────────────────────────────────────────────

// readInto reads all bytes from r and sets them as the document's content,
// then clears the dirty flag. It is the pure-I/O core of NewReadFileCmd:
// no paths, no os.Open, and no Document side-effects beyond SetValue and
// MarkClean. Returns the number of lines loaded.
func readInto(doc Document, r io.Reader) (lines int, err error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	doc.SetValue(string(b))
	doc.MarkClean()
	if len(b) > 0 {
		lines = strings.Count(string(b), "\n")
	}
	return lines, nil
}

// writeFrom writes the document's current text to w, appending a trailing
// newline if the buffer does not already end with one (POSIX text-file
// convention). It is the pure-I/O core of NewWriteFileCmd. Returns the
// number of lines written.
func writeFrom(doc Document, w io.Writer) (lines int, err error) {
	body := doc.Value()
	// A POSIX text file ends with a newline. Every tool that reads this file
	// expects one; appending here so callers never have to remember.
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if _, err := fmt.Fprint(w, body); err != nil {
		return 0, err
	}
	if body != "" {
		lines = strings.Count(body, "\n")
	}
	return lines, nil
}
