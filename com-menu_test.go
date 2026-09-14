package editor

import (
	"strings"
	"testing"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/style"
	"github.com/yongjohnlee80/golib/tui/widget"
)

func TestTopMenu_Definition(t *testing.T) {
	tm := newTopMenu(TopMenuCallbacks{})

	if len(tm.categories) != 3 {
		t.Fatalf("len(categories) = %d, want 3", len(tm.categories))
	}
	if tm.categories[0].Name != "File" {
		t.Errorf("cat 0 = %q, want File", tm.categories[0].Name)
	}
	if tm.categories[1].Name != "Option" {
		t.Errorf("cat 1 = %q, want Option", tm.categories[1].Name)
	}
	if tm.categories[2].Name != "Help" {
		t.Errorf("cat 2 = %q, want Help", tm.categories[2].Name)
	}

	// File has New, Open, Save, Exit
	fileItems := tm.categories[0].Items
	if len(fileItems) != 4 {
		t.Fatalf("len(fileItems) = %d, want 4", len(fileItems))
	}
	expectedFile := []string{"New", "Open", "Save", "Exit"}
	for i, want := range expectedFile {
		if fileItems[i].Name != want {
			t.Errorf("file item %d = %q, want %q", i, fileItems[i].Name, want)
		}
	}

	// Option has Keymaps
	optItems := tm.categories[1].Items
	if len(optItems) != 1 || optItems[0].Name != "Keymaps" {
		t.Errorf("optItems = %+v, want [Keymaps]", optItems)
	}

	// Help has About
	helpItems := tm.categories[2].Items
	if len(helpItems) != 1 || helpItems[0].Name != "About" {
		t.Errorf("helpItems = %+v, want [About]", helpItems)
	}
}

func TestMenuBar_F10_ActivationAndNavigation(t *testing.T) {
	var restored bool
	tm := newTopMenu(TopMenuCallbacks{
		OnRestoreFocus: func() { restored = true },
	})
	mb := tm.Bar()

	if tm.Active() {
		t.Error("menu bar must start inactive")
	}

	// Press F10 to activate
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyF10})
	if !tm.Active() {
		t.Error("F10 must activate menu bar")
	}
	if tm.selectedCategory != 0 {
		t.Errorf("selectedCategory = %d, want 0 (File)", tm.selectedCategory)
	}

	// Press Right arrow -> Option (1)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyRight})
	if tm.selectedCategory != 1 {
		t.Errorf("selectedCategory = %d, want 1 (Option)", tm.selectedCategory)
	}

	// Press Right arrow -> Help (2)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyRight})
	if tm.selectedCategory != 2 {
		t.Errorf("selectedCategory = %d, want 2 (Help)", tm.selectedCategory)
	}

	// Press Right arrow -> wraps back to File (0)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyRight})
	if tm.selectedCategory != 0 {
		t.Errorf("selectedCategory = %d, want 0 (File wrap)", tm.selectedCategory)
	}

	// Press Left arrow -> wraps to Help (2)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyLeft})
	if tm.selectedCategory != 2 {
		t.Errorf("selectedCategory = %d, want 2 (Help wrap)", tm.selectedCategory)
	}

	// Press Escape -> deactivates menu
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEscape})
	if tm.Active() {
		t.Error("Escape must deactivate menu bar")
	}
	if !restored {
		t.Error("deactivate must invoke OnRestoreFocus")
	}
}

func TestMenuBar_Dropdown_NavigationAndClose(t *testing.T) {
	tm := newTopMenu(TopMenuCallbacks{})
	mb := tm.Bar()

	// Activate
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyF10})

	// Press Down to open dropdown for File
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown})
	if !tm.DropdownOpen() {
		t.Fatal("Down arrow must open dropdown")
	}
	if tm.selectedItem != 0 {
		t.Errorf("selectedItem = %d, want 0 (New)", tm.selectedItem)
	}

	// Move down through items: Open (1), Save (2), Exit (3)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown})
	if tm.selectedItem != 1 {
		t.Errorf("selectedItem = %d, want 1 (Open)", tm.selectedItem)
	}
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown})
	if tm.selectedItem != 2 {
		t.Errorf("selectedItem = %d, want 2 (Save)", tm.selectedItem)
	}
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown})
	if tm.selectedItem != 3 {
		t.Errorf("selectedItem = %d, want 3 (Exit)", tm.selectedItem)
	}

	// Press Right arrow to switch dropdown to Option (1)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyRight})
	if tm.selectedCategory != 1 {
		t.Errorf("selectedCategory = %d, want 1 (Option)", tm.selectedCategory)
	}
	if !tm.DropdownOpen() {
		t.Error("dropdown must remain open when switching category with Right arrow")
	}
	if tm.selectedItem != 0 {
		t.Errorf("selectedItem = %d, want 0 (Keymaps)", tm.selectedItem)
	}

	// Press Escape to close dropdown
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEscape})
	if tm.DropdownOpen() {
		t.Error("Escape must close dropdown")
	}
	if !tm.Active() {
		t.Error("menu bar must remain active after closing dropdown")
	}
}

