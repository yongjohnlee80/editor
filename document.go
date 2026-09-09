package editor

// Document is the minimal interface that buffer implementations must satisfy
// to support file operations and document inspection.
//
// It defines an abstract editing surface decoupled from any concrete UI widgets,
// layout containers, or TUI contexts. This allows OS-level commands (e.g. read/write),
// scratch buffers, in-memory documents, or future test mocks to interact with
// buffers uniformly.
//
// EditorPane implements Document.
type Document interface {
	// Value returns the current buffer text verbatim.
	Value() string

	// SetValue replaces the entire buffer text.
	SetValue(string)

	// Path returns the current on-disk path, or "" for an unnamed buffer.
	Path() string

	// SetPath adopts a new on-disk path for the document.
	SetPath(string)

	// MarkDirty records that the buffer has unsaved changes.
	MarkDirty()

	// MarkClean clears the dirty flag after a successful write or load.
	MarkClean()
}
