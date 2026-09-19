// Command claudestub stands in for the Claude Code CLI in tests. It records the
// argv, stdin and environment it was given, then behaves as the CLAUDESTUB_*
// variables tell it to. It is a Go program rather than a shell script so the
// provider's spawn path is exercised on Windows too, which is the platform
// KeyLint ships to.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	// A re-exec of this binary standing in for the node process the real npm
	// shim spawns: it outlives its parent unless the whole tree is killed.
	if os.Getenv("CLAUDESTUB_ROLE") == "grandchild" {
		time.Sleep(time.Second)
		writeFile(os.Getenv("CLAUDESTUB_MARKER"), "alive")
		return
	}

	record()

	// `claude --version` and `claude auth status` are the health probes.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			if v := os.Getenv("CLAUDESTUB_VERSION"); v != "" {
				fmt.Println(v)
				return
			}
			os.Exit(1)
		case "auth":
			if a := os.Getenv("CLAUDESTUB_AUTH"); a != "" {
				fmt.Println(a)
				return
			}
			os.Exit(1)
		}
	}

	if out := os.Getenv("CLAUDESTUB_STDOUT"); out != "" {
		fmt.Fprintln(os.Stdout, out)
	}
	if errOut := os.Getenv("CLAUDESTUB_STDERR"); errOut != "" {
		fmt.Fprintln(os.Stderr, errOut)
	}

	if os.Getenv("CLAUDESTUB_HANG") == "1" {
		spawnGrandchild()
		time.Sleep(5 * time.Minute)
	}

	if code := os.Getenv("CLAUDESTUB_EXIT"); code != "" && code != "0" {
		os.Exit(exitCode(code))
	}
}

// record dumps what the provider handed us, so a test can assert on the exact
// command line, the piped prompt and the environment the CLI would see.
func record() {
	writeFile(os.Getenv("CLAUDESTUB_ARGV_FILE"), strings.Join(os.Args[1:], "\n"))
	writeFile(os.Getenv("CLAUDESTUB_ENV_FILE"), strings.Join(os.Environ(), "\n"))
	if path := os.Getenv("CLAUDESTUB_STDIN_FILE"); path != "" {
		data, _ := io.ReadAll(os.Stdin)
		writeFile(path, string(data))
	}
}

// spawnGrandchild starts a detached-looking child with no inherited pipes, so
// only a tree or process-group kill takes it down.
func spawnGrandchild() {
	if os.Getenv("CLAUDESTUB_MARKER") == "" {
		return
	}
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "CLAUDESTUB_ROLE=grandchild")
	_ = cmd.Start()
}

func writeFile(path, content string) {
	if path == "" {
		return
	}
	_ = os.WriteFile(path, []byte(content), 0o600)
}

func exitCode(s string) int {
	code := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 1
		}
		code = code*10 + int(r-'0')
	}
	if code == 0 {
		return 1
	}
	return code
}