func TestMenuBar_ModalExit_Flow(t *testing.T) {
	var quitCalled bool
	tm := newTopMenu(TopMenuCallbacks{
		OnQuit: func() { quitCalled = true },
	})
	mb := tm.Bar()

	// 1. Test Exit confirmed with Enter on Yes
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyF10})  // activate
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown}) // open dropdown
	// Move down to Exit (item 3)
	for i := 0; i < 3; i++ {
		mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown})
	}
	// Press Enter on Exit
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if !tm.ModalActive() || tm.ActiveModal() == nil {
		t.Fatal("modal should be active")
	}
	if tm.ActiveModal().Title() != "Exit Confirmation" {
		t.Errorf("modal Title = %q, want Exit Confirmation", tm.ActiveModal().Title())
	}
	if tm.ActiveModal().SelectedButton() != 0 {
		t.Errorf("selected button = %d, want 0 (Yes)", tm.ActiveModal().SelectedButton())
	}

	// Confirm with Enter
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if !quitCalled {
		t.Error("confirming Exit modal must call OnQuit")
	}

	// 2. Test Exit canceled with Escape
	quitCalled = false
	tm.openExitModal(nil)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEscape})
	if tm.ModalActive() {
		t.Errorf("modal must be inactive after Escape")
	}
	if quitCalled {
		t.Error("canceling Exit modal must not call OnQuit")
	}

	// 3. Test Exit toggled to No and confirmed
	tm.openExitModal(nil)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyRight}) // toggle to No
	if tm.ActiveModal().SelectedButton() != 1 {
		t.Errorf("selected button = %d, want 1 (No)", tm.ActiveModal().SelectedButton())
	}
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if quitCalled {
		t.Error("selecting No must not call OnQuit")
	}
	if tm.ModalActive() {
		t.Errorf("modal must be inactive after No")
	}
}

func TestMenuBar_CascadingSubmenuKeymaps_Flow(t *testing.T) {
	var switchedKeyset widget.Keyset
	var statusMsg string
	tm := newTopMenu(TopMenuCallbacks{
		OnSetKeyset: func(ks widget.Keyset) {
			switchedKeyset = ks
		},
		OnStatusMessage: func(msg string) {
			statusMsg = msg
		},
	})
	mb := tm.Bar()

	// Navigate to Option -> Keymaps using Alt+o shortcut
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: 'o', Mods: tui.ModAlt})
	if !tm.DropdownOpen() || tm.selectedCategory != 1 {
		t.Fatalf("Alt+o must open Option dropdown: dropdownOpen=%v, cat=%d", tm.DropdownOpen(), tm.selectedCategory)
	}

	// Press hotkey 'k' to open cascading submenu on the right
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: 'k'})
	if !tm.SubmenuOpen() {
		t.Fatal("hotkey 'k' must open cascading submenu")
	}

	// Select Nano using shortcut '2'
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: '2'})
	if switchedKeyset != widget.KeysetNano {
		t.Errorf("switchedKeyset = %v, want KeysetNano", switchedKeyset)
	}
	if statusMsg == "" {
		t.Error("status message should be set on keymap switch")
	}
	if tm.Active() {
		t.Errorf("menu should deactivate after selecting keymap")
	}

	// Re-open and select Vim using '1'
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: 'o', Mods: tui.ModAlt})
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyRight}) // open submenu
	if !tm.SubmenuOpen() {
		t.Fatal("Right arrow must open cascading submenu")
	}
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: '1'})
	if switchedKeyset != widget.KeysetVim {
		t.Errorf("switchedKeyset = %v, want KeysetVim", switchedKeyset)
	}
}

