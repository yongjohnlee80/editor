# Architecture Proposal: Keyboard Event Handling Separation (Revised)

**Author:** Juliet  
**Date:** 2026-09-09  
**Reviewer:** Lector  
**Status:** Implemented & Superseded by Floating Command Line in EditorPane & Menu Subsystem (2026-09-13)
**Related Components:** `App`, `EditorPane`, `Footer`, `TopMenu`, `handler-keyboard.go`

---

## 1. Context & Problem Statement

In the initial refactoring of the editor, structural components were divided as:
- **`EditorPane`**: owns the text buffer, modal editing state machine, path, dirty state, and document I/O.
- **`Footer`**: originally owned the status bar, command prompt label, and ex command text input.
- **`App`**: orchestrates top-level layout (`Dock` + `OverlayHost`) and ex command dispatch (`Registry`).

In subsequent refactoring (2026-09-13), **command-line ownership was moved entirely inside `EditorPane`** as a floating, centered `TextInput` overlay, turning `Footer` into a status-only reporter. Furthermore, a Borland C++ / Turbo Vision-style **`TopMenu`** subsystem was introduced (with File, Option, Help categories, cascading submenus, and modals).

However, the core keyboard architecture established here—a pure, stateless `KeyResolver` coupled with component-local ancestor capture and synchronous `KeyActionSink` dispatch—remains the architectural foundation for the entire application.

### 1.1 Historical Problem Statement (Pre-Refactor)

Prior to the adoption of this proposal, **keyboard event handling was coupled inside `App.handleKey`**:

```go
// (Historical implementation prior to refactor)
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

### Historical Shortcomings
1. **Coupled Concerns in `App`**: `App` inspected `a.footer.Commanding()`, directly queried `a.editorPane.Mode()`, matched raw keys against `LeaderKey`, and executed transitions.
2. **Passive Ancestors**: `EditorPane` and `Footer` had passive `HandleEvent(tui.Event) bool { return false }` methods, relying entirely on `App` at the root of the tree.
3. **Collision Ambiguity**: Configurable keys (such as `LeaderKey`) conflicted with built-in vi bindings without a defined contract.

---

## 2. Framework Event Routing Model (`golib/tui`)

`golib/tui` v0.5.12 implements **leaf-first event routing with ancestor bubbling** (no capture phase):

```text
1. Active Focus Target (Leaf Widget: widget.Editor or widget.TextInput)
      │
      │ (If leaf returns false / unconsumed)
      ▼
2. Local Enclosing Component (Ancestor: EditorPane or MenuBar)
      │
      │ (If enclosing component returns false)
      ▼
3. Root Layout & Application (Dock ──► OverlayHost ──► App)
```

Because leaf widgets run first:
- When typing in **Insert mode**, `widget.Editor` consumes text and keys; they never bubble to `EditorPane`.
- When typing ex commands, `widget.TextInput` consumes text; only unhandled keys (like `<Esc>`) bubble to `EditorPane` (under `ScopeCommandLine`).
- Normal mode navigation keys (e.g. `h`, `j`, `k`, `l`, `w`, `b`) are consumed by `widget.Editor`.
- Unbound keys (e.g. `:` or unmapped leader keys) bubble from `widget.Editor` up to `EditorPane` (under `ScopeEditorNormal`).
- Menu accelerators (`Alt+f`, `Alt+o`, `Alt+h`, `F10`) bubble to `EditorPane`, `MenuBar`, or `App`, where they resolve identically via the shared `KeyResolver`.
- Unrelated Alt chords return `(ActionNone, false)` and bubble to the host.

---

## 3. Analysis of Approaches

### Approach A: Ad-Hoc Matching in Components
Each component (`EditorPane`, `Footer`, `MenuBar`) embeds its own ad-hoc key comparisons and invokes arbitrary callbacks.
- **Flaws**: Violates DRY by duplicating key normalization and chord matching; scatters keybinding definitions across components.

### Approach B: Heavy Centralized Controller (`handler-keyboard.go` as Second Root)
A stateful `KeyboardHandler` service that holds references to `App`, `EditorPane`, and `MenuBar`, inspecting UI state and performing focus/visibility mutations.
- **Flaws**: Violates SRP and DIP by creating a second root controller under a different name, introducing duplicate state tracking and tight coupling.

### Approach C: Hybrid Boundary (Implemented Architecture)
A **pure shared resolver** coupled with **component-local capture** and **synchronous action sinks**:
1. **`handler-keyboard.go`** is a **pure resolver**: defines `InputScope`, `KeyAction`, normalization, and `Resolve(InputScope, tui.KeyEvent) (KeyAction, bool)`. It holds **zero mutable UI state**, no component pointers, and no focus logic.
2. **`EditorPane`** receives a `KeyResolver` and a synchronous `KeyActionSink`:
   - Sees keys unconsumed by `widget.Editor`, checks its live Normal mode, resolves under `ScopeEditorNormal`, and emits semantic actions (e.g. `ActionOpenCommandLine`, `ActionOpenMenuFile`).
   - Sees keys unconsumed by `widget.TextInput`, resolves under `ScopeCommandLine`, and emits `ActionCancelCommandLine` or menu actions.
3. **`MenuBar`** resolves accelerators under `ScopeEditorNormal` and toggles or opens dropdowns.
4. **`App`** alone implements the `KeyActionSink` and acts as the transition coordinator: opening/closing the command line, toggling menu bar and dropdowns, shifting focus, and refreshing status.

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

#### `MenuBar`
```go
type MenuBar struct {
    menu *TopMenu
    // ...
}

func (mb *MenuBar) HandleEvent(ev tui.Event) bool {
    ke, ok := ev.(tui.KeyEvent)
    if !ok || ke.Kind == tui.KeyRelease {
        return false
    }
    // Route Alt accelerators and F10 toggle through shared key resolver
    if mb.menu.resolver != nil {
        if action, ok := mb.menu.resolver.Resolve(ScopeEditorNormal, ke); ok {
            switch action {
            case ActionToggleMenuBar:
                mb.menu.Toggle(ctx)
                return true
            case ActionOpenMenuFile:
                mb.menu.OpenCategory(0, ctx)
                return true
            // ...
            }
        }
    }
    // ...
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
    case ActionToggleMenuBar:
        a.toggleMenuBar()
    case ActionOpenMenuFile:
        a.openMenuCategory(0)
    case ActionOpenMenuOption:
        a.openMenuCategory(1)
    case ActionOpenMenuHelp:
        a.openMenuCategory(2)
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
