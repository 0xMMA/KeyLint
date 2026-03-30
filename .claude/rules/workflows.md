# Workflow Rules

## Commit Format

Conventional commits: `type(scope): description`
- Types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`
- Scope examples: `fix`, `enhance`, `settings`, `clipboard`, `tray`, `shortcut`, `ci`, `docs`
- Under 72 chars, imperative mood, no trailing period

## Never Stage

`.env`, `bin/`, `frontend/dist/`, `wire_gen.go` (unless intentionally changed), `frontend/test-results/`

## Release

Merge to `main`, then `git tag vX.Y.Z[-alpha|-beta|-rc] && git push origin <tag>`. Tag push triggers `release.yml` which builds binaries and creates a draft release. → See `.claude/docs/versioning.md` for full pipeline.

## After Go Service Changes

Always run in order: `wire gen ./internal/app/` → `wails3 generate bindings` → build.
