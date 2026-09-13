# Styling in `golib/tui`: Guide & Reference

This guide covers how styling works in [`golib/tui/style`](https://github.com/yongjohnlee80/golib/tree/main/tui/style) and how to apply colors and text formatting across widgets and custom components.

---

## 1. Core Principles

1. **Immutable Value Semantics**:
   A `style.Style` is an immutable struct value. Calling any setter (e.g. `.Foreground(...)`, `.Bold(...)`) returns a new copy with that property set. Instances never alias or leak state across components:

   ```go
   base := style.New().Foreground(style.ANSI(7))
   errorSt := base.Background(style.ANSI(1)).Bold(true) // 'base' remains untouched
   ```

2. **The Zero Value is Valid**:
   `style.Style{}` and `style.New()` represent a blank style that inherits defaults from the ambient terminal and theme.

3. **ANSI-16-First Philosophy ([ADR-0006](https://github.com/yongjohnlee80/golib/blob/main/docs/tui/adr-0006-styling-tokens-and-theming.md))**:
   `golib/tui` prioritizes ANSI-16 palette indices (`0`–`15`) over hardcoded RGB values. Emitting standard ANSI SGR codes allows the user's terminal emulator palette, contrast preferences, and accessibility settings to control the exact rendered hues.

---

## 2. Color Types in `golib/tui`

Colors in `golib/tui` are represented by the `style.Color` sum type:

| Constructor                   | Usage                          | Example                                         | When to Use                                         |
| ----------------------------- | ------------------------------ | ----------------------------------------------- | --------------------------------------------------- |
| `style.ANSI(0-15)`            | Standard 16-color ANSI palette | `style.ANSI(4)` _(Blue)_                        | **Default choice**. Respects user's terminal theme. |
| `style.ANSI256(0-255)`        | Extended 256-color palette     | `style.ANSI256(214)` _(Orange)_                 | Specific extended terminal colors.                  |
| `style.RGB(r, g, b)`          | 24-bit TrueColor               | `style.RGB(255, 128, 0)`                        | Fixed, theme-independent branding or gradients.     |
| `style.Adaptive(light, dark)` | Dynamic light/dark pair        | `style.Adaptive(style.ANSI(0), style.ANSI(15))` | Adaptive interfaces matching terminal background.   |
| `style.Default()`             | Terminal default color         | `style.Default()`                               | Unsets color override back to default.              |

> [!NOTE]
> `style.ANSI(n)` panics if `n < 0 || n > 15` to catch configuration typos immediately at startup.

---

## 3. ANSI-16 Palette Reference

| Index    | Color Name              | FG SGR | BG SGR | Reference Hex | Typical Role / Semantic                     |
| -------- | ----------------------- | ------ | ------ | ------------- | ------------------------------------------- |
| **`0`**  | **Black**               | `30`   | `40`   | `#000000`     | Terminal backdrop / dark base               |
| **`1`**  | **Red**                 | `31`   | `41`   | `#CD0000`     | Errors, deletions, refutations              |
| **`2`**  | **Green**               | `32`   | `42`   | `#00CD00`     | Success indicators, additions, strings      |
| **`3`**  | **Yellow**              | `33`   | `43`   | `#CDCD00`     | Warnings, search highlights                 |
| **`4`**  | **Blue**                | `34`   | `44`   | `#0000EE`     | Mode badges (`COMMAND:`), primary accent    |
| **`5`**  | **Magenta**             | `35`   | `45`   | `#CD00CD`     | Keywords, special symbols                   |
| **`6`**  | **Cyan**                | `36`   | `46`   | `#00CDCD`     | Functions, identifiers, links               |
| **`7`**  | **White**               | `37`   | `47`   | `#E5E5E5`     | Standard light gray / text foreground       |
| **`8`**  | **Bright Black** (Gray) | `90`   | `100`  | `#7F7F7F`     | Muted text, line numbers, borders           |
| **`9`**  | **Bright Red**          | `91`   | `101`  | `#FF0000`     | Critical alerts, compiler errors            |
| **`10`** | **Bright Green**        | `92`   | `102`  | `#00FF00`     | Active diff additions                       |
| **`11`** | **Bright Yellow**       | `93`   | `103`  | `#FFFF00`     | Cursor line highlights, active search match |
| **`12`** | **Bright Blue**         | `94`   | `104`  | `#5C5CFF`     | Directory listings, selection fills         |
| **`13`** | **Bright Magenta**      | `95`   | `105`  | `#FF00FF`     | Types, constants, numbers                   |
| **`14`** | **Bright Cyan**         | `96`   | `106`  | `#00FFFF`     | Preprocessor directives, regex groups       |
| **`15`** | **Bright White**        | `97`   | `107`  | `#FFFFFF`     | Emphasized text, high-contrast badges       |

---

## 4. Text Attributes & Styling Properties

A `style.Style` fluent chain supports text formatting:

```go
st := style.New().
    Foreground(style.ANSI(15)).     // Text color
    Background(style.ANSI(4)).      // Background fill color
    Bold(true).                     // SGR 1: Bold text
    Faint(true).                    // SGR 2: Dim / faint text
    Italic(true).                   // SGR 3: Italic text
    Underline(true).                // SGR 4: Underlined text
    Blink(true).                    // SGR 5: Blinking text
    Reverse(true).                  // SGR 7: Inverted FG and BG colors
    Strikethrough(true)             // SGR 9: Strikethrough text
```

---

## 5. How to Create Styled Text

### Pattern 1: Floating Command Box (`widget.TextInput` inside `widget.Box`)

Use `widget.NewTextInput` wrapped inside `widget.NewBox` with title and rounded border styling hooks:

```go
package main

import (
    "github.com/yongjohnlee80/golib/tui/style"
    "github.com/yongjohnlee80/golib/tui/widget"
)

// Floating command box container: rounded border with titled header
func newCommandBox() (*widget.TextInput, *widget.Box) {
    cmdInput := widget.NewTextInput()
    cmdBox := widget.NewBox(
        cmdInput,
        widget.WithTitle("COMMAND:"),
        widget.WithStyle(style.New().Border(style.BorderRounded)),
    )
    return cmdInput, cmdBox
}
```

### Pattern 2: Menu Bar and Explorer Cursor Highlight

The menu bar and dropdowns follow the Borland C++ / Turbo Vision styling with autodb explorer highlights:

```go
// Menu bar default: light grey background with black text
menuBarStyle := style.New().Background(style.ANSI(7)).Foreground(style.ANSI(0))

// Mnemonic accelerator accent: highlighted red letter
menuAccentStyle := style.New().Background(style.ANSI(7)).Foreground(style.ANSI(1)).Bold(true)

// Active cursor selection (mirroring autodb explorer cursorRowStyle):
// Cyan ANSI 6 background with black ANSI 0 text
cursorRowStyle := style.New().Background(style.ANSI(6)).Foreground(style.ANSI(0))

// Active cursor selection with accented hotkey:
cursorAccentStyle := style.New().Background(style.ANSI(6)).Foreground(style.ANSI(9)).Bold(true)
```

### Pattern 3: Direct Painting onto `tui.Surface` in `Render`

When building custom components implementing `tui.Component`, use `sur.SetCell(...)` or `sur.Fill(...)` in your `Render` method:

```go
func (m *MyComponent) Render(s tui.Surface) {
    // Define styles:
    bgStyle := style.New().Background(style.ANSI(7)).Foreground(style.ANSI(0))
    hotkeyStyle := style.New().Background(style.ANSI(7)).Foreground(style.ANSI(1)).Bold(true)

    // Fill an entire bounding rectangle:
    s.Fill(tui.Rect{X: 0, Y: 0, W: 10, H: 1}, " ", bgStyle)

    // Set accented hotkey character:
    s.SetCell(1, 0, "F", hotkeyStyle)
    s.SetCell(2, 0, "i", bgStyle)
    s.SetCell(3, 0, "l", bgStyle)
    s.SetCell(4, 0, "e", bgStyle)
}
```

---

## 6. Semantic Theme Tokens (`style.Token`)

For applications that support user-swappable themes, reference semantic tokens instead of raw color numbers:

```go
// Theme-adaptive styles:
panelStyle := style.New().
    Background(style.TokenPanel).
    Foreground(style.TokenForeground)

accentStyle := style.New().
    Background(style.TokenPrimary).
    Foreground(style.TokenTextOnPrimary)
```

Swapping the active `tui.Theme` re-colors all token-styled components automatically without widget cooperation.
