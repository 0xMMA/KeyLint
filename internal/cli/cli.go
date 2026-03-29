package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
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

// readInput returns text from the first available source:
// 1. File path (if filePath is non-empty)
// 2. Stdin (if stdinReader is non-nil)
// 3. Inline string (if inlineText is non-empty)
// Returns an error if no input is provided.
func readInput(filePath, inlineText string, stdinReader io.Reader) (string, error) {
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("reading input file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	if stdinReader != nil {
		data, err := io.ReadAll(stdinReader)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		text := strings.TrimSpace(string(data))
		if text != "" {
			return text, nil
		}
	}
	if inlineText != "" {
		return inlineText, nil
	}
	return "", fmt.Errorf("no input provided — use -f <file>, pipe to stdin, or pass text as argument")
}

// stdinIfPiped returns os.Stdin if it is connected to a pipe, nil otherwise.
func stdinIfPiped() io.Reader {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil
	}
	if stat.Mode()&os.ModeCharDevice == 0 {
		return os.Stdin
	}
	return nil
}
