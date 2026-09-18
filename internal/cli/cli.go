package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"keylint/internal/features/enhance"
	"keylint/internal/features/settings"
	"keylint/internal/logger"
)

// validLogLevels enumerates accepted --log values.
var validLogLevels = map[string]bool{
	"off":     true,
	"trace":   true,
	"debug":   true,
	"info":    true,
	"warning": true,
	"error":   true,
}

// addLogFlag registers the --log flag on the given FlagSet.
func addLogFlag(fs *flag.FlagSet) *string {
	return fs.String("log", "off", "Log level: off|trace|debug|info|warning|error")
}

// initLogger validates the level and initialises the logger.
// Sensitive is always false in CLI mode.
func initLogger(level string) error {
	if !validLogLevels[level] {
		return fmt.Errorf("invalid log level %q — must be one of: off, trace, debug, info, warning, error", level)
	}
	logger.Init(level, false)
	return nil
}

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

// stdinFirstByteTimeout bounds the wait for the FIRST byte on stdin.
//
// Once something arrives the rest is read without a deadline: a slow producer is
// still a producer. What this guards against is the opposite — a pipe that is
// open and silent, which makes the command hang with no output and no
// explanation, looking like the AI call is slow.
const stdinFirstByteTimeout = 2 * time.Second

// readInput returns text from the first source the caller actually asked for:
//
//  1. -f <file>
//  2. inline text
//  3. stdin, and only when neither of the above was given
//
// Stdin comes last on purpose. It used to come second, so `KeyLint -fix "some
// text"` run with anything attached to stdin — a shell wrapper, an editor's
// run pane, a CI step — read that instead of the text on the command line, and
// blocked when nothing ever arrived. An argument the user typed is not
// ambiguous; it wins.
func readInput(filePath, inlineText string, stdinReader io.Reader) (string, error) {
	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("reading input file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	if inlineText != "" {
		return inlineText, nil
	}
	if stdinReader != nil {
		text, err := readStdin(stdinReader)
		if err != nil {
			return "", err
		}
		if text != "" {
			return text, nil
		}
	}
	return "", fmt.Errorf("no input provided — use -f <file>, pipe to stdin, or pass text as argument")
}

// readStdin reads everything on stdin, refusing to wait forever for a pipe that
// never speaks.
func readStdin(r io.Reader) (string, error) {
	type result struct {
		data []byte
		err  error
	}
	// Buffered, so the goroutine can finish and be collected even when this
	// function has already returned on the timeout.
	first := make(chan result, 1)
	go func() {
		buf := make([]byte, 1)
		n, err := r.Read(buf)
		first <- result{data: buf[:n], err: err}
	}()

	var head result
	select {
	case head = <-first:
	case <-time.After(stdinFirstByteTimeout):
		return "", fmt.Errorf(
			"nothing arrived on stdin within %s — pass the text as an argument or use -f <file>",
			stdinFirstByteTimeout)
	}

	if head.err != nil && head.err != io.EOF {
		return "", fmt.Errorf("reading stdin: %w", head.err)
	}
	if head.err == io.EOF && len(head.data) == 0 {
		return "", nil
	}

	// Something is flowing; read the rest at whatever pace it comes.
	rest, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}
	return strings.TrimSpace(string(head.data) + string(rest)), nil
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

// initSettings creates a settings service for CLI use.
func initSettings() (*settings.Service, error) {
	return settings.NewService()
}

// newEnhanceService creates an enhance service for CLI use.
func newEnhanceService(s *settings.Service) *enhance.Service {
	return enhance.NewService(s)
}