func TestMenuBar_ArbitrarySubmenuDepth(t *testing.T) {
	var deepTriggered bool
	deepItem := NewMenuItem("Level 3 Action", 'a', 8, func() {
		deepTriggered = true
	})
	level2 := NewMenuItemWithSubmenu("Level 2", '2', 6, deepItem)
	level1 := NewMenuItemWithSubmenu("Level 1", '1', 6, level2)

	tm := newTopMenu(TopMenuCallbacks{})
	tm.categories = append(tm.categories, MenuCategory{
		Name:      "Deep",
		Hotkey:    'd',
		HotkeyIdx: 0,
		Items:     []*MenuItem{level1},
	})
	mb := tm.Bar()

	// Open Deep menu (cat 3)
	tm.OpenCategory(3, nil)
	if !tm.DropdownOpen() {
		t.Fatal("dropdown should be open")
	}

	// Open level 1 submenu
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyRight})
	if len(tm.submenuStack) != 1 {
		t.Fatalf("submenuStack depth = %d, want 1", len(tm.submenuStack))
	}

	// Open level 2 submenu
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyRight})
	if len(tm.submenuStack) != 2 {
		t.Fatalf("submenuStack depth = %d, want 2", len(tm.submenuStack))
	}

	// Trigger Level 3 Action
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if !deepTriggered {
		t.Fatal("deep item action was not triggered")
	}
	if tm.Active() {
		t.Fatal("menu should deactivate after action trigger")
	}
}

func TestMenuBar_DynamicCategoriesRendering(t *testing.T) {
	tm := newTopMenu(TopMenuCallbacks{})
	// Add 4th category
	tm.categories = append(tm.categories, MenuCategory{
		Name:      "Tools",
		Hotkey:    't',
		HotkeyIdx: 0,
		Items: []*MenuItem{
			NewMenuItem("Linter", 'l', 0, nil),
		},
	})

	surf := newMockSurface(80, 1)
	tm.Bar().Render(surf)
	screen := surf.String()

	if !strings.Contains(screen, "File") || !strings.Contains(screen, "Option") ||
		!strings.Contains(screen, "Help") || !strings.Contains(screen, "Tools") {
		t.Fatalf("menu bar did not render all categories including 4th category; screen:\n%s", screen)
	}
}

func TestMenuBar_MnemonicHotkeys(t *testing.T) {
	tm := newTopMenu(TopMenuCallbacks{})
	mb := tm.Bar()

	// Activate menu
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyF10})

	// Press 'h' -> selects Help and opens dropdown
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: 'h'})
	if !tm.DropdownOpen() || tm.selectedCategory != 2 {
		t.Errorf("expected Help dropdown open, got cat=%d, open=%v", tm.selectedCategory, tm.DropdownOpen())
	}

	// Close dropdown
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEscape})

	// Press 'f' -> selects File and opens dropdown
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: 'f'})
	if !tm.DropdownOpen() || tm.selectedCategory != 0 {
		t.Errorf("expected File dropdown open, got cat=%d, open=%v", tm.selectedCategory, tm.DropdownOpen())
	}

	// Press 'x' -> triggers Exit modal
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: 'x'})
	if !tm.ModalActive() || tm.ActiveModal() == nil {
		t.Fatal("hotkey 'x' should open Exit modal")
	}
}

func TestMenuBar_PlacementLayout(t *testing.T) {
	tm := newTopMenu(TopMenuCallbacks{})
	mb := tm.Bar()

	// Default: Top -> H = 1
	sz := mb.Layout(tui.Constraints{MaxW: 80, MaxH: 24})
	if sz.W != 80 || sz.H != 1 {
		t.Errorf("Top placement layout = %+v, want (80, 1)", sz)
	}

	// Left: W = 16, H = MaxH (explorer style)
	tm.SetPlacement(PlacementLeft)
	sz = mb.Layout(tui.Constraints{MaxW: 80, MaxH: 24})
	if sz.W != 16 || sz.H != 24 {
		t.Errorf("Left placement layout = %+v, want (16, 24)", sz)
	}

	// Right: W = 16, H = MaxH
	tm.SetPlacement(PlacementRight)
	sz = mb.Layout(tui.Constraints{MaxW: 80, MaxH: 24})
	if sz.W != 16 || sz.H != 24 {
		t.Errorf("Right placement layout = %+v, want (16, 24)", sz)
	}
}

func TestMenuBar_ModalNotImplemented_Flow(t *testing.T) {
	tm := newTopMenu(TopMenuCallbacks{})
	mb := tm.Bar()

	// File -> New
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyF10})
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown}) // New is item 0
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})

	if !tm.ModalActive() || tm.ActiveModal() == nil {
		t.Fatal("modal should be active for New")
	}
	if tm.ActiveModal().Title() != "Not Implemented" {
		t.Errorf("modal Title = %q, want Not Implemented", tm.ActiveModal().Title())
	}

	// Dismiss with Enter
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if tm.ModalActive() {
		t.Errorf("modal should be inactive after dismiss")
	}

	// Help -> About
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyF10})
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyLeft}) // wrap to Help
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown})
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})

	if !tm.ModalActive() || tm.ActiveModal() == nil {
		t.Fatal("modal should be active for About")
	}

	// Dismiss with Escape
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEscape})
	if tm.ModalActive() {
		t.Errorf("modal should be inactive after Escape")
	}
}

