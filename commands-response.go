package editor

import "github.com/yongjohnlee80/golib/tui"

// This file contains the concrete implementations for the command layer:
//
//   - Response[R]         — the standard CommandResponse[R] implementation
//   - Ok, Prompt, Refuse  — constructors that build Response values by intent
//   - Handler.Run, .Help  — methods on the Handler type declared in commands.go
//   - Register[R]         — boxes a typed Command[R] into a type-erased Handler

// ─── Response[R] ─────────────────────────────────────────────────────────────

// Response is the standard implementation of CommandResponse[R]. Build it
// with Ok, Prompt, or Refuse rather than constructing it directly; the
// constructors encode intent and prevent accidental field miswiring.
type Response[R any] struct {
	status CommandStatus
	result R
	err    error
}

// Status implements CommandResponse.
func (r Response[R]) Status() CommandStatus { return r.status }

// Result implements CommandResponse.
func (r Response[R]) Result() R { return r.result }

// Err implements CommandResponse. Returns nil on a successful (StatusOK)
// response; the wrapped OS or policy error on any other status.
func (r Response[R]) Err() error { return r.err }

// ─── Constructors ─────────────────────────────────────────────────────────────

// Ok wraps result in a StatusOK response. Use this as the return value of a
// successful command.
//
//	return Ok("\"notes.md\" 14L written")
func Ok[R any](result R) Response[R] {
	return Response[R]{status: StatusOK, result: result}
}

// Prompt returns a StatusPromptNeeded response carrying prefill. The prefill
// is the string that App should seed into the command line input so the user
// only has to type the missing part (e.g. a filename after ":w ").
//
// Prompt is specifically typed as Response[string] because the prefill IS
// the result. It is returned by Command[string] implementations; commands
// with other result types use Refuse[R] or Ok[R] instead.
//
//	return Prompt("w ")   // reopens ":" pre-seeded with "w "
func Prompt(prefill string) Response[string] {
	return Response[string]{status: StatusPromptNeeded, result: prefill}
}

// Refuse returns a StatusRefused response with an explanatory error. The
// caller shows Err().Error() in the status bar; the command line stays open
// so the user can correct the command.
//
// Refuse is generic so it can be used by any Command[R], regardless of the
// result type — a refused command never has a meaningful result.
//
//	return Refuse[string](errors.New("unsaved changes — :q! to discard"))
func Refuse[R any](err error) Response[R] {
	var zero R
	return Response[R]{status: StatusRefused, result: zero, err: err}
}

// ─── Handler methods ──────────────────────────────────────────────────────────

// Run executes the handler and returns a type-erased response. Called by
// Registry.Dispatch after looking up the handler for a verb.
func (h Handler) Run(ctx *tui.Context, arg string) CommandResponse[any] {
	return h.run(ctx, arg)
}

// Help returns the one-line description registered with this handler.
// Intended for ":help" listings and command-palette overlays.
func (h Handler) Help() string { return h.help }

// ─── Register ─────────────────────────────────────────────────────────────────

// Register wraps a typed Command[R] in a Handler suitable for storage in the
// Registry. The inner closure adapts CommandResponse[R] → CommandResponse[any]
// (boxing the typed result into any) so that the registry map stays
// homogeneous without reflection or unsafe casts.
//
//	reg.Add(Register(NewWriteFileCmd(doc), "write the buffer to disk"), "w", "write")
func Register[R any](cmd Command[R], help string) Handler {
	return Handler{
		help: help,
		run: func(ctx *tui.Context, arg string) CommandResponse[any] {
			resp := cmd(ctx, arg)
			// Box the typed result into any. Status and Err pass through
			// unchanged; only the result value needs boxing.
			return Response[any]{
				status: resp.Status(),
				result: any(resp.Result()),
				err:    resp.Err(),
			}
		},
	}
}
