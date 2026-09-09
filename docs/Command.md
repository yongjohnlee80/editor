# Command Pattern & Architecture Conventions

This document outlines the command layer pattern used in the editor. It is intended as a guide and tutorial reference for separating application concerns, isolating business and OS operations from UI controls, and building extensible command registries.

---

## 1. Motivation & Core Principles

In interactive terminal applications (especially modal editors like Vim or Neovim), user inputs often dispatch distinct actions:
- Writing a buffer to disk (`:w`)
- Opening a file (`:e`)
- Discarding changes and quitting (`:q!`)

When these operations are embedded directly within the root UI component (e.g. inside `App.runCommand`), several problems arise:
1. **Coupled Concerns**: File I/O, error handling, path resolution, and modal UI states become tangled with UI layout and focus logic.
2. **Untestable Operations**: Testing file operations requires driving the full TUI event loop, terminal backend, and widget hierarchy.
3. **Rigid Vocabulary**: Adding, testing, or overriding commands requires modifying the central application component.

### Separation of Concerns

To address this, the command architecture separates four primary responsibilities:

| Concern | File | Responsibility |
|---|---|---|
| **Interface & Type Declarations** | [`commands.go`](../commands.go) | `CommandStatus`, `CommandResponse[R]`, `Command[R]`, `Handler` |
| **Response Implementations** | [`commands-response.go`](../commands-response.go) | `Response[R]`, constructors (`Ok`, `Prompt`, `Refuse`), `Register[R]` |
| **Registry & Dispatch** | [`commands-registry.go`](../commands-registry.go) | Verb-to-handler lookup, command line parsing, dispatch loop |
| **Buffer Abstraction** | [`document.go`](../document.go) | `Document` interface (decoupled from widgets) |
| **Domain/OS Implementations** | [`commands-os.go`](../commands-os.go) | Base I/O (`readInto`, `writeFrom`), `NewReadFileCmd`, `NewWriteFileCmd`, `InitOSCommands` |

---

## 2. Type Architecture

```
Command[R] (Typed Function)
   │
   ▼
Register[R](cmd, help)
   │ (Boxes R into any)
   ▼
Handler (Type-Erased) ──► Stored in Registry.handlers map[string]Handler
                               │
                               ▼
              Registry.Dispatch(ctx, line) ──► CommandResponse[any]
```

### 1. `CommandStatus` (String-backed Enum)

Rather than integer status codes requiring redundant `.String()` translation tables, `CommandStatus` is defined directly as `string`:

```go
type CommandStatus string

const (
    StatusOK           CommandStatus = "OK"
    StatusPromptNeeded CommandStatus = "PromptNeeded"
    StatusRefused      CommandStatus = "Refused"
    StatusUnknown      CommandStatus = "Unknown"
)
```

- Self-documenting when printed or serialized in logs.
- Eliminates secondary switch statements to resolve status labels.

### 2. `CommandResponse[R any]` (Error-Bearing Interface)

A command's execution outcome must capture both success payloads and failure states. Instead of returning `(CommandResponse[R], error)` where errors might compete with status codes, the response itself carries the error:

```go
type CommandResponse[R any] interface {
    Status() CommandStatus
    Result() R
    Err() error
}
```

This guarantees uniform consumption at call sites:
```go
resp := registry.Dispatch(ctx, line)
switch resp.Status() {
case StatusOK:
    // Handle success with resp.Result()
case StatusPromptNeeded:
    // Prompt user with prefill from resp.Result()
case StatusRefused, StatusUnknown:
    // Display resp.Err().Error()
}
```

### 3. `Command[R any]` Signature

Commands are typed function values:

```go
type Command[R any] func(ctx *tui.Context, arg string) CommandResponse[R]
```

- `ctx`: Framework context for UI queries or layout triggers.
- `arg`: Parsed argument string (e.g. path following `:w <path>`), already trimmed.
- Returns `CommandResponse[R]`, typically created using standard constructors:
  - `Ok[R](result R)`: Status is `StatusOK`, `Err()` is `nil`.
  - `Prompt(prefill string)`: Status is `StatusPromptNeeded`, carrying prefill hint.
  - `Refuse[R](err error)`: Status is `StatusRefused`, carrying explanatory error.

---

## 3. Separation of Interface & Implementation

A fundamental design rule in this codebase is: **Never mix interfaces with concrete implementations**.

### In `commands.go`
Contains only abstract types and contracts:
- `CommandStatus`
- `CommandResponse[R any]`
- `Command[R any]`
- `Handler` struct declaration

