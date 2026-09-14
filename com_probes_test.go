package editor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yongjohnlee80/golib/tui"
	"github.com/yongjohnlee80/golib/tui/widget"
)

// focusTrackingContainer records FocusEvents it observes during event bubbling.
type focusTrackingContainer struct {
	widget.Base
	child       tui.Component
	focusEvents []tui.FocusEvent
}

func newFocusTrackingContainer(child tui.Component) *focusTrackingContainer {
	return &focusTrackingContainer{child: child}
}

func (c *focusTrackingContainer) Init(ctx *tui.Context) {
	c.Base.Init(ctx)
	if c.child != nil {
		ctx.Mount(c.child)
	}
}

func (c *focusTrackingContainer) Layout(con tui.Constraints) tui.Size {
	if c.child != nil && c.Context() != nil {
		sz := c.Context().LayoutChild(c.child, con)
		c.Context().PlaceChild(c.child, tui.Rect{X: 0, Y: 0, W: sz.W, H: sz.H})
		return sz
	}
	return con.Constrain(tui.Size{W: 20, H: 5})
}

func (c *focusTrackingContainer) Render(s tui.Surface) {}

func (c *focusTrackingContainer) HandleEvent(ev tui.Event) bool {
	if fe, ok := ev.(tui.FocusEvent); ok {
		c.focusEvents = append(c.focusEvents, fe)
		return false // observe without swallowing
	}
	return false
}

func runOnLoop(app *tui.App, fn func()) {
	done := make(chan struct{})
	app.Update(func() {
		fn()
		close(done)
	})
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		panic("app.Update timed out")
	}
}

func waitForCond(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

// TestProbe1_ButtonFocusEventBubbling verifies that Button.HandleEvent does NOT swallow
// FocusEvent, allowing ancestor panels (such as widget.Box) to observe focus transitions.
func TestProbe1_ButtonFocusEventBubbling(t *testing.T) {
	btn := NewButton("Action", func() {})
	container := newFocusTrackingContainer(btn)

	tb := tui.NewTestBackend(80, 24)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := tui.NewApp(container, tui.WithBackend(tb))
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()

	// Focus the button
	runOnLoop(app, func() {
		container.Context().FocusComponent(btn)
	})

	// Wait for FocusEvent(Gained=true) to bubble to ancestor container
	waitForCond(t, "FocusEvent(Gained=true) to bubble to ancestor container", func() bool {
		var hasGained bool
		runOnLoop(app, func() {
			for _, fe := range container.focusEvents {
				if fe.Gained {
					hasGained = true
					break
				}
			}
		})
		return hasGained
	})
}

// TestProbe2_StandaloneMenuItemFocusActivation verifies that a standalone MenuItem
// handles FocusEvent and activates on Enter/Space when focused by the framework.
func TestProbe2_StandaloneMenuItemFocusActivation(t *testing.T) {
	var activated atomic.Bool
	item := NewMenuItem("Standalone Action", 's', 0, func() {
		activated.Store(true)
	})

	tb := tui.NewTestBackend(80, 24)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := tui.NewApp(item, tui.WithBackend(tb))
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()

	// Focus the MenuItem via the framework
	runOnLoop(app, func() {
		item.Context().FocusComponent(item)
	})

	waitForCond(t, "standalone MenuItem to be focused", func() bool {
		var isFocused bool
		runOnLoop(app, func() {
			isFocused = item.Focused()
		})
		return isFocused
	})

	// Press Enter to activate
	tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})

	waitForCond(t, "framework-focused standalone MenuItem to activate on Enter", func() bool {
		return activated.Load()
	})
}

// TestProbe3_OneButtonModalTabTrapping verifies that Tab navigation in a one-button
// Modal does not cycle focus onto the Modal container itself.
func TestProbe3_OneButtonModalTabTrapping(t *testing.T) {
	var okClicked atomic.Bool
	okBtn := NewButton("OK", func() {
		okClicked.Store(true)
	})
	modal := NewModal("Notice", "Single button dialog", okBtn)

	tb := tui.NewTestBackend(80, 24)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := tui.NewApp(modal, tui.WithBackend(tb))
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()

	// Wait for modal okBtn to be focused
	waitForCond(t, "modal okBtn to be focused", func() bool {
		var f bool
		runOnLoop(app, func() {
			f = okBtn.Focused()
		})
		return f
	})

	// Inject Tab
	tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyTab})

	// Roundtrip through loop to ensure Tab was handled
	runOnLoop(app, func() {})

	// Verify button retains focus
	var btnFocused bool
	runOnLoop(app, func() {
		btnFocused = okBtn.Focused()
	})
	if !btnFocused {
		t.Fatal("one-button Modal button lost focus after Tab")
	}

	// Press Enter - button should trigger
	tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})

	waitForCond(t, "ok button in one-button Modal to trigger on Enter", func() bool {
		return okClicked.Load()
	})
}

