package editor

import (
	"testing"

	"github.com/yongjohnlee80/golib/tui"
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
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyF10}) // activate
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown}) // open dropdown
	// Move down to Exit (item 3)
	for i := 0; i < 3; i++ {
		mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown})
	}
	// Press Enter on Exit
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if tm.modal != modalExit {
		t.Fatalf("modal = %v, want modalExit", tm.modal)
	}
	if tm.exitChoice != 0 {
		t.Errorf("exitChoice = %d, want 0 (Yes)", tm.exitChoice)
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
	if tm.modal != modalNone {
		t.Errorf("modal = %v, want modalNone after Escape", tm.modal)
	}
	if quitCalled {
		t.Error("canceling Exit modal must not call OnQuit")
	}

	// 3. Test Exit toggled to No and confirmed
	tm.openExitModal(nil)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyRight}) // toggle to No
	if tm.exitChoice != 1 {
		t.Errorf("exitChoice = %d, want 1 (No)", tm.exitChoice)
	}
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if quitCalled {
		t.Error("selecting No must not call OnQuit")
	}
	if tm.modal != modalNone {
		t.Errorf("modal = %v, want modalNone", tm.modal)
	}
}

func TestMenuBar_ModalKeymaps_Flow(t *testing.T) {
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

	// Navigate to Option -> Keymaps
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyF10})   // activate
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyRight}) // move to Option
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown})  // open dropdown
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter}) // select Keymaps

	if tm.modal != modalKeymaps {
		t.Fatalf("modal = %v, want modalKeymaps", tm.modal)
	}

	// Select Nano using shortcut '2'
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: '2'})
	if switchedKeyset != widget.KeysetNano {
		t.Errorf("switchedKeyset = %v, want KeysetNano", switchedKeyset)
	}
	if statusMsg == "" {
		t.Error("status message should be set on keymap switch")
	}
	if tm.modal != modalNone {
		t.Errorf("modal = %v, want modalNone after commit", tm.modal)
	}

	// Re-open and select Vim using arrow down + Enter
	tm.openKeymapsModal(nil)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyUp}) // toggle to Vim (0)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if switchedKeyset != widget.KeysetVim {
		t.Errorf("switchedKeyset = %v, want KeysetVim", switchedKeyset)
	}
}

func TestMenuBar_ModalNotImplemented_Flow(t *testing.T) {
	tm := newTopMenu(TopMenuCallbacks{})
	mb := tm.Bar()

	// File -> New
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyF10})
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown}) // New is item 0
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})

	if tm.modal != modalNotImplemented {
		t.Fatalf("modal = %v, want modalNotImplemented", tm.modal)
	}
	if tm.modalMsg != "File -> New" {
		t.Errorf("modalMsg = %q, want File -> New", tm.modalMsg)
	}

	// Dismiss with Enter
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if tm.modal != modalNone {
		t.Errorf("modal = %v, want modalNone after dismiss", tm.modal)
	}

	// Help -> About
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyF10})
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyLeft}) // wrap to Help
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown})
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})

	if tm.modal != modalNotImplemented {
		t.Fatalf("modal = %v, want modalNotImplemented for About", tm.modal)
	}
	if tm.modalMsg != "Help -> About" {
		t.Errorf("modalMsg = %q, want Help -> About", tm.modalMsg)
	}

	// Dismiss with Escape
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEscape})
	if tm.modal != modalNone {
		t.Errorf("modal = %v, want modalNone after Escape", tm.modal)
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
	tm.modal = modalExit
	sz = mo.Layout(tui.Constraints{MinW: 0, MaxW: 80, MinH: 0, MaxH: 24})
	if sz.W != 80 || sz.H != 24 {
		t.Errorf("modal overlay sz = %+v, want (80, 24)", sz)
	}
}