### In `commands-response.go`
Contains concrete struct definitions, method implementations, and constructor helpers:
- `Response[R any]` (implements `CommandResponse[R]`)
- `Ok[R]`, `Prompt`, `Refuse[R]` constructors
- `(h Handler) Run(...)` and `(h Handler) Help()`
- `Register[R any](cmd Command[R], help string) Handler` adapter

### In `document.go`
Defines the `Document` interface needed by buffer operations:
```go
type Document interface {
    Value() string
    SetValue(string)
    Path() string
    SetPath(string)
    MarkDirty()
    MarkClean()
}
```
Any widget (`EditorPane`), scratch buffer, or mock can implement `Document` without importing or depending on command implementations.

---

## 4. The Registry Pattern

The `Registry` (`commands-registry.go`) manages command verbs and aliases:

```go
reg := NewRegistry()

// Register commands with aliases
reg.Add(Register(NewWriteFileCmd(doc), "write buffer to disk"), "w", "write")
reg.Add(Register(NewReadFileCmd(doc), "edit (open) file from disk"), "e", "edit")

// Dispatch user input line
resp := reg.Dispatch(ctx, ":w notes.txt")
```

### Key Registry Features:
- **Type Erasure via `Register[R]`**: Adapts typed `Command[R]` into uniform `Handler` returning `CommandResponse[any]` so they can reside in a single `map[string]Handler`.
- **Multiple Aliases**: Supports registering multiple verbs (e.g. `"w"` and `"write"`) to the same handler in one call.
- **Overridable**: Re-adding a verb quietly overrides previous registrations, facilitating plugin or user customizations.
- **Robust Dispatch**: Trims leading whitespace and `:` prefixes, splits verb and arguments, and returns `StatusUnknown` with descriptive error if not found.

---

## 5. Layered OS Command Implementation (`commands-os.go`)

File operations demonstrate a two-tier testing and execution pattern:

```
[ NewWriteFileCmd / NewReadFileCmd ]  <-- Tier 2: Path handling, prompt logic, os.MkdirAll, os.Open
                 │
                 ▼
      [ writeFrom / readInto ]        <-- Tier 1: Pure I/O on io.Writer / io.Reader (POSIX formatting, line counting)
                 │
                 ▼
           [ Document ]               <-- Buffer abstraction (EditorPane or mockDocument)
```

1. **Pure I/O Core**:
   `readInto(doc Document, r io.Reader)` and `writeFrom(doc Document, w io.Writer)` operate strictly on streams. They handle line counting and POSIX trailing newline guarantees without any OS or filesystem dependencies. These are fast and 100% testable in-memory.
2. **OS Command Wrappers**:
   `NewWriteFileCmd` and `NewReadFileCmd` handle filesystem specifics:
   - Creating parent directories via `os.MkdirAll(dir, 0o755)`
   - Checking for empty paths on unnamed buffers (`StatusPromptNeeded`)
   - Managing file handles (`os.OpenFile`, `os.Open`)
3. **Public Registration Hook**:
   `InitOSCommands(reg *Registry, doc Document)` provides a clean entrypoint for application startup, mounting standard `:w`, `:write`, `:e`, `:edit` verbs onto the registry in one line.

---

## 6. Integration in `App`

The root component (`App`) delegates all ex command handling:

```go
func (a *App) registerCommands() {
    // Mount modular OS commands
    InitOSCommands(a.registry, a.editorPane)

    // Application lifecycle commands
    a.registry.Add(Register(quitCmd, "quit editor"), "q", "quit")
    a.registry.Add(Register(forceQuitCmd, "quit without saving"), "q!")
    a.registry.Add(Register(wqCmd, "write and quit"), "wq")
}

func (a *App) runCommand(line string) {
    a.closeCommand()
    resp := a.registry.Dispatch(a.ctx, line)

    switch resp.Status() {
    case StatusOK:
        if s, ok := resp.Result().(string); ok && s != "" {
            a.setMessage(s)
        }
    case StatusPromptNeeded:
        prefill := ""
        if s, ok := resp.Result().(string); ok {
            prefill = s
        }
        a.openCommand(prefill)
        a.setMessage("new file: type a name after :w")
    case StatusRefused, StatusUnknown:
        if resp.Err() != nil {
            a.setMessage(resp.Err().Error())
        }
    }
}
```

This keeps `App` focused on window layout and UI presentation while command semantics and execution remain isolated, modular, and testable.