func TestMenuOverlay_Layout(t *testing.T) {
	tm := newTopMenu(TopMenuCallbacks{})
	mo := tm.Overlay()

	// When inactive, layout size is empty
	sz := mo.Layout(tui.Constraints{MinW: 0, MaxW: 80, MinH: 0, MaxH: 24})
	if sz.W != 0 || sz.H != 0 {
		t.Errorf("inactive overlay sz = %+v, want (0, 0)", sz)
	}

	// When dropdown is open, layout spans full area
	tm.dropdownOpen = true
	sz = mo.Layout(tui.Constraints{MinW: 0, MaxW: 80, MinH: 0, MaxH: 24})
	if sz.W != 80 || sz.H != 24 {
		t.Errorf("active dropdown overlay sz = %+v, want (80, 24)", sz)
	}

	// When modal is active, layout spans full area
	tm.dropdownOpen = false
	tm.openExitModal(nil)
	sz = mo.Layout(tui.Constraints{MinW: 0, MaxW: 80, MinH: 0, MaxH: 24})
	if sz.W != 80 || sz.H != 24 {
		t.Errorf("modal overlay sz = %+v, want (80, 24)", sz)
	}
}

func TestMenuStyle_Standalone(t *testing.T) {
	var nilStyle *MenuStyle
	if nilStyle.Bar() != defaultMenuStyle.bar {
		t.Fatal("nilStyle.Bar() did not return defaultMenuStyle.bar")
	}
	if nilStyle.Accent() != defaultMenuStyle.accent {
		t.Fatal("nilStyle.Accent() did not return defaultMenuStyle.accent")
	}
	if nilStyle.Highlight() != defaultMenuStyle.highlight {
		t.Fatal("nilStyle.Highlight() did not return defaultMenuStyle.highlight")
	}
	if nilStyle.HighlightAccent() != defaultMenuStyle.highlightAccent {
		t.Fatal("nilStyle.HighlightAccent() did not return defaultMenuStyle.highlightAccent")
	}
	if nilStyle.Border() != defaultMenuStyle.border {
		t.Fatal("nilStyle.Border() did not return defaultMenuStyle.border")
	}
	if nilStyle.ItemStyle(false) != defaultMenuStyle.bar {
		t.Fatal("nilStyle.ItemStyle(false) did not return defaultMenuStyle.bar")
	}
	if nilStyle.ItemStyle(true) != defaultMenuStyle.highlight {
		t.Fatal("nilStyle.ItemStyle(true) did not return defaultMenuStyle.highlight")
	}
	if nilStyle.AccentStyle(false) != defaultMenuStyle.accent {
		t.Fatal("nilStyle.AccentStyle(false) did not return defaultMenuStyle.accent")
	}
	if nilStyle.AccentStyle(true) != defaultMenuStyle.highlightAccent {
		t.Fatal("nilStyle.AccentStyle(true) did not return defaultMenuStyle.highlightAccent")
	}

	bar := style.New().Foreground(style.ANSI(1))
	acc := style.New().Foreground(style.ANSI(2))
	hl := style.New().Foreground(style.ANSI(3))
	hlAcc := style.New().Foreground(style.ANSI(4))
	border := style.New().Foreground(style.ANSI(5))
	custom := NewMenuStyle(bar, acc, hl, hlAcc, border)

	if custom.Bar() != bar || custom.Accent() != acc || custom.Highlight() != hl ||
		custom.HighlightAccent() != hlAcc || custom.Border() != border {
		t.Fatal("custom MenuStyle getters did not return expected styles")
	}
	if custom.ItemStyle(false) != bar || custom.ItemStyle(true) != hl {
		t.Fatal("custom ItemStyle did not match expected values")
	}
	if custom.AccentStyle(false) != acc || custom.AccentStyle(true) != hlAcc {
		t.Fatal("custom AccentStyle did not match expected values")
	}

	tm := newTopMenu(TopMenuCallbacks{})
	if tm.MenuStyle() != defaultMenuStyle {
		t.Fatal("tm.MenuStyle() should return defaultMenuStyle when unset")
	}
	if tm.Bar().MenuStyle() != defaultMenuStyle {
		t.Fatal("tm.Bar().MenuStyle() should return defaultMenuStyle when unset")
	}

	tm.SetStyle(custom)
	if tm.MenuStyle() != custom {
		t.Fatal("tm.MenuStyle() did not return custom style after SetStyle")
	}
	if tm.Bar().MenuStyle() != custom {
		t.Fatal("tm.Bar().MenuStyle() did not return custom style after SetStyle")
	}

	tm.Bar().SetStyles(bar, accent, highlight, highlightAccent, border)
	if tm.MenuStyle().Bar() != bar {
		t.Fatal("Bar().SetStyles did not apply custom bar style")
	}
}
