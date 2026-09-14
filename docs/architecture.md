# Editor Architecture & TUI Concepts

This project serves as a reference implementation and tutorial for building terminal user interfaces (TUIs) using [`golib/tui`](https://github.com/yongjohnlee80/golib).

It demonstrates how to compose modal text editors, handle application lifecycles, manage focus and event routing, and structure component hierarchies.

---

## 1. Core Concepts: `tui.Component` vs. `tui/widget`

A common question when analyzing the codebase is the distinction between a **Component** and a **Widget**:

| Concept             | What it is                                                            | Where it lives                              | Examples                                                              |
| ------------------- | --------------------------------------------------------------------- | ------------------------------------------- | --------------------------------------------------------------------- |
| **`tui.Component`** | The fundamental **interface** required of _every_ node in the UI tree | `github.com/yongjohnlee80/golib/tui`        | Implemented by `App`, `tui.Dock`, `tui.Stack`, `widget.Box`           |
| **Widget**          | A pre-built, reusable **UI control struct**                           | `github.com/yongjohnlee80/golib/tui/widget` | `widget.Editor`, `widget.Box`, `widget.StatusBar`, `widget.TextInput` |

### `tui.Component` (The Interface Contract)

Every node in the component tree must satisfy `tui.Component`:

```go
type Component interface {
    Init(ctx *Context)             // called once on mount; retain ctx and subscribe
    Layout(c Constraints) Size     // constraints down, size up
    Render(s Surface)              // paint own chrome onto the terminal cell surface
    HandleEvent(ev Event) bool     // process keyboard, tick, or custom events
}
```

The runtime doesn't distinguish between an entire application screen, a layout container (`Dock`, `Flex`), or a leaf input field — all nodes participate in the same 4-method lifecycle.

### The Single-Goroutine Invariant

All component state, layout geometry, tree mutations, and event handlers are strictly owned by the **application loop goroutine**.

- Methods like `Init`, `Layout`, `Render`, and `HandleEvent` are only ever invoked on this loop.
- Because of this, component fields (such as `dirty`, `path`, `message` in `App`) require **no mutexes or locks** for normal operations.
- Background work (e.g. file I/O, subprocesses) is spawned via `ctx.Go(...)` or `app.Go(...)` and reports results back onto the loop goroutine through addressed `tui.TaskResult` events.

### Widgets and `widget.Base`

There is no `type Widget interface` in Go. Instead, "Widget" refers to the pre-packaged library of controls in `tui/widget`.

Every widget implements `tui.Component` and embeds `widget.Base` by value:

- **`widget.Base`** provides boilerplate plumbing: retaining the mount `Context`, exposing `NodeID()`, and providing `MarkDirty()` / `RequestLayout()`.
- Widgets opt into optional capabilities via type assertions checked by the runtime:
  - `tui.Focusable`: accepts keyboard focus (e.g., `TextInput`, `Editor`).
  - `tui.CursorReporter`: reports physical hardware cursor placement and shape to the terminal driver.
  - `tui.Container`: manages child components (e.g., `Box`, `Split`, `OverlayHost`).

---

## 2. Component Lifecycle & Call Conventions

### Who calls `Init`? (Interface Dispatch)

When searching for callers of `(a *App) Init(ctx *tui.Context)` using LSP (`<leader>gr`), tools report 0 direct references. This is because **`Init` is never called directly on `*App`**; it is called polymorphically through the `tui.Component` interface by the framework runtime:

```text
main()
  │
  ├─> editor.New(cfg, path, quit)      // instantiates *App
  │
  ├─> tui.NewApp(app, WithBackend(b))  // hands *App as root Component
  │
  └─> tuiApp.Run(ctx)
        │
        └─> a.mount(nil, a.root)       // in golib/tui/app.go:224
              │
              └─> comp.Init(n.ctx)     // in golib/tui/tree.go:96 (invokes App.Init)
```

### The Lifecycle Phases

1. **Mount (`Init`)**:
   - Invoked exactly once when a component enters the active UI tree.
   - The passed `*tui.Context` remains valid for the component's entire mounted lifetime.
   - Used to mount children (`ctx.Mount(child)`), register event subscriptions (`tui.SubscribeScoped`), and start periodic timers (`ctx.Every(time.Second)`).
2. **Layout (`Layout`)**:
   - Constraints flow down from parent to child (`c tui.Constraints`).
   - Containers size and place children via `ctx.LayoutChild` and `ctx.PlaceChild`.
   - Returns chosen `tui.Size`. No side effects or tree mutations are permitted during layout.
3. **Render (`Render`)**:
   - Paints visual cells into the provided `tui.Surface`.
   - Components only paint their own chrome/background; the framework handles clipping and automatically iterates over children to render them into sub-surfaces.
4. **Event Handling (`HandleEvent`)**:
   - Receives keyboard, mouse, tick, and task completion events.
   - Returning `true` consumes the event, stopping further bubbling up the tree.
5. **Teardown**:
   - Cancelling the root context or calling `quit()` unwinds all nodes, cancelling in-flight tasks and tearing down subscriptions.

---

## 3. Layout Model & Comparison with Flutter

The layout engine in `golib/tui` implements the same fundamental paradigm popularized by Flutter:

> _"Constraints go down. Sizes go up. Parent sets position."_

### How Layout Works

```text
Parent                                        Child
  │                                             │
  ├─── 1. Passes Constraints (min/max W & H) ──>│
  │    (ctx.LayoutChild(child, constraints))    │
  │                                             │
  │<── 2. Returns Chosen Size ──────────────────┤
  │    (return tui.Size{W, H})                  │
  │                                             │
  ├─── 3. Sets Position & Bounding Rect ───────>│
  │    (ctx.PlaceChild(child, rect))            │
```

### Comparison: `golib/tui` vs. Flutter

| Aspect                 | Flutter                                                                                                                                                | `golib/tui`                                                                                                    |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------- |
| **Layout Axiom**       | Constraints down, sizes up, parent positions                                                                                                           | Constraints down, sizes up, parent positions                                                                   |
| **Tree Architecture**  | **3 Trees**:<br>1. `Widget` (ephemeral/immutable blueprint)<br>2. `Element` (lifecycle & state manager)<br>3. `RenderObject` (layout, paint, hit-test) | **1 Retained Tree**:<br>A single persistent `tui.Component` pointer owns state, layout, rendering, and events. |
| **Equivalent Concept** | `golib/tui.Component` is equivalent to Flutter's **`RenderBox`**                                                                                       | Flutter's `RenderBox` is equivalent to `golib/tui.Component`                                                   |
| **State Handling**     | Re-instantiates `Widget` tree on `setState()`                                                                                                          | Mutates struct fields directly on the loop goroutine; calls `ctx.MarkDirty()`                                  |
| **Overlays**           | `Stack` and `Overlay`                                                                                                                                  | `tui.Stack` and `widget.OverlayHost`                                                                           |

---

## 4. UI Architecture of the Editor

The editor composes multiple widgets into a clean, layered terminal application:

```text
App (root tui.Component)
 └─ host (*widget.OverlayHost - embeds tui.Stack)
     ├─ Layer 0: dock (*tui.Dock)
     │   ├─ Configurable Edge: menu.bar (*MenuBar) [File  Option            Help]
     │   │                                         (Top, Bottom, Left, or Right)
     │   ├─ Center (Fills): editorPane (*EditorPane)
     │   │   ├─ editor (*widget.Editor)            [vi buffer & editing state]
     │   │   └─ commanding overlay:
     │   │       └─ cmdBox (*widget.Box)           [centered floating box]
     │   │           └─ cmdInput (*widget.TextInput)
     │   │
     │   └─ Bottom (Pinned): footer (*footer)
     │       └─ status (*widget.StatusBar)         [NORMAL  file.txt  14:22:00]
     │
     └─ Layer 1: menu.overlay (*MenuOverlay)
         ├─ Dropdown Card (renders active MenuCategory items)
         ├─ Cascading Submenu Cards (recursive submenuStack navigation of arbitrary depth)
         └─ activeModal (*Modal - implements tui.FocusScope, mounted under MenuOverlay)
             └─ child buttons (*Button - implements tui.Focusable, mounted via Context.Mount)
```

### Component Roles

1. **`editor *widget.Editor`**:
   The core text editing buffer. Owns the vi state machine (Normal vs. Insert mode), cursor navigation, line buffers, and text mutations.
2. **`editorPane *EditorPane`**:
   The primary workspace component wrapping the editor buffer and the floating command line. Manages `cmdInput` within a centered `widget.Box` overlay when commanding is active. Handles keyboard shortcuts unconsumed by the inner editor, owns focus switching between editor and `cmdInput`, and consumes command/editing inputs while deliberately allowing unhandled keys and unrecognized Alt chords to bubble to ancestor containers.
3. **`footer *footer`**:
   A dedicated 3-section status line pinned at the bottom: mode indicator on the left, file path or transient feedback message in the center, and wall clock on the right. Does not handle command input.
4. **`menu *TopMenu`**:
   The application menu controller comprising `MenuBar` (mounted on `dock` at Top, Bottom, Left, or Right) and `MenuOverlay` (mounted on `OverlayHost` layer 1). Fully decoupled from editor keysets or hardcoded switch statements; accepts arbitrary caller-supplied categories, items, and action closures.
5. **`Button`, `Modal`, `MenuItem` (Behavioral Prototypes - ADR 0098)**:
   Proving-ground behavioral prototypes establishing contracts for future `golib/tui/widget` primitives:
   - **`Button`**: Implements `tui.Focusable`, listens for runtime `FocusEvent` transitions and preserves ancestor event bubbling by returning `false`, supports disabled state (`AcceptsFocus() == false`), semantic roles (`ButtonRoleNormal`, `ButtonRoleDefault`, `ButtonRoleCancel`), and explicit `Mnemonic rune` declarations.
   - **`Modal`**: Implements `tui.FocusScope` (`TrapsFocus() == true`) to confine keyboard navigation during dialog interactions and restore prior focus upon unmount. Mounts all child buttons via `ctx.Mount(b)` into distinct `NodeID` identities, enforces pure rendering without manual child double-painting, and selects the first enabled or default-role button deterministically.
   - **`MenuItem`**: Standalone or popup menu item supporting display label, hotkey accelerator, disabled state, checked status, and child submenu items as an extensible data model. Enter/Space activation is supported under framework focus or dropdown selection.
6. **`host *widget.OverlayHost`**:
   Wraps the dock layout as layer 0 and hosts overlay cards (menu dropdowns, cascading submenus, and modals) on layer 1.

### Deep Dive: What `OverlayHost` Does

Terminal UIs lack CSS-style `z-index`. `widget.OverlayHost` solves multi-layer rendering and modal popups:

- **Z-Ordering**: Embedding `*tui.Stack`, any layers added to `host` render on top of the editor and receive input events in reverse order (top-most layer intercepts input first).
- **Bus Handshake**: Listens to `overlayOpenEvent` and `overlayCloseEvent` on the bus. Child widgets (such as dropdowns, fuzzy finders, or selection lists) can emit an open event anywhere in the tree, and `OverlayHost` automatically mounts the popup at the root level without requiring explicit plumbing through parent widgets.
- **Modal Attachment**: Provides the attachment anchor for `widget.Float` dialogs (e.g. confirmation prompts, search modals), which can be displayed and dismissed without disturbing the underlying editor geometry.

---

## 5. ANSI-16 Palette Reference

`golib/tui` follows an **ANSI-16-first** philosophy ([ADR-0006](file:///home/johno/go/pkg/mod/github.com/yongjohnlee80/golib@v0.5.12/docs/tui/adr-0006-styling-tokens-and-theming.md)). `style.ANSI(n)` accepts indices `0` through `15`, which map directly to standard terminal palette colors and SGR escape codes:

| Index | Color Name | FG SGR | BG SGR | Reference Hex | Typical Role / Semantic |
|---|---|---|---|---|---|
| **0** | Black | `30` | `40` | `#000000` | Terminal backdrop / dark base |
| **1** | Red | `31` | `41` | `#CD0000` | Errors, deletions, warnings |
| **2** | Green | `32` | `42` | `#00CD00` | Success, additions, strings |
| **3** | Yellow | `33` | `43` | `#CDCD00` | Warnings, search highlights |
| **4** | Blue | `34` | `44` | `#0000EE` | Mode badges (`COMMAND:`), primary |
| **5** | Magenta | `35` | `45` | `#CD00CD` | Keywords, special symbols |
| **6** | Cyan | `36` | `46` | `#00CDCD` | Functions, identifiers, links |
| **7** | White | `37` | `47` | `#E5E5E5` | Default foreground / light gray |
| **8** | Bright Black | `90` | `100` | `#7F7F7F` | Muted text, line numbers, borders |
| **9** | Bright Red | `91` | `101` | `#FF0000` | Critical alerts, bright errors |
| **10** | Bright Green | `92` | `102` | `#00FF00` | Active diff adds, bright success |
| **11** | Bright Yellow | `93` | `103` | `#FFFF00` | Cursor line, active highlights |
| **12** | Bright Blue | `94` | `104` | `#5C5CFF` | Directory names, selection |
| **13** | Bright Magenta | `95` | `105` | `#FF00FF` | Types, constants, numbers |
| **14** | Bright Cyan | `96` | `106` | `#00FFFF` | Preprocessor, regex groups |
| **15** | Bright White | `97` | `107` | `#FFFFFF` | Emphasized text, badge text |

### Key Properties
- **No Unsolicited Upsampling**: `style.ANSI(n)` always emits standard ANSI SGR codes so that the user's terminal emulator palette, contrast settings, and colorblind profiles are respected.
- **Background vs. Foreground**: Setting `.Background(style.ANSI(n))` emits SGR `40+n` (or `100+(n-8)` for brights); setting `.Foreground(style.ANSI(n))` emits SGR `30+n` (or `90+(n-8)` for brights).
- **Validation**: Calling `style.ANSI(n)` with `n < 0 || n > 15` panics immediately at construction to fail misconfigurations fast.

