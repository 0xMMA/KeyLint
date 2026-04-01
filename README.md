# KeyLint

[![Build Linux](https://github.com/0xMMA/KeyLint/actions/workflows/build-linux.yml/badge.svg)](https://github.com/0xMMA/KeyLint/actions/workflows/build-linux.yml)
[![GitHub release (latest by date including pre-releases)](https://img.shields.io/github/v/release/0xMMA/KeyLint?include_prereleases)](https://github.com/0xMMA/KeyLint/releases)
[![GitHub all releases](https://img.shields.io/github/downloads/0xMMA/KeyLint/total)](https://github.com/0xMMA/KeyLint/releases)
[![License: MIT + Commons Clause](https://img.shields.io/badge/license-MIT%20%2B%20Commons%20Clause-blue.svg)](LICENSE)

**Fix text instantly with AI — one hotkey, perfect writing.**

Perfect for professional emails, chat messages, documents, and social media posts. No more copy-pasting between tools — KeyLint fixes your text instantly, wherever you're typing.

## How It Works

1. **Select** text in any app
2. Press **Ctrl+G**
3. **Fixed.** Your text is corrected and written back — no window, no interruption.

## Features

| Feature | Description |
|---------|-------------|
| **Silent Fix** | Press the hotkey — clipboard text is fixed and written back. No window, no copy-paste loop. |
| **Deep Enhance** | Open the enhancement panel to rewrite for tone, brevity, or formality. Preview before replacing. |
| **BYOK** | Bring your own API key. Keys stay in your OS keyring and never leave your machine. |
| **Multilingual** | Detects the language and corrects without translating. English stays English, German stays German. |
| **System Tray** | Lives quietly in your system tray. Uses minimal resources. |
| **Private** | All API calls go through the local Go backend. No telemetry, no cloud sync, no clipboard history stored. |

## Supported AI Providers

- **OpenAI** (GPT-4o, GPT-4, etc.)
- **Anthropic** (Claude)
- **Ollama** (local, fully offline)
- **AWS Bedrock** (Claude, Titan, etc.)

## Installation

1. Download the latest release from [keylint.io](https://keylint.io) or the [Releases](https://github.com/0xMMA/KeyLint/releases/latest) page
2. Run the installer
3. Launch KeyLint — the welcome wizard walks you through choosing a provider and entering your API key
4. Press **Ctrl+G** on any selected text

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

MIT + Commons Clause — see [LICENSE](LICENSE) for details.

---

<details>
<summary><strong>Development</strong></summary>

### Stack

| Layer | Technology |
|-------|------------|
| Desktop runtime | [Wails v3](https://v3.wails.io/) (Go) |
| Backend language | Go 1.26 |
| Dependency injection | [Wire](https://github.com/google/wire) |
| Frontend | Angular v21 |
| UI components | PrimeNG v21 (Aura preset, orange/zinc theme) |

### Prerequisites

- Go 1.26+
- Node.js 24 LTS
- Wails v3 CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- Linux: `sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev`
- Windows cross-compilation from Linux: `sudo apt install gcc-mingw-w64`

### Build & Run

```bash
# Hot-reload dev server (Angular + Go)
wails3 dev

# Build frontend only
cd frontend && npm run build

# Build production binary (Linux)
go build -tags production -o bin/KeyLint .

# Windows cross-compilation
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
  go build -tags production -ldflags="-w -s -H windowsgui" -o bin/KeyLint.exe .
```

### Testing

```bash
# Frontend unit tests (Vitest)
cd frontend && npm test

# Go unit tests
go test ./internal/...

# E2E tests (requires ng serve on :4200)
npx playwright test
```

### Evaluation (Pyramidize Quality)

The pyramidize feature has an automated eval pipeline that measures output quality against baseline test data using deterministic checks (structure, info coverage, hallucination detection) and LLM-as-judge scoring.

```bash
# Setup: create .env in project root with your API key
echo "ANTHROPIC_API_KEY=sk-ant-..." > .env

# Run eval (uses //go:build eval tag — isolated from normal tests)
EVAL_PROVIDER=claude go test -tags eval ./internal/features/pyramidize/ -v -timeout 600s

# Or use the wrapper script (supports --provider / --model flags)
./scripts/eval.sh --provider claude

# Interactive human review mode (side-by-side comparison)
./scripts/eval-human.sh --provider claude

# Results are logged to test-data/eval-runs/<timestamp>/
# Each run produces: summary.json, results.jsonl, samples/
```

### Wire DI Regeneration

```bash
go install github.com/google/wire/cmd/wire@latest
wire gen ./internal/app/
wails3 generate bindings
```

### Project Structure

```
KeyLint/
├── main.go                         # Entry point — CLI flags, Wails app setup
├── internal/
│   ├── app/                        # Wire DI (wire.go + wire_gen.go)
│   ├── cli/                        # Headless CLI commands (-fix, -pyramidize)
│   └── features/                   # Vertical slices: settings, shortcut, clipboard, tray, enhance, welcome, pyramidize
├── frontend/
│   ├── src/app/
│   │   ├── core/                   # WailsService (bindings bridge), MessageBus, guards
│   │   ├── features/               # fix, text-enhancement, settings, welcome-wizard, dev-tools
│   │   └── layout/                 # Sidebar shell
│   └── bindings/                   # Auto-generated by wails3 generate bindings
├── website/                        # Landing page (GitHub Pages → keylint.io)
└── .github/workflows/
    ├── build-linux.yml             # PR/push → Linux binary artifact
    └── build-windows.yml           # Tag push → Windows .exe + draft release
```

### CI/CD

- **Linux build** — runs on every PR and push to `main`, uploads binary artifact
- **Windows build** — runs on `v*` tag push, cross-compiles `.exe`, creates a draft GitHub release

</details>
