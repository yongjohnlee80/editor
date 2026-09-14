package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// OS command tests are split into two tiers:
//
//  1. Base I/O layer (readInto / writeFrom) — tested with in-memory readers
//     and writers; zero file system access, zero TUI.
//  2. Command layer (NewReadFileCmd / NewWriteFileCmd) — tested against a
//     real temp directory to prove path resolution, dir creation, and the
//     "no name" prompt path all work end-to-end.
//
// Both tiers use mockDocument as a stand-in for EditorPane. The mock
// implements Document without any widget dependency, which keeps these tests
// fast and self-contained.

// ─── mockDocument ────────────────────────────────────────────────────────────

// mockDocument is a minimal Document implementation for tests. It records
// every mutation so tests can assert on the document's final state without
// mounting a TUI.
type mockDocument struct {
	value string
	path  string
	dirty bool
}

func (m *mockDocument) Value() string     { return m.value }
func (m *mockDocument) SetValue(v string) { m.value = v }
func (m *mockDocument) Path() string      { return m.path }
func (m *mockDocument) SetPath(p string)  { m.path = p }
func (m *mockDocument) MarkDirty()        { m.dirty = true }
func (m *mockDocument) MarkClean()        { m.dirty = false }

// ─── Base I/O: readInto ──────────────────────────────────────────────────────

func TestReadInto_LoadsContentAndClearsDocument(t *testing.T) {
	doc := &mockDocument{dirty: true}
	r := strings.NewReader("hello\nworld\n")

	lines, err := readInto(doc, r)
	if err != nil {
		t.Fatalf("readInto: %v", err)
	}
	if doc.value != "hello\nworld\n" {
		t.Errorf("value = %q, want %q", doc.value, "hello\nworld\n")
	}
	if doc.dirty {
		t.Error("dirty must be false after readInto")
	}
	if lines != 2 {
		t.Errorf("lines = %d, want 2", lines)
	}
}

func TestReadInto_EmptyReaderGivesZeroLines(t *testing.T) {
	doc := &mockDocument{}
	lines, err := readInto(doc, strings.NewReader(""))
	if err != nil {
		t.Fatalf("readInto empty: %v", err)
	}
	if lines != 0 {
		t.Errorf("lines = %d, want 0 for empty reader", lines)
	}
}

// ─── Base I/O: writeFrom ─────────────────────────────────────────────────────

func TestWriteFrom_WritesContentAndAppendsTrailingNewline(t *testing.T) {
	doc := &mockDocument{value: "hello\nworld"} // no trailing newline
	var buf strings.Builder

	lines, err := writeFrom(doc, &buf)
	if err != nil {
		t.Fatalf("writeFrom: %v", err)
	}
	if got := buf.String(); got != "hello\nworld\n" {
		t.Errorf("written = %q, want trailing newline appended", got)
	}
	if lines != 2 {
		t.Errorf("lines = %d, want 2", lines)
	}
}

func TestWriteFrom_PreservesExistingTrailingNewline(t *testing.T) {
	doc := &mockDocument{value: "hello\n"}
	var buf strings.Builder

	_, err := writeFrom(doc, &buf)
	if err != nil {
		t.Fatalf("writeFrom: %v", err)
	}
	// Must NOT double the newline.
	if got := buf.String(); got != "hello\n" {
		t.Errorf("written = %q, want exactly one trailing newline", got)
	}
}

func TestWriteFrom_EmptyBufferWritesNothing(t *testing.T) {
	doc := &mockDocument{value: ""}
	var buf strings.Builder

	lines, err := writeFrom(doc, &buf)
	if err != nil {
		t.Fatalf("writeFrom empty: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("written = %q, want empty for empty buffer", buf.String())
	}
	if lines != 0 {
		t.Errorf("lines = %d, want 0 for empty buffer", lines)
	}
}

// ─── Command: NewWriteFileCmd ─────────────────────────────────────────────────

func TestNewWriteFileCmd_WritesToDiskAndAdoptsPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "out.txt")
	doc := &mockDocument{value: "hello\nworld", dirty: true}

	cmd := NewWriteFileCmd(doc)
	resp := cmd(nil, p)
	if resp.Status() != StatusOK {
		t.Errorf("Status = %v, want OK", resp.Status())
	}
	// The path must be adopted for subsequent bare ":w".
	if doc.path != p {
		t.Errorf("doc.path = %q, want %q", doc.path, p)
	}
	// The dirty flag must be cleared.
	if doc.dirty {
		t.Error("dirty must be false after a successful write")
	}
	// The file must exist on disk with a trailing newline.
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.HasSuffix(string(b), "\n") {
		t.Errorf("file content = %q, want trailing newline", b)
	}
	// The summary must mention "written".
	if !strings.Contains(resp.Result(), "written") {
		t.Errorf("summary = %q, want it to say 'written'", resp.Result())
	}
}

