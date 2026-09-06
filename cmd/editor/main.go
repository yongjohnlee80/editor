// Command editor is a small modal text editor on golib/tui.
//
//	editor FILE          open FILE (the default verb)
//	editor open FILE     the same thing, spelled explicitly
//	editor version       print the version and the commit it was built from
//	editor --version     the same thing
//	editor help          list the verbs
//
// Commands follow the lm/cmd/cli-v2 shape: one Cmd* type per verb, registered
// with google/subcommands, sharing flags through an embedded Base.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/google/subcommands"
	"github.com/yongjohnlee80/editor"
)

func main() {
	// --version is a top-level flag as well as a verb, because "--version" is
	// what people type. It is checked before Execute so it works with no verb
	// at all, which is the whole point of it.
	showVersion := flag.Bool("version", false, "print the version and commit, then exit")
	// -config is BOTH a top-level flag and a verb flag. It has to be: the bare
	// form `editor -config X FILE` parses its flags before any verb exists, so
	// a verb-only flag would be rejected as unknown — which it was, while a
	// comment here claimed otherwise.
	topConfig := flag.String("config", "", "path to editor.toml (default: search)")

	open := &CmdOpen{}
	registerCommands(open)

	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Fprint(os.Stdout, editor.ReadBuildInfo().String())
		os.Exit(int(subcommands.ExitSuccess))
	}

	// `editor FILE` — a bare argument that is not a verb is a filename, so the
	// common case needs no verb. Rewriting the argument list is what keeps
	// this ONE code path: `editor FILE` and `editor open FILE` reach the same
	// Execute, rather than a shortcut that can drift from the real command.
	if args := flag.Args(); len(args) > 0 && !isVerb(args[0]) {
		// The top-level -config is handed to the command, so the bare form
		// and the explicit verb reach the same Execute with the same
		// configuration. A verb-level -config still wins if both are given,
		// because SetFlags runs after this.
		open.ConfigPath = *topConfig
		rewriteAsOpen(args)
	}

	os.Exit(int(subcommands.Execute(context.Background())))
}

// registerCommands registers the verb set. It is a function so a test can
// build the same commander the binary does, rather than asserting against a
// list that could drift from what main actually registers.
func registerCommands(open *CmdOpen) {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(open, "")
	subcommands.Register(&CmdVersion{}, "")
}

// isVerb reports whether s names a registered command.
//
// The set is derived by ASKING the commander rather than hard-coding a list,
// so registering a verb cannot forget to teach this about it — a hard-coded
// list would silently treat a new verb as a filename.
func isVerb(s string) bool {
	found := false
	subcommands.DefaultCommander.VisitCommands(func(_ *subcommands.CommandGroup, c subcommands.Command) {
		if c.Name() == s {
			found = true
		}
	})
	return found
}

// rewriteAsOpen re-parses the residual arguments as `open <args>`.
//
// Only the residual arguments are re-parsed: the top-level flags were already
// consumed by flag.Parse, and their values are carried across explicitly by
// the caller rather than being re-parsed here.
func rewriteAsOpen(args []string) {
	_ = flag.CommandLine.Parse(append([]string{OpenName}, args...))
}

func usage() {
	fmt.Fprint(os.Stderr, `editor — a small modal text editor on golib/tui.

Usage:
  editor [-config PATH] [FILE]   open FILE, or an unnamed buffer
  editor open [-config PATH] [FILE]
                                 the same, spelled explicitly
  editor version             print version, commit and a greeting
  editor --version           the same
  editor help [verb]         details for a verb

Flags:
`)
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr)
}
