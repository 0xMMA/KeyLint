package cli

import (
	"fmt"
	"io"
)

// Run dispatches a CLI command. args[0] is the command name ("-fix" or "-pyramidize").
// stdout receives the command output; stderr receives error messages.
func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("no command provided")
	}

	switch args[0] {
	case "-fix":
		return runFix(args[1:], stdout, stderr)
	case "-pyramidize":
		return runPyramidize(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown command: %q", args[0])
	}
}
