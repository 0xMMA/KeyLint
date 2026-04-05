package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"keylint/internal/features/pyramidize"
)

// pyramidizer abstracts the pyramidize service for testability.
type pyramidizer interface {
	Pyramidize(req pyramidize.PyramidizeRequest) (pyramidize.PyramidizeResult, error)
}

func runPyramidize(args []string, stdout io.Writer, stderr io.Writer) error {
	settings, err := initSettings()
	if err != nil {
		return fmt.Errorf("loading settings: %w", err)
	}

	svc := pyramidize.NewService(settings, nil)
	return runPyramidizeWith(args, stdout, stderr, svc)
}

func runPyramidizeWith(args []string, stdout io.Writer, stderr io.Writer, svc pyramidizer) error {
	fs := flag.NewFlagSet("pyramidize", flag.ContinueOnError)
	fs.SetOutput(stderr)

	filePath := fs.String("f", "", "Input file path")
	docType := fs.String("type", "auto", "Document type: auto|email|wiki|memo|powerpoint")
	jsonOut := fs.Bool("json", false, "Output full result as JSON")
	provider := fs.String("provider", "", "AI provider override: claude|openai|ollama")
	model := fs.String("model", "", "Model override (e.g. claude-sonnet-4-6)")
	style := fs.String("style", "professional", "Communication style")
	relationship := fs.String("relationship", "professional", "Relationship level")
	variant := fs.Int("variant", 0, "Prompt variant (0=latest, 1=v1, 2=v2)")
	logLevel := addLogFlag(fs)

	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := initLogger(*logLevel); err != nil {
		return err
	}

	inlineText := strings.Join(fs.Args(), " ")
	text, err := readInput(*filePath, inlineText, stdinIfPiped())
	if err != nil {
		return err
	}

	req := pyramidize.PyramidizeRequest{
		Text:               text,
		DocumentType:       *docType,
		CommunicationStyle: *style,
		RelationshipLevel:  *relationship,
		Provider:           *provider,
		Model:              *model,
		PromptVariant:      *variant,
	}

	result, err := svc.Pyramidize(req)
	if err != nil {
		return fmt.Errorf("pyramidize failed: %w", err)
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Fprintln(stdout, result.FullDocument)
	return nil
}
