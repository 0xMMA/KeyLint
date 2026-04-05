# Ship Changes (Commit → Push → PR)

Perform a complete git workflow to ship the current changes in this repo.

## Context

- **Branch convention:** `feat/<name>`, `fix/<name>`, `chore/<name>`
- **Target branch:** `main`
- **CI:** `build-linux.yml (Build CI)` runs automatically on PR open (tests + compile)
- **Release:** done separately via `git tag` — do NOT tag here

## Steps

1. **Review changes**
   Run `git status` and `git diff` (staged + unstaged) to understand
   what's changing. Also run `git log --oneline -8` to match commit style.

2. **Run verification** (skip only if tests clearly unrelated to changes)
   - `cd frontend && npm test` — must show 0 failures
   - `go test ./internal/...` — must pass
   - If any Go service files changed: confirm `wire gen` and
     `wails3 generate bindings` were already run (check bindings timestamps)

3. **Stage files**
   Add relevant files. Never stage: `.env`, `bin/`, `frontend/dist/`,
   `wire_gen.go` (only if unchanged), `frontend/test-results/`.

4. **Commit**
   Conventional commit format: `type(scope): description`
   - Types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`
   - Scope examples: `fix`, `enhance`, `settings`, `clipboard`, `tray`,
     `shortcut`, `ci`, `docs`
   - Under 72 chars, imperative mood, no trailing period
   - Match the style from `git log --oneline`

5. **Push**
   `git push origin HEAD`
   Never force push. If push is rejected, report the error and stop.

6. **Open PR**
   - Check first: `gh pr list --head $(git branch --show-current)`
   - If one exists, report its URL and stop
   - Otherwise: `gh pr create` with:
     - Title: same as commit subject
     - Body: what changed, why, and any manual verification steps
     - Include note that `build-linux.yml (Build CI)` CI will run automatically
   - If $ARGUMENTS is provided, use it as extra context for the PR body

## Notes
- Stop and report clearly if any step fails
- Never amend a commit that has already been pushed
- Never skip test verification without explicitly telling the user why
