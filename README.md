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
make build          # -> bin/editor
make run FILE=x.md  # build and open x.md
```

`make` targets: `build`, `run`, `test`, `vet`, `fmt`, `tidy`, `clean`.
Every `cmd/*` directory becomes a binary in `bin/`; today that is `cmd/editor`.

## Using it

The editing panel is golib's `widget.Editor`, so it is modal and vim-flavoured
out of the box: `i` inserts, `Esc` returns to Normal, `hjkl`/`w`/`b` move, `dd`
deletes, `u` undoes, counts and visual modes work.

The footer shows **MODE** on the left, the file path in the middle and the clock
on the right. Typing `:` (or the configured leader key) turns that footer into a
command line:

| Command | Effect |
|---|---|
| `:q` | quit — **refused** if the buffer is modified |
| `:q!` | quit, discarding changes |
| `:w` | write; on an unnamed buffer it prompts for a name |
| `:w NAME` | write to `NAME` and adopt it as the buffer's path |
| `:wq` | write, then quit |

An unknown command is refused **by name** rather than ignored, so a typo says so
instead of appearing to work.

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
```

`editor.example.toml` states the defaults, and a test asserts it stays in sync
with them.

The parser is a **deliberately small subset of TOML** — sections, `key = value`,
booleans, quoted strings, `#` comments — chosen so the editor has no dependency
beyond golib. It is strict: an unknown section or key is a startup error rather
than a silent skip, because a setting that looks applied and is not is worse
than a refusal. If the config ever needs arrays, nested tables or datetimes,
swap in a real TOML library rather than growing this one.

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
