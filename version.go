package editor

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version is the release, stamped at link time:
//
//	-ldflags "-X github.com/yongjohnlee80/editor.Version=v1.2.3"
//
// It is the ONLY piece of build information that needs stamping. The commit,
// its time and whether the tree was dirty all come from the build itself via
// [debug.ReadBuildInfo], so they are correct for `go build` and `go install`
// alike — not only for a build that went through the Makefile.
var Version = "dev"

// BuildInfo is what --version reports.
type BuildInfo struct {
	Version  string
	Commit   string // full VCS revision, or "unknown"
	Time     string // commit time, or "unknown"
	Dirty    bool   // the working tree had uncommitted changes
	GoVer    string
	Platform string // GOOS/GOARCH
}

// ReadBuildInfo gathers the build stamps. It never fails: a binary built in a
// way that carries no VCS data reports "unknown" rather than refusing to say
// what it is.
func ReadBuildInfo() BuildInfo {
	bi := BuildInfo{
		Version:  Version,
		Commit:   "unknown",
		Time:     "unknown",
		GoVer:    runtime.Version(),
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return bi
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			bi.Commit = s.Value
		case "vcs.time":
			bi.Time = s.Value
		case "vcs.modified":
			bi.Dirty = s.Value == "true"
		}
	}
	return bi
}

// ShortCommit is the first 12 characters of the revision, which is what a
// person reads. It returns the whole string when it is shorter, so an
// "unknown" commit stays legible instead of being truncated to nonsense.
func (b BuildInfo) ShortCommit() string {
	if len(b.Commit) <= 12 {
		return b.Commit
	}
	return b.Commit[:12]
}

// Greeting is the human line --version leads with.
func Greeting() string {
	return "editor — a small modal text editor on golib/tui. Happy editing!"
}

// String renders the full --version report.
func (b BuildInfo) String() string {
	var sb strings.Builder
	fmt.Fprintln(&sb, Greeting())
	fmt.Fprintln(&sb)
	fmt.Fprintf(&sb, "  version   %s\n", b.Version)
	commit := b.ShortCommit()
	if b.Dirty {
		// Said out loud: a dirty build does not correspond to any commit, so
		// reporting the hash alone would name a tree that was never built.
		commit += " (uncommitted changes)"
	}
	fmt.Fprintf(&sb, "  commit    %s\n", commit)
	fmt.Fprintf(&sb, "  built     %s\n", b.Time)
	fmt.Fprintf(&sb, "  go        %s\n", b.GoVer)
	fmt.Fprintf(&sb, "  platform  %s\n", b.Platform)
	return sb.String()
}
