# Architecture Proposal: Keyboard Event Handling Separation (Revised)

**Author:** Juliet  
**Date:** 2026-09-09  
**Reviewer:** Lector  
**Status:** Revised per Architectural Review (`$KB_ROOT/agents/lector/reviews/2026-09-09-editor-keyboard-event-separation-proposal-review.md`)  
**Related Components:** `App`, `EditorPane`, `Footer`, `handler-keyboard.go`

---

## 1. Context & Problem Statement

In the refactored editor architecture, structural components have been split into modular units:
- **`EditorPane`**: owns the text buffer, modal editing state machine, path, dirty state, and document I/O.
- **`Footer`**: owns the status bar, command prompt label, and ex command text input.
- **`App`**: orchestrates top-level layout (`Dock` + `OverlayHost`) and ex command dispatch (`Registry`).

However, **keyboard event handling remains coupled inside `App.handleKey`**:

```go
func (a *App) handleKey(k tui.KeyEvent) bool {
    if k.Kind == tui.KeyRelease {
        return false
    }
    if a.footer.Commanding() {
        if k.Code == tui.KeyEscape {
            a.closeCommand()
            return true
        }
        return false
    }
    if a.editorPane.Mode() != widget.ModeNormal {
        return false
    }
    if k.Text == ":" || (a.cfg.Keyboard.LeaderKey != "" && k.Text == a.cfg.Keyboard.LeaderKey) {
        a.openCommand("")
        return true
    }
    return false
}
```

### Shortcomings
1. **Coupled Concerns in `App`**: `App` inspects `a.footer.Commanding()`, directly queries `a.editorPane.Mode()`, matches raw keys against `LeaderKey`, and executes transitions.
2. **Passive Ancestors**: `EditorPane` and `Footer` have passive `HandleEvent(tui.Event) bool { return false }` methods, relying entirely on `App` at the root of the tree.
3. **Collision Ambiguity**: Configurable keys (such as `LeaderKey`) can conflict with built-in vi bindings without a defined contract.

---

## 2. Framework Event Routing Model (`golib/tui`)

`golib/tui` v0.5.12 implements **leaf-first event routing with ancestor bubbling** (no capture phase):

```text
1. Active Focus Target (Leaf Widget: widget.Editor or widget.TextInput)
      │
      │ (If leaf returns false / unconsumed)
      ▼
2. Local Enclosing Component (Ancestor: EditorPane or Footer)
      │
      │ (If enclosing component returns false)
      ▼
3. Root Layout & Application (Dock ──► OverlayHost ──► App)
```

Because leaf widgets run first:
- When typing in **Insert mode**, `widget.Editor` consumes text and keys; they never bubble to `EditorPane`.
- When typing ex commands, `widget.TextInput` consumes normal text; only unhandled keys (like `<Esc>`) bubble to `Footer`.
- Normal mode navigation keys (e.g. `h`, `j`, `k`, `l`, `w`, `b`) are consumed by `widget.Editor`.
- Unbound keys (e.g. `:` or unmapped leader keys) bubble from `widget.Editor` up to `EditorPane`.

---

## 3. Analysis of Approaches

### Approach A: Ad-Hoc Matching in Components
Each component (`EditorPane`, `Footer`) embeds its own ad-hoc key comparisons and invokes arbitrary callbacks.
- **Flaws**: Violates DRY by duplicating key normalization and chord matching; scatters keybinding definitions across components.

### Approach B: Heavy Centralized Controller (`handler-keyboard.go` as Second Root)
A stateful `KeyboardHandler` service that holds references to `App`, `EditorPane`, and `Footer`, inspecting UI state and performing focus/visibility mutations.
- **Flaws**: Violates SRP and DIP by creating a second root controller under a different name, introducing duplicate state tracking and tight coupling.

### Approach C: Hybrid Boundary (Recommended & Adopted)
A **pure shared resolver** coupled with **component-local capture** and **synchronous action sinks**:
1. **`handler-keyboard.go`** is a **pure resolver**: defines `InputScope`, `KeyAction`, normalization, and `Resolve(InputScope, tui.KeyEvent) (KeyAction, bool)`. It holds **zero mutable UI state**, no component pointers, and no focus logic.
2. **`EditorPane` and `Footer`** receive a `KeyResolver` and a synchronous `KeyActionSink`:
   - `EditorPane` sees keys unconsumed by `widget.Editor`, checks its live Normal mode, resolves under `ScopeEditorNormal`, and emits semantic actions (e.g. `ActionOpenCommandLine`).
   - `Footer` sees keys unconsumed by `widget.TextInput`, resolves under `ScopeCommandLine`, and emits `ActionCancelCommandLine`.
