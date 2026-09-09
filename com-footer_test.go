package editor

import (
	"context"
	"testing"
	"time"

	"github.com/yongjohnlee80/golib/tui"
)

// These tests exercise Footer in isolation or over a minimal test harness.
func TestFooter_Construction(t *testing.T) {
	f, _ := newFooter(NewDefaultKeyResolver(" "), func(KeyAction){})
	if f.status == nil {
		t.Fatal("expected status bar widget to be initialized")
	}
	if f.prompt == nil {
		t.Fatal("expected prompt text widget to be initialized")
	}
	if f.input == nil {
		t.Fatal("expected text input widget to be initialized")
	}
	if f.Commanding() {
		t.Error("expected commanding mode to be false initially")
	}
}

func TestFooter_OpenAndCloseCommand(t *testing.T) {
	f, _ := newFooter(NewDefaultKeyResolver(" "), func(KeyAction){})
	f.OpenCommand("w ")
	if !f.Commanding() {
		t.Error("expected commanding to be true after OpenCommand")
	}
	if got := f.CommandValue(); got != "w " {
		t.Errorf("expected command value 'w ', got %q", got)
	}

	f.CloseCommand()
	if f.Commanding() {
		t.Error("expected commanding to be false after CloseCommand")
	}
	if got := f.CommandValue(); got != "" {
		t.Errorf("expected command value empty, got %q", got)
	}
}

func TestFooter_HarnessRendering(t *testing.T) {
	tb := tui.NewTestBackend(80, 1)
	f, _ := newFooter(NewDefaultKeyResolver(" "), func(KeyAction){})
	f.SetStatus(" NORMAL ", "test.txt", "12:00:00 ")

	ctx, cancel := context.WithCancel(context.Background())
	a := tui.NewApp(f, tui.WithBackend(tb))
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("App.Run did not return within 3s after cancel")
		}
	}()

	time.Sleep(50 * time.Millisecond)
	s := tb.String()
	if len(s) == 0 {
		t.Error("expected rendered output in footer")
	}
}