func TestNewWriteFileCmd_CreatesParentDirectories(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "dir", "out.txt")
	doc := &mockDocument{value: "content"}

	cmd := NewWriteFileCmd(doc)
	resp := cmd(nil, p)
	if resp.Status() != StatusOK {
		t.Fatalf("WriteFileCmd with deep path: %v", resp.Err())
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("file not created in deep directory: %v", err)
	}
}

func TestNewWriteFileCmd_UsesDocPathWhenArgIsEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), "named.txt")
	doc := &mockDocument{value: "data", path: p}

	cmd := NewWriteFileCmd(doc)
	resp := cmd(nil, "") // no arg — falls back to doc.Path()
	if resp.Status() != StatusOK {
		t.Fatalf("WriteFileCmd (no arg): %v", resp.Err())
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected file at doc path: %v", err)
	}
}

func TestNewWriteFileCmd_NoPathAndNoDocPathNeedsPrompt(t *testing.T) {
	doc := &mockDocument{} // unnamed, no path

	cmd := NewWriteFileCmd(doc)
	resp := cmd(nil, "")
	if resp.Status() != StatusPromptNeeded {
		t.Errorf("Status = %v, want PromptNeeded", resp.Status())
	}
	// The prefill must seed "w " so the user only types the filename.
	if resp.Result() != "w " {
		t.Errorf("prefill = %q, want %q", resp.Result(), "w ")
	}
}

// ─── Command: NewReadFileCmd ──────────────────────────────────────────────────

func TestNewReadFileCmd_LoadsFileIntoDocument(t *testing.T) {
	p := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(p, []byte("# Notes\nline two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := &mockDocument{}

	cmd := NewReadFileCmd(doc)
	resp := cmd(nil, p)
	if resp.Status() != StatusOK {
		t.Fatalf("ReadFileCmd: %v", resp.Err())
	}
	if !strings.Contains(doc.value, "# Notes") {
		t.Errorf("doc.value = %q, want file contents", doc.value)
	}
	// The path must be adopted.
	if doc.path != p {
		t.Errorf("doc.path = %q, want %q", doc.path, p)
	}
	// The dirty flag must be clear after a load.
	if doc.dirty {
		t.Error("dirty must be false after ReadFileCmd")
	}
}

func TestNewReadFileCmd_UsesDocPathWhenArgIsEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), "existing.txt")
	if err := os.WriteFile(p, []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := &mockDocument{path: p}

	cmd := NewReadFileCmd(doc)
	resp := cmd(nil, "") // no arg — falls back to doc.Path()
	if resp.Status() != StatusOK {
		t.Fatalf("ReadFileCmd (no arg): %v", resp.Err())
	}
}

func TestNewReadFileCmd_NoPathAndNoDocPathIsRefused(t *testing.T) {
	doc := &mockDocument{} // unnamed

	cmd := NewReadFileCmd(doc)
	resp := cmd(nil, "")
	if resp.Status() != StatusRefused {
		t.Errorf("Status = %v, want Refused for unnamed buffer with no arg", resp.Status())
	}
}

func TestNewReadFileCmd_MissingFileIsRefused(t *testing.T) {
	doc := &mockDocument{}

	cmd := NewReadFileCmd(doc)
	resp := cmd(nil, "/tmp/this-file-does-not-exist-xyz.txt")
	if resp.Status() != StatusRefused {
		t.Errorf("Status = %v, want Refused for a missing file", resp.Status())
	}
}

// ─── EditorPane implements Document ──────────────────────────────────────────

// A compile-time assertion that *EditorPane satisfies the Document interface.
// If any method is missing or has the wrong signature, this line will not compile.
var _ Document = (*EditorPane)(nil)

func TestInitOSCommands_RegistersCommandsInRegistry(t *testing.T) {
	doc := &mockDocument{value: "sample text"}
	reg := NewRegistry()
	InitOSCommands(reg, doc)

	verbs := reg.Verbs()
	found := make(map[string]bool)
	for _, v := range verbs {
		found[v] = true
	}

	for _, expected := range []string{"w", "write", "e", "edit"} {
		if !found[expected] {
			t.Errorf("expected verb %q to be registered by InitOSCommands", expected)
		}
	}
}
