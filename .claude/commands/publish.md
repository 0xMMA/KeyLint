# Publish a GitHub Draft Release

Polish and publish a draft GitHub Release: rewrite the auto-generated notes into human-readable form, confirm, then flip draft → published.

---

## Step 1 — List draft releases

Run:
```
gh release list --json tagName,isDraft,isPrerelease,name,createdAt \
  --jq '[.[] | select(.isDraft==true)]'
```

- If the result is empty: report "No draft releases found." and stop.
- If `$ARGUMENTS` is non-empty, treat it as the tag name — skip Steps 1–2 and jump to Step 3.

---

## Step 2 — Select which draft to publish

- If exactly one draft exists, auto-select it and confirm the choice in text (no prompt needed).
- If multiple drafts exist (up to 4 shown), use `AskUserQuestion` to let the user pick by tag name.

---

## Step 3 — Fetch current release details

Run:
```
gh release view <tag> --json tagName,name,body,isPrerelease,assets
```

Read and note: tag name, release title, current body/notes, `isPrerelease` flag, and asset file names.

---

## Step 4 — Rewrite the release notes

Derive the previous tag:
```
git describe --tags --abbrev=0 --exclude 'v0.0.0-*' <tag>^
```
(`--exclude` skips the dev-channel tags `v0.0.0-main` / `v0.0.0-pr.N`, which are not releases.)

Fetch commits since the previous tag:
```
git log <prev-tag>..<tag> --oneline
```

**Filter commits** — include only user-visible changes:
- ✅ Keep: `feat`, `fix`, `perf`, `refactor` that affects behaviour or UI
- ❌ Omit: `chore`, `docs`, `test`, `ci`, CI/workflow changes, icon/branding-only changes, README edits, dependency bumps with no user-visible effect

Rewrite the notes using this structure:
```markdown
## What's new

- **Feature name** — plain-English description of what it does for the user.

## What's fixed

- **Bug description** — what was wrong and what was corrected.

**Full Changelog:** https://github.com/0xMMA/KeyLint/compare/<prev-tag>...<tag>
```

Rules:
- Omit a section entirely if it has no entries (e.g. no fixes → no `## What's fixed`).
- No "Installing" block — GitHub shows assets directly on the release page.
- Keep tone concise and user-facing — no developer jargon or raw commit syntax.

---

## Step 5 — Show draft and confirm

Print the proposed release notes in full, then use `AskUserQuestion`:

> Publish `<tag>` with these notes?
> - **Yes** — update notes and publish
> - **Edit notes myself** — stop here; user edits manually with `gh release edit <tag>` then re-runs `/publish`
> - **No** — stop, leave as draft

Confirmation is mandatory — never auto-publish without it.

---

## Step 6 — Update notes and publish

Run:
```
gh release edit <tag> \
  --notes "<rewritten notes>" \
  --draft=false
```

- Add `--prerelease` only if `isPrerelease` was `true` on the draft.
- Omit `--latest` — GitHub sets this automatically for stable releases.

If `gh release edit` fails, report the error and stop — do not leave the release in a broken state.

---

## Step 7 — Report outcome

Print the published tag and the release URL:
```
https://github.com/0xMMA/KeyLint/releases/tag/<tag>
```
