# ADR-0001: Store the editor's configuration in TOML

- Status: accepted
- Date: 2026-09-07
- Deciders: Johno

## Context

The editor needs a handful of settings, and it needs them before a terminal
exists — wrap mode changes how the editing panel is constructed, so it cannot
be applied after the fact. That rules out asking interactively on first run.

This next paragraph is deliberately one very long unwrapped line, so that toggling HorizontalWrap in the config visibly changes how it is displayed and whether a horizontal scroll indicator appears at the right-hand edge of the panel.

Three options were considered:

1. Environment variables only. Cheap, but a set of six exported variables is
   not something anyone edits comfortably, and there is nowhere to write a
   comment explaining why a value was chosen.
2. A bespoke key=value format. No dependency, but every reader has to learn it,
   and it grows badly the moment a list or a nested table is needed.
3. TOML. One dependency, universally understood, comments are first class.

## Decision

TOML, parsed with `BurntSushi/toml`, decoded **onto** the defaults so that a
file setting one key leaves every other key alone.

Unknown keys are a startup **error**, not a silent skip:

```toml
[editor]
HorizontalWrap = true

[keyboard]
LeaderKey = " "
```

A setting that looks applied and is not is worse than a refusal — the reader
has no way to tell the difference without reading the source.

## Consequences

- A missing config file is not an error; the defaults are the documented ones.
- `editor.example.toml` states those defaults, and a test asserts it stays in
  sync, so the example cannot document a configuration nobody runs.
- If the config ever needs arrays or nested tables, they come for free.

## Notes for editing this file

Try these while it is open:

- `i` to insert, `Esc` to return to Normal mode — watch MODE in the footer.
- `dd` to delete a line, `u` to undo it.
- `:w` to write, `:q` to quit. `:q` refuses while the buffer is modified.
- `:wq` to do both, and `:q!` to discard.
