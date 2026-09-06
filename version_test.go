package editor

import (
	"strings"
	"testing"
)

// --version must report the release, a commit and a greeting. Asserted on the
// rendered output because that string is the product: a caller reads it.
func TestBuildInfo_StringReportsEverything(t *testing.T) {
	t.Parallel()
	got := BuildInfo{
		Version: "v1.2.3", Commit: "0123456789abcdef0123", Time: "2026-09-07T00:00:00Z",
		GoVer: "go1.27.0", Platform: "linux/amd64",
	}.String()

	for _, want := range []string{
		"editor", "Happy editing!", // the greeting
		"v1.2.3",       // the version
		"0123456789ab", // the SHORT commit
		"2026-09-07T00:00:00Z", "go1.27.0", "linux/amd64",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("--version output is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "uncommitted") {
		t.Error("a clean build must not be reported as dirty")
	}
}

// A DIRTY build says so. Reporting the hash alone would name a tree that was
// never built.
func TestBuildInfo_DirtyIsStated(t *testing.T) {
	t.Parallel()
	got := BuildInfo{Version: "dev", Commit: "abc123", Dirty: true}.String()
	if !strings.Contains(got, "uncommitted changes") {
		t.Errorf("a dirty build must say so:\n%s", got)
	}
}

// A short or absent commit is left intact rather than sliced into nonsense.
func TestBuildInfo_ShortCommitDoesNotTruncateUnknown(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"unknown", "abc", ""} {
		if got := (BuildInfo{Commit: in}).ShortCommit(); got != in {
			t.Errorf("ShortCommit(%q) = %q, want it unchanged", in, got)
		}
	}
	long := "0123456789abcdefghij"
	if got := (BuildInfo{Commit: long}).ShortCommit(); got != "0123456789ab" {
		t.Errorf("ShortCommit(%q) = %q, want 12 characters", long, got)
	}
}

// ReadBuildInfo never fails: a binary with no VCS data must still say what it
// is, with "unknown" rather than an empty field.
func TestReadBuildInfo_AlwaysAnswers(t *testing.T) {
	t.Parallel()
	bi := ReadBuildInfo()
	if bi.Version == "" || bi.Commit == "" || bi.Time == "" || bi.GoVer == "" || bi.Platform == "" {
		t.Errorf("ReadBuildInfo left a field empty: %+v", bi)
	}
}
