package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/google/subcommands"
	"github.com/yongjohnlee80/editor"
)

// VersionName is the verb. `editor --version` is wired to the same output.
const VersionName = "version"

// CmdVersion prints the greeting, the release and the commit it was built from.
type CmdVersion struct{}

func (c *CmdVersion) Name() string { return VersionName }

func (c *CmdVersion) Synopsis() string {
	return "Print the version, the commit it was built from, and say hello."
}

func (c *CmdVersion) Usage() string {
	return `version:
  Print the greeting, release, commit hash, build time, Go version and
  platform. "editor --version" prints the same thing.

`
}

// SetFlags takes none: version reads no configuration, so a -config here would
// be a flag that does nothing.
func (c *CmdVersion) SetFlags(*flag.FlagSet) {}

func (c *CmdVersion) Execute(context.Context, *flag.FlagSet, ...any) subcommands.ExitStatus {
	fmt.Fprint(os.Stdout, editor.ReadBuildInfo().String())
	return subcommands.ExitSuccess
}