3. **`App`** alone implements the `KeyActionSink` and acts as the transition coordinator: opening/closing the command line, toggling sibling visibility, shifting focus, and refreshing the status bar.

---

## 4. Detailed Design Specification

### 4.1 Type System (`handler-keyboard.go`)

```go
package editor

import "github.com/yongjohnlee80/golib/tui"

// InputScope identifies the logical interaction mode for key resolution.
type InputScope uint8

const (
    ScopeEditorNormal InputScope = iota
    ScopeCommandLine
)

// KeyAction represents a semantic action triggered by keyboard input.
type KeyAction uint8

const (
    ActionNone KeyAction = iota
    ActionOpenCommandLine
    ActionCancelCommandLine
)

// KeyResolver resolves physical key events into semantic actions within an input scope.
type KeyResolver interface {
    Resolve(scope InputScope, ev tui.KeyEvent) (KeyAction, bool)
}

// KeyActionSink is a synchronous callback for executing semantic key actions.
type KeyActionSink func(KeyAction)
```

### 4.2 Leader Key Collision Policy

To prevent ambiguous behavior when a user configures `keyboard.LeaderKey` in `Config`:
- **Policy (Fail-Fast Validation)**: During config validation (`config.go`), `LeaderKey` is validated against conflicting single-rune vi Normal mode commands (derived from the editor's effective keymap).
- If a user configures a key consumed by vi (e.g., `h`, `j`, `k`, `l`, `i`, `a`, `d`, `y`), configuration loading rejects it with an explicit descriptive error (`leader key %q collides with built-in vi binding`).
- Safe defaults (such as `" "` spacebar, or `","`) are accepted.
- This ensures `widget.Editor` and `EditorPane` never have silent collision failures.

### 4.3 Component Integration

#### `EditorPane`
```go
type EditorPane struct {
    // ... existing fields ...
    resolver KeyResolver
    sink     KeyActionSink
}

func (ep *EditorPane) HandleEvent(ev tui.Event) bool {
    ke, ok := ev.(tui.KeyEvent)
    if !ok || ke.Kind == tui.KeyRelease {
        return false
    }
    // Only resolve application actions in Normal mode. In Insert mode,
    // keys are consumed by the child widget.Editor before reaching here.
    if ep.Mode() != widget.ModeNormal || ep.resolver == nil || ep.sink == nil {
        return false
    }

    if action, ok := ep.resolver.Resolve(ScopeEditorNormal, ke); ok {
        ep.sink(action)
        return true
    }
    return false
}
```

#### `Footer`
```go
type Footer struct {
    // ... existing fields ...
    resolver KeyResolver
    sink     KeyActionSink
}

func (f *Footer) HandleEvent(ev tui.Event) bool {
    ke, ok := ev.(tui.KeyEvent)
    if !ok || ke.Kind == tui.KeyRelease {
        return false
    }
    if !f.commanding || f.resolver == nil || f.sink == nil {
        return false
    }

    if action, ok := f.resolver.Resolve(ScopeCommandLine, ke); ok {
        f.sink(action)
        return true
    }
    return false
}
```

#### `App` (The Action Coordinator)
`App` wires the synchronous action sink:
```go
func (a *App) handleKeyAction(action KeyAction) {
    switch action {
    case ActionOpenCommandLine:
        a.openCommand("")
    case ActionCancelCommandLine:
        a.closeCommand()
    }
}
```

### 4.4 Why Synchronous Sinks Over `Bus.Publish`
`golib/tui`'s application bus (`ctx.Bus().Publish`) is an enqueue-only broadcast mechanism processed on subsequent drain loops. Using it for mandatory state transitions:
1. Breaks synchronous atomicity on the application loop goroutine.
2. Cannot guarantee a consumed event triggers the corresponding transition immediately within the frame.
3. Obscures direct dependencies.

A direct `KeyActionSink` function provides a synchronous, single-owner contract that is easily tested and completely predictable.

---

## 5. Summary of SOLID & DRY Alignment

- **Single Responsibility Principle (SRP)**:
  - `handler-keyboard.go` owns binding definition, normalization, and resolution.
  - `EditorPane` & `Footer` own local ancestor event filtering.
  - `App` owns cross-component layout, focus, and lifecycle coordination.
- **Open/Closed Principle (OCP)**: New actions or scopes can be added to the resolver without changing how components capture events.
- **Liskov Substitution & Interface Segregation (ISP)**: Components depend solely on the minimal `KeyResolver` interface and `KeyActionSink` func.
- **Dependency Inversion (DIP)**: Neither `EditorPane` nor `Footer` imports or references `*App`.
- **DRY**: Binding tables and key matching logic are declared once in `handler-keyboard.go`.

---
*End of Revised Proposal.*
