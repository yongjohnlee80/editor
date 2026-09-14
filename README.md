# editor

A small modal text editor built on [golib/tui](https://github.com/yongjohnlee80/golib).

```
┌ notes.md ──────────────────┐
│ the vim editor panel       │
│                            │
└────────────────────────────┘
 NORMAL   notes.md   14:22:07
```

## Build and run

```sh
make build            # -> bin/editor
make edit-sample      # open a scratch copy of the sample ADR
make run FILE=x.md    # build and open x.md
make version          # what the built binary reports
```

`make` targets: `build`, `run`, `sample`, `edit-sample`, `version`, `test`,
`race`, `vet`, `fmt`, `tidy`, `clean`. Every `cmd/*` directory becomes a binary
in `bin/`; today that is `cmd/editor`.

`make sample` copies `testdata/sample-adr.md` to `bin/sample-adr.md`. Because
`bin/` is gitignored, that copy is a build artifact: edit it, save it, delete
it — the tracked original is untouched. It carries a 234-character line on
purpose, so toggling `HorizontalWrap` visibly changes something.

## Command line

```
editor [FILE]                     open FILE, or an unnamed buffer
editor open [-config PATH] [FILE] the same, spelled explicitly
editor version                    version, commit and a greeting
editor --version                  the same
editor help [verb]                details for a verb
```

Commands follow the `lm/cmd/cli-v2` shape — one `Cmd*` type per verb registered
with `google/subcommands`, sharing flags through an embedded `Base`. A bare
argument that is not a verb is treated as a filename, so the common case needs
no verb; the verb set is read back from the commander rather than hard-coded, so
adding a verb cannot forget to teach the filename check about it.

`-config` works both before and after the verb, and the verb-level one wins.

## Using it

The editing panel is golib's `widget.Editor`, so it is modal and vim-flavoured
out of the box: `i` inserts, `Esc` returns to Normal, `hjkl`/`w`/`b` move, `dd`
deletes, `u` undoes, counts and visual modes work.

The footer acts as a dedicated status reporter: **MODE** on the left, the file
path or transient message in the middle, and the clock on the right.

Typing `:` (or the configured leader key) opens a floating, centered command input
box inside the editor pane:

| Command | Effect |
|---|---|
| `:q` | quit — **refused** if the buffer is modified |
| `:q!` | quit, discarding changes |
| `:w` | write; on an unnamed buffer it prompts for a name |
| `:w NAME` | write to `NAME` and adopt it as the buffer's path |
| `:wq` | write, then quit |

An unknown command is refused **by name** rather than ignored, so a typo says so
instead of appearing to work. Pressing `Esc` dismisses the command line and returns
focus to the buffer editor.

## Menu Bar

The editor provides a Borland C++ 3.0 / Turbo Vision-style menu bar with File and
Option categories on the left, and Help pegged to the right (when docked horizontally):

- **Activation & Accelerators**: Press `F10` to toggle the menu bar, or use dedicated
  Alt accelerators: `Alt+F` (File), `Alt+O` (Option), `Alt+H` (Help).
- **Mnemonic Navigation**: Menu items display accented hotkeys in red (`[F]ile`, `[O]ption`,
  `[H]elp`, `[N]ew`, `[O]pen`, `[S]ave`, `E[x]it`, `[K]eymaps`, `[A]bout`).
- **Cascading Submenus**: `Option -> Keymaps` opens a cascading submenu to the right
  offering `1. Vim (modal)` and `2. Nano (modeless)`, allowing live runtime keyset switching.
- **Modals**: Selecting `File -> Exit` opens an "Are you sure to quit?" confirmation modal.
  Unimplemented items display an informative "Not Implemented" modal dialog.
- **Behavioral Prototype Widget Architecture (ADR 0098)**: Menu bars, dropdowns, and dialogs are built
  from decoupled primitives (`Button`, `Modal`, `MenuItem`, `MenuBar`) serving as behavioral prototypes
  for upstream extraction into `golib/tui/widget`. Modals act as `tui.FocusScope` focus traps with
  automatic prior-focus restoration; child buttons are framework-mounted with distinct `NodeID` identities.
  Keyboard and mnemonic navigation are fully supported; mouse click gestures and pane resizing are specified
  in ADR 0098 for upstream implementation.
- **Configurable Placement**: The menu bar can be docked along any screen edge: `"top"`,
  `"bottom"`, `"left"`, or `"right"`. When placed on the left or right, it renders as a
  vertical navigation sidebar with selection highlights mirroring autodb's explorer panel.

## Configuration

Config is optional — a missing file is not an error. The search order is
`-config PATH`, then `$EDITOR_CONFIG`, then `./editor.toml`, then
`$XDG_CONFIG_HOME/editor/editor.toml`.

```toml
[editor]
# Soft-wrap long lines to the panel width. This also hides the horizontal
# scroll indicator, because wrapped text has no horizontal extent to scroll.
HorizontalWrap = false

[keyboard]
# A single key that opens the command line, alongside ":" — which always works,
# so a bad value here cannot lock you out.
LeaderKey = " "

[menu]
# Docking placement edge for the menu bar: "top", "bottom", "left", or "right".
Placement = "top"
```

`editor.example.toml` states the defaults, and a test asserts it stays in sync
with them.

Parsing is `BurntSushi/toml`, decoded **onto** the defaults, so a file that sets
one key leaves the others alone. It is **strict**: an unknown key or section is a
startup error rather than a silent skip, because a setting that looks applied and
is not is worse than a refusal.

## Tests

```sh
go test ./...
go test -race ./...
```

The editor is driven over `tui.TestBackend` — no PTY. That matters here: the
binary refuses to start without a terminal, so those tests are the only place
the wiring is actually observed rather than assumed. App state is loop-owned, so
the harness reads it through `App.Update`, which runs on the loop goroutine;
reading it directly is a data race, and the race detector says so.

## Architecture & Tutorial Guides

- [`docs/architecture.md`](docs/architecture.md): In-depth walkthrough of the editor's design, component lifecycles, `tui.Component` vs. `tui/widget`, the Flutter-style layout engine, and modal overlays.
- [`docs/styles.md`](docs/styles.md): Complete guide and reference for `golib/tui/style`, including the ANSI-16 color palette, typography attributes, and patterns for rendering styled colored text.