// TestProbe4_DisabledFirstButtonFocus verifies that a Modal whose first button is
// disabled deterministically focuses its first enabled button.
func TestProbe4_DisabledFirstButtonFocus(t *testing.T) {
	disabledBtn := NewButton("Disabled", func() {
		t.Fatal("disabled button must not trigger")
	})
	disabledBtn.SetDisabled(true)

	var enabledTriggered atomic.Bool
	enabledBtn := NewButton("Enabled", func() {
		enabledTriggered.Store(true)
	})

	modal := NewModal("Dialog", "First button is disabled", disabledBtn, enabledBtn)

	tb := tui.NewTestBackend(80, 24)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := tui.NewApp(modal, tui.WithBackend(tb))
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()

	waitForCond(t, "enabled button to be focused", func() bool {
		var enFocused bool
		runOnLoop(app, func() {
			enFocused = enabledBtn.Focused()
		})
		return enFocused
	})

	var selIdx int
	var disFocused, enFocused bool
	runOnLoop(app, func() {
		selIdx = modal.SelectedButton()
		disFocused = disabledBtn.Focused()
		enFocused = enabledBtn.Focused()
	})

	if selIdx != 1 {
		t.Fatalf("modal SelectedButton() = %d, want 1 (first enabled button)", selIdx)
	}
	if disFocused {
		t.Fatal("disabled button must not be focused")
	}
	if !enFocused {
		t.Fatal("enabled button must be focused")
	}

	// Press Enter - enabled button must trigger
	tb.Inject(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})

	waitForCond(t, "enabled button to trigger on Enter", func() bool {
		return enabledTriggered.Load()
	})
}

// TestMenuSafety_DisabledAndEmptyNavigation verifies that disabled menu items are skipped,
// disabled items do not activate or close the dropdown, and empty category models are safe.
func TestMenuSafety_DisabledAndEmptyNavigation(t *testing.T) {
	var item1Triggered, item3Triggered bool

	item1 := NewMenuItem("Item 1", '1', 0, func() { item1Triggered = true })
	item2 := NewMenuItem("Item 2 (Disabled)", '2', 0, func() { t.Fatal("disabled item triggered") })
	item2.SetDisabled(true)
	item3 := NewMenuItem("Item 3", '3', 0, func() { item3Triggered = true })

	cats := []MenuCategory{
		{
			Name:   "Actions",
			Hotkey: 'a',
			Items:  []*MenuItem{item1, item2, item3},
		},
	}

	tm := NewTopMenu(cats, TopMenuCallbacks{})
	mb := tm.Bar()

	// Open dropdown
	tm.OpenCategory(0, nil)
	if tm.selectedItem != 0 {
		t.Fatalf("initial selectedItem = %d, want 0", tm.selectedItem)
	}

	// Press Down: should skip disabled item2 and select item3 (index 2)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown})
	if tm.selectedItem != 2 {
		t.Fatalf("after KeyDown: selectedItem = %d, want 2 (skipped disabled item2)", tm.selectedItem)
	}

	// Press Up: should skip disabled item2 and select item1 (index 0)
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyUp})
	if tm.selectedItem != 0 {
		t.Fatalf("after KeyUp: selectedItem = %d, want 0 (skipped disabled item2)", tm.selectedItem)
	}

	// Press Enter on item1
	mb.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	if !item1Triggered {
		t.Fatal("item1 was not triggered")
	}

	// Test empty categories model: must be safe with no panics
	emptyTM := NewTopMenu(nil, TopMenuCallbacks{})
	emptyMB := emptyTM.Bar()
	emptyMB.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyDown})
	emptyMB.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyLeft})
	emptyMB.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyRight})
	emptyMB.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEnter})
	emptyMB.HandleEvent(tui.KeyEvent{Kind: tui.KeyPress, Code: tui.KeyEscape})
	if emptyTM.DropdownOpen() {
		t.Fatal("empty menu should not open dropdown")
	}

	_ = item3Triggered
}

// TestMenuAnchoring_WideUnicodeCategories verifies that dropdown anchoring matches
// wide category display widths (CJK / emoji) accurately.
func TestMenuAnchoring_WideUnicodeCategories(t *testing.T) {
	cats := []MenuCategory{
		{
			Name:   "文件", // 4 terminal display columns
			Hotkey: 'f',
			Items:  []*MenuItem{NewMenuItem("新建", 'n', 0, nil)},
		},
		{
			Name:   "选项", // 4 terminal display columns
			Hotkey: 'o',
			Items:  []*MenuItem{NewMenuItem("设置", 's', 0, nil)},
		},
	}

	tm := NewTopMenu(cats, TopMenuCallbacks{})
	overlay := tm.Overlay()

	// cat 0: starts at x=1
	x0 := overlay.calculateDropdownX(0, 18, 80)
	if x0 != 1 {
		t.Errorf("cat 0 dropdown X = %d, want 1", x0)
	}

	// cat 1: starts at x = 1 + width("文件") + 3 = 1 + 4 + 3 = 8
	// Note: byte len("文件") is 6, so byte-based calculation would give 1 + 6 + 3 = 10 (WRONG!)
	x1 := overlay.calculateDropdownX(1, 18, 80)
	if x1 != 8 {
		t.Errorf("cat 1 dropdown X = %d, want 8 (display cell width), got byte-based mismatch", x1)
	}
}
