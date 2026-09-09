package editor

import (
	"fmt"
	"strings"

	"github.com/yongjohnlee80/golib/tui"
)

// Registry maps command verbs to their Handlers. It is the single place where
// the editor's command vocabulary is declared; App.runCommand dispatches
// through it rather than owning a switch statement.
//
// # Registration
//
// Commands are registered at startup via Add. Each Handler may share multiple
// verb aliases ("w" and "write", "q" and "quit"). Adding a verb a second
// time silently replaces the first — useful for overriding a built-in.
//
//	reg := NewRegistry()
//	reg.Add(Register(NewWriteFileCmd(doc), "write the buffer to disk"), "w", "write")
//	reg.Add(Register(quitCmd, "quit the editor"), "q", "quit")
//
// # Dispatch
//
// Dispatch parses the raw command line, looks up the verb, and calls its
// Handler. The caller (App.runCommand) switches on the returned Status to
// decide what to show in the status bar — no separate error to juggle.
//
// # Concurrency
//
// A Registry is not safe for concurrent use. All Add and Dispatch calls
// happen on the TUI event loop goroutine, so no locking is needed.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry returns an empty Registry ready to accept command registrations.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Add registers a Handler under one or more verb aliases.
//
// Multiple aliases for the same command are idiomatic — vim supports both
// ":w" and ":write", ":q" and ":quit". Providing them in a single Add call
// keeps the registration site readable.
//
//	reg.Add(Register(writeCmd, "write the buffer to disk"), "w", "write")
func (r *Registry) Add(h Handler, verbs ...string) {
	for _, v := range verbs {
		r.handlers[v] = h
	}
}

// Dispatch parses line, looks up the verb, and calls the matching Handler.
//
// line is the raw text from the command input, optionally prefixed with ":".
// Dispatch trims whitespace and the leading ":" before splitting on the first
// space to extract the verb and its trailing argument.
//
// # Dispatch table
//
//   - Empty line      → StatusOK, no side effects.
//   - Known verb      → the Handler's Run result is returned as-is.
//   - Unknown verb    → StatusUnknown with an Err naming the verb, consistent
//     with vim's "E492: Not an editor command".
//
// Dispatch always returns a non-nil CommandResponse. Callers can safely call
// resp.Status() without a nil check.
func (r *Registry) Dispatch(ctx *tui.Context, line string) CommandResponse[any] {
	cmd := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ":"))
	if cmd == "" {
		return Response[any]{status: StatusOK}
	}
	verb, arg, _ := strings.Cut(cmd, " ")
	arg = strings.TrimSpace(arg)

	h, ok := r.handlers[verb]
	if !ok {
		return Response[any]{
			status: StatusUnknown,
			err:    fmt.Errorf("not an editor command: %s", verb),
		}
	}
	return h.Run(ctx, arg)
}

// Verbs returns all registered verb strings in an unspecified order.
// Useful for building a ":help" listing or populating a command-palette
// overlay that shows every available command with its description.
func (r *Registry) Verbs() []string {
	out := make([]string, 0, len(r.handlers))
	for v := range r.handlers {
		out = append(out, v)
	}
	return out
}
