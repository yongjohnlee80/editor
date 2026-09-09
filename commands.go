package editor

import "github.com/yongjohnlee80/golib/tui"

// This file contains the interface and type declarations for the command
// layer. Concrete implementations live in commands-response.go; the dispatch
// table lives in commands-registry.go; OS-level commands live in commands-os.go.
//
// # Design overview
//
// The three concerns that previously lived in App.runCommand are now separate:
//
//  1. Parsing  — splitting the raw command line into a verb and an argument.
//  2. Dispatch — looking up and executing the right handler for a verb.
//  3. Effect   — the side effects of a command (file I/O, quitting, etc.).
//
// # Error handling
//
// Command[R] has no separate error return. Every outcome — success, refusal,
// prompt request, unexpected OS failure — is expressed as a CommandResponse[R]
// with an appropriate CommandStatus and, when needed, an Err(). This keeps
// callers uniform: they always switch on Status, never on (response, error).
//
// # Type-erasure for the registry
//
// Commands are typed (Command[string], Command[struct{}], …) but the Registry
// must store handlers of different result types in the same map. Handler is
// the type-erased unit: Register[R] boxes any Command[R] into a Handler whose
// run closure returns CommandResponse[any]. Callers that need the concrete
// type back switch on Status rather than type-asserting the result.

// ─── CommandStatus ───────────────────────────────────────────────────────────

// CommandStatus is the outcome of a command execution. It is a string so
// that the constant value IS the human-readable label — no String() method
// is needed, and fmt.Println(StatusOK) prints "OK" directly.
type CommandStatus string

const (
	// StatusOK means the command completed successfully.
	StatusOK CommandStatus = "OK"

	// StatusPromptNeeded means the command cannot proceed without more
	// input from the user (e.g. ":w" on an unnamed buffer). The caller
	// should open the command line pre-seeded with the Result() string.
	StatusPromptNeeded CommandStatus = "PromptNeeded"

	// StatusRefused means the command was understood but refused by policy
	// (e.g. ":q" with unsaved changes). The caller should surface Err()
	// in the status bar without closing the command line.
	StatusRefused CommandStatus = "Refused"

	// StatusUnknown means no handler was found for the given verb. The
	// caller should name the unrecognised verb in the status bar.
	StatusUnknown CommandStatus = "Unknown"
)

// ─── CommandResponse ─────────────────────────────────────────────────────────

// CommandResponse is the value every command returns. R is the type of the
// command-specific result payload.
//
// All three methods are available on the interface so callers never need a
// type assertion: switch on Status(), read Result() on OK, read Err() on
// Refused/Unknown. The concrete implementation is Response[R] in
// commands-response.go.
type CommandResponse[R any] interface {
	// Status returns the outcome of the command execution.
	Status() CommandStatus

	// Result returns the command's typed payload. Its meaning is
	// command-specific; always check Status first.
	Result() R

	// Err returns the error associated with a non-OK response, or nil on
	// success. Callers display Err().Error() in the status bar.
	Err() error
}

// ─── Command[R] ──────────────────────────────────────────────────────────────

// Command is the typed signature for an ex command implementation.
//
//   - ctx is the TUI context, available for focus changes and dirty marks.
//   - arg is the text after the verb on the command line (e.g. the path in
//     ":w path/to/file"), already trimmed. Empty for commands with no argument.
//
// Commands return a CommandResponse[R] — typically a Response[R] built with
// the Ok, Prompt, or Refuse[R] constructors from commands-response.go.
//
// There is no separate error return. Every outcome is expressed through the
// response's Status and Err() so callers stay uniform.
type Command[R any] func(ctx *tui.Context, arg string) CommandResponse[R]

// ─── Handler ─────────────────────────────────────────────────────────────────

// Handler is the type-erased form of a command, stored in the Registry.
// It wraps a Command[R] so that commands of different result types can live
// in the same map. Produced by Register; not constructed directly.
type Handler struct {
	// run executes the command and boxes its result into CommandResponse[any].
	run func(ctx *tui.Context, arg string) CommandResponse[any]

	// help is a short one-line description for ":help" or a command palette.
	help string
}
