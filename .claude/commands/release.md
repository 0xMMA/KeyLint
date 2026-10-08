# Release — Tag & Publish

Create a version tag on `main` and push it to trigger the GitHub release CI workflow.
This command is run **after** a PR has been merged to `main`.

## Context

- **Version format:** `v<major>.<minor>.<patch>[-suffix]` — the `v` prefix is required
- **Suffixes (pre-release):** `-alpha`, `-beta`, `-rc1` (omit for a stable release)
- **Release trigger:** pushing a `v*` tag fires `release.yml`, which:
  - Builds Linux + Windows binaries
  - Generates `latest.json` for the in-app updater
  - Creates a **draft** GitHub Release — must be published manually
- **Bump guidance from conventional commits:**
  - `feat!` or body contains `BREAKING CHANGE` → major
  - `feat:` / `feat(` (no `!`) → minor
  - `fix`, `chore`, `docs`, `refactor`, `test` only → patch

## Steps

1. **Pre-flight** — stop immediately if any check fails:
   - Confirm current branch is `main` (`git branch --show-current`)
   - Confirm clean working tree (`git status --porcelain` must be empty)
   - Fetch remote and confirm HEAD matches `origin/main`:
     `git fetch origin main --quiet && git rev-parse HEAD` vs `git rev-parse origin/main`
   - If diverged: tell the user to pull or resolve, then stop

2. **Detect current version**
   - Run `git describe --tags --abbrev=0 --exclude 'v0.0.0-*' 2>/dev/null || echo "none"` to get the last tag (the exclude skips the moving dev-channel tags `v0.0.0-main` / `v0.0.0-pr.N`, which are not releases)
   - If "none": no previous tags — skip bump math, go to step 4

3. **Analyze commits since last tag**
   - Run `git log <last-tag>..HEAD --oneline`
   - Check for breaking changes: any line containing `feat!` or `BREAKING CHANGE`
   - Check for features: any line starting with `feat:` or `feat(`
   - Derive the recommended bump type (highest applies):
     - Breaking → major
     - Feature → minor
     - Otherwise → patch
   - Compute all three candidate versions by splitting the last tag (strip `v`):
     e.g. last = `v4.0.1` → patch=`v4.0.2`, minor=`v4.1.0`, major=`v5.0.0`

4. **Ask the user to choose**
   - Use `AskUserQuestion` with **two questions**:

   **Q1 — Bump type:** Show all three candidates; mark the recommended one.
   If no previous tag, offer `v1.0.0` as the only option.

   **Q2 — Pre-release suffix:**
   - Stable release (no suffix)
   - `-alpha` — early preview
   - `-beta` — feature-complete, public testing
   - `-rc1` — release candidate

   - Combine: e.g. minor bump + `-alpha` → `v4.1.0-alpha`
   - If user selects "Other" on Q1, validate the input matches `v\d+\.\d+\.\d+.*`
     and re-ask if invalid

5. **Confirm before acting** — this is irreversible:
   - Print the exact tag that will be created and pushed
   - Remind the user what CI will do:
     > `release.yml` will build Linux + Windows binaries and open a draft release on GitHub.
   - Use `AskUserQuestion` to get explicit confirmation:
     "Create and push tag `<version>`?" — Yes / No (stop)
   - If No: stop cleanly, no changes made

6. **Tag and push**
   - `git tag <version>`
   - `git push origin <version>`
   - If push fails: delete the local tag (`git tag -d <version>`), report the error, and stop
   - Never force-push tags

7. **Report outcome**
   - Print the tag name
   - Link to GitHub Actions: `https://github.com/0xMMA/KeyLint/actions`
   - Link to Releases page: `https://github.com/0xMMA/KeyLint/releases`
   - Remind: "Once CI completes, open the draft release and click Publish."

## Notes
- Stop and report clearly if any step fails — never skip a failure silently
- Never tag from a branch other than `main`
- Never use `--force` on tags
- If $ARGUMENTS is provided, treat it as the target version string (skip questions Q1/Q2,
  go straight to step 5 confirmation with that version)
