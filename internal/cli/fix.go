package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// enhancer abstracts the enhance service for testability.
type enhancer interface {
	Enhance(text string) (string, error)
}

func runFix(args []string, stdout io.Writer, stderr io.Writer) error {
	settings, err := initSettings()
	if err != nil {
		return fmt.Errorf("loading settings: %w", err)
	}

	enhanceSvc := newEnhanceService(settings)
	return runFixWith(args, stdout, stderr, enhanceSvc)
}

func runFixWith(args []string, stdout io.Writer, stderr io.Writer, svc enhancer) error {
	fs := flag.NewFlagSet("fix", flag.ContinueOnError)
	fs.SetOutput(stderr)
	filePath := fs.String("f", "", "Input file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	inlineText := strings.Join(fs.Args(), " ")
	text, err := readInput(*filePath, inlineText, stdinIfPiped())
	if err != nil {
		return err
	}

	result, err := svc.Enhance(text)
	if err != nil {
		return fmt.Errorf("enhance failed: %w", err)
	}

	fmt.Fprintln(stdout, result)
	return nil
}
