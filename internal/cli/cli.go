package cli

import (
	"errors"
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

// stdinIdleTimeout is how long stdin may stay quiet before the command gives up.
//
// It is an IDLE timeout, reset by every chunk that arrives — not a deadline on
// the first byte and not one on the whole read. Both of those get it wrong in
// opposite directions: `pdftotext big.pdf - | keylint -fix` can take seconds to
// say anything and must not be cut off, while a pipe that emits one header line
// and then holds the descriptor open would hang forever under a first-byte rule.
//
// Fifteen seconds because this path is only reached when neither -f nor inline
// text was given, so it costs nothing in the common case and leaves room for a
// slow producer.
// A var rather than a const so tests can shorten it: asserting the real value
// twice would add half a minute to every CI run.
var stdinIdleTimeout = 15 * time.Second

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

// readStdin reads everything on stdin, giving up if it falls silent.
//
// The read runs on its own goroutine because there is no way to interrupt a
// blocked Read on an arbitrary reader. On timeout that goroutine is left behind
// holding whatever it has read; the CLI exits immediately afterwards, so it is
// leaked for the length of a process teardown. See the callers — this function
// is reached only from `-fix` and `-pyramidize`, both of which run before Wails
// boots and exit when they are done.
func readStdin(r io.Reader) (string, error) {
	type chunk struct {
		data []byte
		err  error
	}
	// Buffered by one so the reader can always deposit its last chunk and exit,
	// even when nobody is listening any more.
	chunks := make(chan chunk, 1)
	go func() {
		defer close(chunks)
		buf := make([]byte, 32*1024)
		for {
			n, err := r.Read(buf[:])
			if n > 0 {
				out := make([]byte, n)
				copy(out, buf[:n])
				chunks <- chunk{data: out}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					chunks <- chunk{err: err}
				}
				return
			}
		}
	}()

	idle := time.NewTimer(stdinIdleTimeout)
	defer idle.Stop()

	var collected []byte
	for {
		select {
		case c, open := <-chunks:
			if !open {
				return strings.TrimSpace(string(collected)), nil
			}
			if c.err != nil {
				return "", fmt.Errorf("reading stdin: %w", c.err)
			}
			collected = append(collected, c.data...)
			// Progress resets the clock: a slow producer is still a producer.
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(stdinIdleTimeout)

		case <-idle.C:
			if len(collected) > 0 {
				return "", fmt.Errorf(
					"stdin went quiet for %s without closing — the producer is still holding it open",
					stdinIdleTimeout)
			}
			return "", fmt.Errorf(
				"nothing arrived on stdin within %s — pass the text as an argument or use -f <file>",
				stdinIdleTimeout)
		}
	}
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
