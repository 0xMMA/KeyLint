# Replacing PrimeNG: decision and research

## Decision (Michael, 2026-10-10)

**Angular CDK + Tailwind + our own components, with our own look.** This is the report's fallback, chosen deliberately over its primary (Angular Material). Material's M3 shape (ripples, floating-label fields, Google proportions) stays recognisable however it is themed, and KeyLint should look like its own tool, not a Google app.

- **Colours:** keep the orange primary and the grey surfaces of today's theme (Aura preset, `orange` primary, dark only). Everything else (type, spacing, radii, density, component shapes, motion) gets a modern redesign, done with the `frontend-design` skill.
- **Behaviour and accessibility** come from Google-owned packages only: `@angular/cdk` (overlay, a11y, listbox, menu) and `@angular/aria` where it helps. Aria is developer preview on Angular 21 and stable from 22. Angular 22 stays blocked until PrimeNG is gone, so on 21 we prefer the stable CDK primitives and adopt Aria after the Angular 22 upgrade.
- **Tailwind (v4, MIT)** is accepted against the report's advice. It is a build-time tool and the shipped app contains only generated CSS, so even an abandoned Tailwind cannot break a released build. The vendor risk the report raises (Tailwind Labs' 2026 layoffs) means slower maintenance, not a relicensing lever. It replaces PrimeFlex and carries the design tokens.
- **Icons:** vendored SVGs (Lucide or Material Symbols, picked with the design), no icon package.
- **Migration:** incremental, PrimeNG 21 and the new components coexist; order as in the sketch below, with step 1 becoming "design tokens in Tailwind + coexistence check" instead of the Material theme. Deadline: before Angular 21 LTS ends (2027-06).

## Status: done (2026-10-11, PR #123)

PrimeNG is out. `primeng`, `@primeuix/themes` (and the `@primeuix/*` packages it pulled in), `primeicons` and PrimeFlex are no longer dependencies, `providePrimeNG` and the Aura preset are gone from `app.config.ts`, and so are the PrimeNG overrides in `styles.scss`. `@angular/animations` went too: only PrimeNG needed `provideAnimationsAsync()`, and the CDK overlay does not. Every screen runs on KeyLint's own components in `frontend/src/app/shared/ui/` in the look of [`design.md`](design.md).

**They were removed because PrimeTek relicensed them** (PrimeNG 22, `@primeuix/*` 3.x and primeicons 8 are under the commercial PrimeUI License; see `docs/roadmap.md` E4 step 2). Do not add any PrimeTek package back. Dependabot used to fence the non-MIT versions with explicit `versions:` ranges; that fence went with the packages, so nothing in CI would stop a re-added one.

How it turned out, against the decision above:

- **Behaviour:** the stable CDK primitives on Angular 21 (overlay, portal) under our own `kl-select`, `klTooltip`, `kl-tabs` and the rest; no `@angular/aria` yet. It comes with the Angular 22 upgrade, which PrimeNG no longer blocks (roadmap E4 step 2). That upgrade is done (PR #125, Angular 22.2); `@angular/aria` still waits for a component that needs it.
- **Icons:** Lucide (ISC), the SVG bodies vendored into `shared/ui/icon/icons.ts`; the licence is `frontend/public/LICENSE-lucide.txt`, so it ships in the build. No icon package, no icon font.
- **Fonts:** Schibsted Grotesk and Literata, subset woff2 in `frontend/public/fonts` with their OFL texts; nothing is fetched at runtime.
- **Diff marks** on the Fix page come from `diff` (jsdiff, BSD-3-Clause), word level.
- **Tailwind** carries the `--kl-*` tokens, with only KeyLint's colours as utilities. Its preflight reset was off while PrimeNG was in and is on since the removal; every screen was compared before and after.
- **Tests:** the jsdom `ResizeObserver`/`matchMedia` stubs were PrimeNG's and are gone. `e2e/ui-tokens.spec.ts` now also fails if a `--p-*` token, a `p-` class or element, or an icon font shows up on a screen.

**Size** (production build): at the research baseline (`783f309`) about 1.07 MB of JS, 360 KB of global CSS and 624 KB of icon fonts. Just before the removal (PrimeNG still provided, no longer rendered) 816 KB of JS and 30 KB of global CSS; after it 611 KB of JS (initial 335 KB, was 488 KB), 17 KB of global CSS, no icon fonts, and 0.99 MB on disk for the whole `dist` (was 1.86 MB).

The research below is kept as written. Its scores and recommendation predate the decision.

---

**Checked:** 2026-10-10. Data comes from the npm registry API, the npm downloads API (week 2026-10-02 to 10-08), the GitHub REST API via `gh`, and the pages linked below. The inventory comes from `main` at `783f309`.
**Scope:** research only. No code was changed. One throwaway production build went to the scratchpad to measure a size baseline, and was deleted afterwards.

---

## TL;DR

| | Choice |
|---|---|
| **Primary** | **Angular Material + CDK** (Google, MIT), plus about 4 thin KeyLint components that Material lacks or would fight against (`kl-message`, `kl-tag`, a button loading state, plain styled `input`/`textarea`) |
| **Fallback** | **Angular CDK + `@angular/aria`** (also Google, MIT; Aria is stable since v22) with our own styled components. Same owner, no Material look, more work. |
| Icons | **Material Symbols** (Google, Apache-2.0): **vendor the ~22 SVGs into the repo** and register them with `MatIconRegistry`, so there is no icon npm dependency. Lucide (ISC) is the equal alternative if a thin line look matters more. |
| Layout utilities | **Plain CSS.** KeyLint uses PrimeFlex about 25 times (`mt-*`, `mb-*`, `flex`, `gap-3`, `w-full`), so about 10 lines of own utilities or component SCSS replace it. **No Tailwind.** |
| Effort | **M**, about 5–7 PRs, done incrementally. PrimeNG 21 and Material 21 can coexist during the migration. |
| Deadline driver | Angular 21 LTS ends **2027-06**. PrimeTek is sunsetting PrimeNG LTS for new sales, so 21.x patches may stop earlier. |

---

## Step 1: inventory (what a replacement must cover)

11 non-spec files import PrimeNG. Two of them hold most of the usage: `settings.component.ts` (1,444 lines) and `text-enhancement.component.ts` (1,686 lines).

### Components and directives

| PrimeNG piece | Count | Where (file: count) | Notes |
|---|---|---|---|
| `p-button` | 53 | text-enhancement 23, settings 18, dev-channel 6, welcome-wizard 6, shortcut-recorder 3, dev-tools 2, fix 1, provider-card 1 | `severity` is used heavily (secondary 33, warn/danger/success/info), plus `outlined` ×3, `text` ×1, `size="small"`, icon-only buttons, `[loading]`, and a custom `#content` template (Pyramidize button with a `Ctrl+↵` hint) |
| `p-message` | 24 | settings 16, dev-channel 5, fix 4, text-enhancement 2, provider-card 2, welcome-wizard 1 | inline alerts in warn/error/success/info/secondary. **Material has no equivalent.** |
| `p-select` | 11 | text-enhancement 5, settings 4, active-provider 2 | `optionLabel`/`optionValue`, `appendTo`, `styleClass="w-full"`. **`[editable]` (free text plus a list, i.e. a combobox)** appears in settings and active-provider, with `#item`/`#selectedItem` templates. `model-options.ts` encodes PrimeNG's null-vs-`""` semantics. This is the hardest piece to replace. |
| `p-tag` | 10 | settings 5, dev-channel 2, welcome-wizard 2, provider-card 1 | status pills |
| `p-tabs`/`p-tablist`/`p-tab`/`p-tabpanel(s)` | 2 tab sets, 6 tabs | settings, text-enhancement | the ResizeObserver stub in specs exists because of these |
| `pTooltip` | 11 | text-enhancement 6, shell 5 | shell: tooltips only while the sidebar is collapsed |
| `pInputText` / `pTextarea` | 7 / 5 | settings, text-enhancement, welcome-wizard, fix | the canvas/fix textareas carry their own layout |
| `p-toggle-switch` | 5 | settings | (#26 knob clipping was a PrimeNG styling bug) |
| `p-card` | 3 | dev-tools 2, settings 1 | |
| `p-slider` | 1 | settings | the threshold slider (ADR-002 calls it "dead") |
| `p-progressSpinner` | 1 | text-enhancement | |
| `p-inputnumber` | 1 | text-enhancement | |
| `p-checkbox` | 1 | fix | |
| `providePrimeNG` + `@primeuix/themes` Aura preset (orange primary) | 1 | `app.config.ts` | `darkModeSelector: '.app-dark'`, `cssLayer: false` |

**Not used:** no toast, dialog, confirm dialog, table, menu, MessageService, ConfirmationService or Stepper. The welcome wizard is already a custom `@switch` component, so it only needs buttons, tags, a message and an input.

### Icons (primeicons 7)

There are 22 distinct icons and about 45 usages: times, refresh, check (6 each), sparkles (4), trash, download, copy, arrow-circle-up (2 each), wrench, spinner+spin, send, replay, plus, play, pencil, key, history, **file-word** (a brand-ish icon; Lucide v1 removed brand icons, and Material Symbols has none), cog, clipboard, chevron-right/left/down, bolt. The primeicons font costs about **624 KB** of font files in `media/` (eot/ttf/woff/woff2/svg).

### PrimeFlex

About 25 uses, all trivial: `styleClass` mb-2 ×4, mb-3 ×3, mt-2 ×2, mt-3 ×6, w-full ×2; `class` mt-3/mt-4 ×6, `flex gap-3` ×2. The whole of `primeflex.css` is imported (373 KB minified) and makes up most of the **360 KB global CSS** bundle.

### Hidden coupling (easy to miss)

- **142 `var(--p-*)` references across 11 files**: `--p-text-muted-color` ×27, `--p-primary-color` ×24, `--p-content-border-color` ×18, surface-50…950, `--p-red-400`, `--p-amber-*`, and more. KeyLint's own component styles depend on PrimeNG's token names. A replacement needs a KeyLint token layer first.
- **`styles.scss`**: about 60 lines of `.app-dark` overrides that patch PrimeNG gaps (`.p-tablist`, `.p-inputtext`, `.p-select-label`, disabled states, #28, #37), plus the `<html class="app-dark">` contract documented in `index.html` and `architecture.md`.
- **Tests coupled to PrimeNG DOM**: 7 Vitest specs (~51 references) and 7 Playwright specs (~41 references) use selectors such as `p-tab[value=…]`, `p-tabpanel[data-p-active]`, `.p-select-option`, `.p-select-overlay`, `.p-select-label`, `.p-toggleswitch`, `.p-button-secondary`, `.p-button-loading`, `.p-card`, `pi-chevron-*`. `e2e/dark-mode.spec.ts` asserts on `.p-select*` computed colours. These get rewritten alongside each component, preferably onto `data-testid`/roles, as `testing.md` already asks.
- **Baseline bundle** (prod build, 2026-10-10): about 1.07 MB JS raw across all chunks (main 114 KB), 360 KB global CSS, 624 KB icon fonts. This is a desktop app served from disk, so size matters little. Dropping PrimeFlex and the icon font alone saves about 1 MB on disk.

---

## Step 2: candidates (facts, checked 2026-10-10)

### Facts table

| | Angular Material + CDK | Spartan UI | ng-zorro-antd | Taiga UI | Clarity (`@clr/angular`) | ng-bootstrap |
|---|---|---|---|---|---|---|
| Repo | [angular/components](https://github.com/angular/components) | [spartan-ng/spartan](https://github.com/spartan-ng/spartan) | [NG-ZORRO/ng-zorro-antd](https://github.com/NG-ZORRO/ng-zorro-antd) | [taiga-family/taiga-ui](https://github.com/taiga-family/taiga-ui) | [vmware-clarity/ng-clarity](https://github.com/vmware-clarity/ng-clarity) | [ng-bootstrap/ng-bootstrap](https://github.com/ng-bootstrap/ng-bootstrap) |
| Stars | 25,030 | 2,935 | 9,180 | 4,062 | 431 (old `vmware/clarity` repo had more) | 8,223 |
| Created | 2016-01 | 2023-04 | 2017-08 | 2020-09 | 2022-03 (repo split) | 2015-09 |
| Last push | 2026-10-10 | 2026-10-08 | 2026-10-10 | 2026-10-09 | 2026-10-09 | 2026-06-22 |
| Open issues + PRs | 1,386 | 93 | 788 | 151 | 82 | 442 |
| Commits on default branch, last 12 mo | 1,202 | 1,283 | 300 | 1,948 | 313 | not measured |
| Distinct human authors, 12 mo | 56 | 63 | 32 | 50 | 6 | not measured |
| Top author share, 12 mo | crisbeto 47% (Google team) | spread: top ~19% each across Leonidas, marcjulian, Merlin, ashley-hunter, goetzrobin | Laffery 38% | spread: 5 people at 8–18% each | 2 people = 83% | not measured |
| Latest | 22.2.2 (2026-10-07); v21-lts 21.2.14 | brain 1.6.3 (2026-10-08) | 22.1.2 (2026-10-10) | 5.27.0 (2026-10-05) | 18.4.0 (2026-10-06) | 21.0.0 (2026-06-22) |
| Stable releases, 12 mo | 54 | 23 (1.0 only since 2026-06-24; ~3 years of alphas before) | 22 | 78 | 19 | 2 |
| npm downloads per week | 2.47 M (cdk 4.13 M) | 77 k (brain) | 468 k | 28 k (core) | 30 k | 602 k |
| Licence | MIT | MIT | MIT | Apache-2.0 | MIT (fonts OFL) | MIT |
| Copyright holder | **Google LLC** | **Robin Goetz (individual)** | Alibaba.com | "Acpekt" (unexplained entity; the project originated at Tinkoff / T-Bank) | **Broadcom** | contributors |
| Backing | Angular team at Google, same repo cadence as the framework | volunteers + GitHub Sponsors (to founder) + Zerops infra sponsor | community core team, Alibaba origin | T-Bank internal product, open-sourced | Broadcom (after the VMware acquisition) | community |
| Paid tier / licence key? | none | none found (Blocks appear free) | none found | none found | none | none |
| Angular 21 | yes (21.2.x; peer `^21 \|\| ^22`) | yes (peer `>=21 <23`) | 21.3.3 (`^21` only) | yes (peer `>=19`) | 18.x (`^21.1 \|\| 22`) | 20.0.0 |
| Angular 22 | yes, **same day** (2026-06-03) | yes | 22.0.0 on 2026-07-21 (**+48 days**) | yes (loose peer) | 18.4.0 on 2026-10-06 (**+4 months**) | 21.0.0 on 2026-06-22 (+19 days) |
| Lag on previous majors | 0 days (19, 20, 21) | n/a (pre-1.0) | +31 d (v19), +34 d (v20), +51 d (v21) | loose peer, not measured | v21 support came 2026-05-11 (**~6 months**) | +29 d, +22 d, +23 d |
| Zoneless / signals | yes (the Angular team's reference) | yes, built for it | partial: zoneless fixes in 20.4.x, no headline claim found (unverified) | yes (signal-heavy, not verified in depth) | not verified | not verified |
| Dark theme | M3 `mat.theme()` with `theme-type: dark` and a built-in `$orange-palette`; `--mat-sys-*` tokens; per-component `*-overrides` mixins | Tailwind/shadcn CSS variables, `.dark` class | separate dark stylesheet (Less); CSS-variable build is "experimental" | `tuiTheme="dark"` | yes | Bootstrap 5.3 colour modes |
| Extra deps | none beyond Angular | **Tailwind v4 (required)**, `@angular/cdk`, clsx, tw-animate-css, luxon (peer) | date-fns 2, `@ant-design/icons-angular`, tinycolor, Less for theming | ~10 sibling packages (`@ng-web-apis/*`, polymorpheus, ...) | `@cds/core` (**archived 2026-02**) | Bootstrap CSS, Popper |

Sources: npm registry `registry.npmjs.org/<pkg>`, `api.npmjs.org/downloads/point/last-week/<pkg>`, `gh api repos/<repo>`, `gh api repos/<repo>/commits?since=2025-10-10`, `gh api repos/<repo>/license`, [angular.dev/reference/releases](https://angular.dev/reference/releases), [angular.dev/guide/aria/overview](https://angular.dev/guide/aria/overview), [Spartan 1.0 announcement (2026-06-24)](https://angularindepth.com/article/announcing-spartan-ui-1-0/), [spartan.ng installation](https://www.spartan.ng/documentation/installation), [ng-zorro theming](https://ng.ant.design/docs/customize-theme/en), [PrimeTek "next chapter"](https://primeui.dev/nextchapter).

### Notes per candidate

**Angular Material + CDK.** The copyright is Google's and it ships from the same repo and release train as Angular. Every major has landed on the day Angular shipped, and the `^21 || ^22` peer means adopting it on 21 does not block the move to 22. It has 54 stable releases a year, and long-lived `vNN-lts` dist-tags exist back to v11. Google could deprioritise Material (it pushes Aria for custom design systems), but the relicensing risk is the lowest of all candidates: Google has no paid UI tier to upsell. Gaps for KeyLint: no inline message/alert, no button loading state, no simple tag (chips are heavier), and M3 dropped the `color` input, so `severity="warn/danger/success"` needs our own CSS classes. `mat-form-field` is opinionated (floating label, subscript space), so the plain inputs and the canvas textareas are better kept native with our own CSS. The CDK's shared ResizeObserver guards `typeof ResizeObserver !== 'undefined'`, so Material tabs should not throw in jsdom (seen in `cdk/fesm2022/observers-private.mjs`; not run). Coverage: select → `mat-select` with custom `mat-option` content; editable select → `matAutocomplete` on `matInput`; tabs → `mat-tab-group`; tooltip → `matTooltip`; switch → `mat-slide-toggle`; slider, spinner, checkbox and card all exist.

**`@angular/aria` (part of the fallback).** Headless, accessible directives from the Angular team: Autocomplete, Listbox, Select, Multiselect, Combobox, Menu, Menubar, Toolbar, Accordion, Tabs, Tree, Grid. It was developer preview in v21 and stable in v22 (per secondary coverage of the [v22 announcement](https://blog.angular.dev/announcing-angular-v22-c52bb83a4664); I did not open the blog itself). It follows the same versioning and peers as Material (`^21 || ^22`), with 109 k downloads a week. Stable status only arrives with Angular 22, so on Angular 21 we would build on a developer-preview API.

**Spartan UI.** Spartan is the most serious non-Google option and the one with the nicest model against rug-pulls: the styled layer ("helm") is **copied into our repo** by a CLI generator, so we own that code outright, and only the headless "brain" is an npm dependency. It has 63 human authors over 12 months, no dominant committer, and no paid tier found. Against it under the owner's weighting:
- 1.0 only shipped 2026-06-24, so it has been stable for about 3.5 months.
- The LICENSE copyright is one individual (Robin Goetz), and sponsorship goes to his personal GitHub Sponsors.
- It has no company or foundation (the new OpenNG foundation hosts Spectator/Elf, not Spartan).
- It **requires Tailwind v4**. Tailwind Labs cut 3 of its 4 engineers in January 2026 after revenue fell about 80% ([DevClass](https://www.devclass.com/2026/01/08/tailwind-labs-lays-off-75-percent-of-its-engineers-thanks-to-brutal-impact-of-ai/), [eWeek](https://www.eweek.com/news/tailwind-labs-lays-off-engineers-due-to-ai/)), and it sells Tailwind Plus. Tailwind v4 is MIT and a build-time tool, so a pinned version keeps working; the risk is slower maintenance, not a licence change. Even so, it puts a second small vendor under the UI.

Spartan passes the 1–2k-star bar, so it is not disqualified, but it is not conservative.

**ng-zorro-antd.** Large, MIT, actively released, with about 4–7 weeks of lag per Angular major. Each major pins only its own Angular major (`^21`), so ng-zorro would gate future Angular upgrades by about 1.5 months. Copyright is Alibaba, but development is now community-led (the top committers are not obviously Alibaba staff; not verified). Theming is Less-based, and the CSS-variable/dynamic theme is still labelled "experimental", so a custom orange dark theme is more friction than with Material. The design language is Ant Design (dense, enterprise). It carries old deps (date-fns 2). It is viable, but not better than Material on any of the top three criteria.

**Taiga UI.** Technically strong: Apache-2.0, very active (78 releases a year), 50 authors, a good dark theme, and a loose `>=19` peer. On the owner's weighting, though, its corporate backer is T-Bank (formerly Tinkoff Bank, Russia), which third-party sanctions aggregators list on OFAC's SDN list (since 2023, per [this aggregator](https://sanctions.blackseanews.net/en/entry/4398); **I did not verify this against OFAC directly**). Using public Apache-2.0 code is generally not a sanctions problem (not legal advice), but a backer under sanctions is a continuity and GitHub/npm-access risk that a conservative choice should avoid. The LICENSE copyright line reads "Acpekt", an entity I could not identify. It has a small download base (28 k a week) and a large sibling-package surface.

**Clarity.** See the rejected list.

**ng-bootstrap.** It genuinely qualifies on age (2015), stars (8.2k), licence (MIT) and Angular lag (~3 weeks), but it made only 2 stable releases in 12 months and has no select/switch component (native elements plus Bootstrap CSS). It also brings Bootstrap's global CSS and look. It is a respectable choice, but Material covers more of the inventory with better backing.

**Build-it-yourself baseline (CDK + own thin components).** This gives the smallest dependency surface (only Google packages) and total control of the look, which fits a dark-only, eight-screen app. The cost is writing and testing roughly 12 components, where select/combobox, tabs and tooltip are the hard ones for accessibility. `@angular/aria` largely removes that hard part from v22 on, which is why it is the fallback.

### Icon sets

| Set | Owner | Licence | Stars | Activity (12 mo) | Angular package | Notes |
|---|---|---|---|---|---|---|
| **Material Symbols** | Google ([google/material-design-icons](https://github.com/google/material-design-icons), 54k★) | Apache-2.0 | 54,098 | active | `mat-icon` (font ligatures or SVG via `MatIconRegistry`) | The npm packagings (`material-symbols`, `@material-symbols/svg-400`) are **one person's** (marella), so vendor the SVGs from Google's repo instead. Has every icon on the list except a Word logo (use `description`/`article`). |
| **Lucide** | community ("Lucide Icons and Contributors"; Feather fork) | ISC (+MIT for Feather-derived icons) | 24,929 | 694 commits, 170 authors | `@lucide/angular` 1.55 (new, signals/zoneless, Angular ≥17); the legacy `lucide-angular` peers only up to 21 | npm publishing is a single account (ericfennis). v1 dropped brand icons. Its thin line look is closest to primeicons. |
| Tabler Icons | Paweł Kuna (individual) | MIT | 22,190 | 182 of 221 commits by one person | `@tabler/icons-angular` (2.8k/wk) | Paid "premium" formats exist alongside the free MIT set, which is an open-core signal. Bus factor 1. |
| `@ng-icons/*` | individual | MIT | 602 | | wrapper over many sets | **rejected** (under the star bar, wrapper only) |

**Recommendation:** with only 22 icons, **vendor the SVG files into the repo** (e.g. `frontend/src/assets/icons/` or a TS map registered with `MatIconRegistry.addSvgIconLiteral`). Once copied, the files keep the licence they were copied under. That removes the icon library as a dependency completely, along with the 624 KB font. Use Material Symbols (Google, Apache-2.0) for consistency with Material. If the owner prefers the current thin line look, vendor Lucide instead; the risk profile is the same once vendored.

### Layout utilities (PrimeFlex replacement)

PrimeFlex 4.0.0 is MIT and frozen (0 releases in 12 months). It is not relicensed, but it is dead weight. KeyLint's usage is about 25 class uses from 8 classes. **Replace them with plain CSS**: a few `.mt-2/.mt-3/.mt-4/.mb-2/.mb-3/.flex/.gap-3/.w-full` rules in `styles.scss`, or better, move them into the component SCSS that already exists. Tailwind would add a build dependency on a financially strained vendor to replace 8 classes. That only makes sense if Spartan is chosen, because Spartan forces it.

---

## Weighted comparison

Scores run 1–5 (5 = best). Weights follow the owner's order: longevity ×5, licence risk ×4, Angular tracking ×3, coverage/dark/a11y ×2, migration effort/bundle ×1.

| Criterion (weight) | Material + CDK | CDK + Aria + own | Spartan | ng-zorro | Taiga | ng-bootstrap |
|---|---|---|---|---|---|---|
| Longevity and backing (×5) | **5**: Google, 10 years, 56 authors, lockstep releases | **5**: same repo and owner | 3: active and spread, but individual copyright, no company, stable only 3.5 months | 4: 9 years, active, Alibaba origin, community-run | 3: active, but sanctioned corporate backer | 3: old, but 2 releases a year |
| Licence / relicensing risk (×4) | **5**: Google, MIT, no paid UI tier | **5** | 3: MIT and helm code is owned, but individual copyright plus a Tailwind Labs dependency | 4: MIT, Alibaba, no paid tier | 3: Apache-2.0; copyright entity unclear | 4: MIT, community |
| Angular tracking (×3) | **5**: same day, peer spans next major | 4: Aria stable only from v22 | 4: `>=21 <23`, short history | 3: +4–7 weeks, pins one major | 4: loose peer | 4: ~3 weeks |
| Coverage, dark theme, a11y (×2) | 4: lacks message/tag/loading; M3 dark theme is good | 3: we build the visuals | **5**: covers all of it incl. alert/badge | 4 | 4 | 3: no select/switch |
| Migration effort / bundle (×1) | 4 (M) | 2 (L) | 3 (M–L, adds Tailwind and copied code) | 3 (M, Less theming) | 2 (M–L, different idioms) | 3 (M) |
| **Weighted total (max 75)** | **72** | **65** | **52** | **56** | **49** | **52** |

The fallback scores high on risk because it carries the same backing as the primary. It loses on effort and on the v22-only stability of Aria.

---

## Rejected, with reasons

| Candidate | Reason |
|---|---|
| **Clarity** (`@clr/angular`) | Its 431 stars are below the bar. Over 12 months 6 human authors contributed, and 2 of them made 83% of commits. `@cds/core` (the web-component core) was **archived 2026-02**. Copyright is Broadcom, whose post-VMware open-source commitment is an open question. Angular 21 support arrived about 6 months after Angular 21. |
| **ngx-bootstrap** | Company-owned (Valor Software) with 616 open issues. ng-bootstrap is the better Bootstrap option, and neither beats Material. |
| **`@ng-icons/*`** | 602 stars, single maintainer, and only a wrapper. |
| **Tabler Icons** (as primary icon set) | Bus factor 1 (83% of commits from the owner), and a paid premium tier exists. Fine to vendor a few SVGs, but not to depend on. |
| **Taiga UI** | Not disqualified by the numbers, but rejected under criterion 2: sanctioned corporate backer (unverified with OFAC) and an unexplained copyright holder. |
| **PrimeNG Community key** | Out of scope by the owner's decision (roadmap E4, 2026-10-10): yearly renewal, banner without the key, and the key must stay out of the repo. |
| **Tailwind** (standalone, as PrimeFlex replacement) | Overkill for 8 classes, and it adds a vendor that lost about 80% of revenue in 2026. Acceptable only as a Spartan prerequisite. |

---

## Recommendation

**Primary: Angular Material + CDK 21.2.x now, then move to 22 alongside Angular.**
1. Of all candidates it has the lowest relicensing risk. The copyright belongs to Google, which has no paid UI tier to upsell. It ships from the Angular team's own repo.
2. It is never late for a new Angular major, and its peer range already includes 22, so it unblocks the Angular 22 upgrade instead of gating it.
3. It covers all hard widgets (select, combobox via autocomplete, tabs, tooltip, slider, toggle) with Google-maintained accessibility. The gaps are small and are exactly the pieces that are trivial to own.

**Own thin components** (in `frontend/src/app/shared/ui/` or similar, each ≤ ~60 lines):
- `kl-message` replaces `p-message` (24 uses), with severities info/success/warn/error/secondary
- `kl-tag` replaces `p-tag` (10 uses)
- a button loading state: either a tiny directive that swaps the icon for a `mat-progress-spinner`, or a `kl-button` wrapper around `matButton` that also maps `severity` to classes
- native `input`/`textarea` with KeyLint CSS where `mat-form-field` would change the layout

**Fallback: Angular CDK + `@angular/aria` + own styled components.** Use it if Material's M3 look or `mat-form-field`/override friction turns out to be more work than writing the visuals ourselves. Owner and licence are identical, so this costs effort, not risk. Do it after Angular 22, when Aria is stable.

**Not recommended, but credible:** Spartan UI. It is worth revisiting in a year if it gains an organisation or foundation and holds a steady 1.x cadence.

**Icons:** vendor about 22 Material Symbols SVGs (Lucide as the equal-risk alternative). **Layout:** plain CSS, PrimeFlex removed.

---

## Migration sketch (incremental, coexisting)

**Coexistence works on paper.** PrimeNG 21.1.10 peers `@angular/cdk ^21.0.0`, and Material 21.2.14 needs `@angular/cdk 21.2.14` exactly, which satisfies both. Both are standalone imports, so components can migrate file by file. PrimeNG injects its theme at runtime (`cssLayer: false`), while Material's theme is a compile-time SCSS mixin, so they do not share class names (`.p-*` vs `.mat-*`/`.mdc-*`). Both append overlays to `body`, so z-index stacking between a PrimeNG overlay and a CDK overlay on the same screen needs a look. **This is not tried in a build;** step 1 verifies it.

Order (one PR each, all green and through `review-pr` before the next):

1. **Token layer and coexistence spike.** Introduce `--kl-*` tokens in `styles.scss` (text, muted, primary, border, surface-0…950, warn/error), map them to the current `--p-*` values, and replace the 142 `var(--p-*)` references with `--kl-*`. This step is visually a no-op. Add `@angular/material@21.2.x`, set up `mat.theme()` (dark, `$orange-palette`), and confirm it builds and renders next to PrimeNG in WebView2 and WebKitGTK. Bump `@angular/cdk` to match.
2. **Plain CSS utilities and icons.** Remove PrimeFlex, add the 8 utility rules or move them into component SCSS. Vendor the SVG icons, register them, and replace `pi pi-*` (including the `[icon]` input on `p-button`, which goes away with step 3). Drop `primeicons`.
3. **Own thin components:** `kl-message`, `kl-tag`, and the button loading/severity treatment. Migrate `p-message` (24), `p-tag` (10), and `p-button` (53) file by file, starting with the small files (fix, dev-tools, welcome-wizard, provider-card, shortcut-recorder, dev-channel), then settings and text-enhancement.
4. **Simple form controls:** checkbox, slide toggle, slider, progress spinner, number input, card, and the plain input/textarea styling.
5. **Tooltips and tabs:** `matTooltip` (11), `mat-tab-group` (2 sets). Rewrite the `p-tab[value=…]`/`p-tabpanel[data-p-active]` test selectors onto `data-testid`/roles.
6. **Select and combobox (the risky one):** `mat-select` for fixed lists, and `matAutocomplete` for the `[editable]` model pickers in settings and active-provider. Write tests first for the null-vs-`""` behaviour documented in `model-options.ts`. Rewrite the `.p-select*` selectors in Vitest and `dark-mode.spec.ts`.
7. **Removal:** delete `providePrimeNG`, `primeng`, `@primeuix/themes`, and the PrimeNG overrides in `styles.scss`. Update `index.html`'s `.app-dark` comment, `architecture.md` (Dark Mode, PrimeNG v21 sections), `testing.md` (ResizeObserver/matchMedia notes; re-check whether Material tabs or overlays still need the stubs), `CLAUDE.md` (stack line), and the Dependabot PrimeNG fence (#89). Then do the **Angular 22 upgrade** as its own step (`ng update @angular/core @angular/cli @angular/material`, TypeScript 6.0).

Big-bang is not advisable: settings and text-enhancement together exceed 3,000 lines with many tests coupled to PrimeNG DOM, and incremental PRs keep the Windows smoke test meaningful.

**Effort: M.** About 5–7 PRs. Steps 1–5 are mechanical but wide (about 120 component usages, about 90 test selectors). Step 6 is the only part with real behavioural risk. As a rough guide, this is a few focused days of agent implementation plus review, and Michael's Windows visual check after steps 1, 3 and 6.

**Bundle:** removing PrimeFlex (−~300 KB CSS) and the primeicons fonts (−624 KB) outweighs Material's JS, which is tree-shaken per component and roughly comparable to PrimeNG's for this set (**estimate, not measured**). The net result is probably smaller on disk, which matters little for a local desktop app anyway.

---

## Open risks and uncertainty

- **Not verified by running:** PrimeNG/Material coexistence, Material tabs/overlays in jsdom without stubs, and Material rendering in WebKitGTK/WebView2. Step 1 is designed to check these.
- **Look and feel:** M3 defaults read as "Google". Getting the current Aura-orange dark look needs `mat.*-overrides` mixins. If that becomes a fight, it is the trigger for the fallback.
- **Combobox semantics:** PrimeNG's editable select and `matAutocomplete` differ in how free text and option values map to the model. `model-options.ts` holds that logic, so test it first.
- **Google risk is not zero:** Google can deprecate Material components (it did so with the legacy MDC migration in v15, which was disruptive but MIT-preserving and documented with schematics). The risk is churn, not relicensing.
- **Taiga sanctions claim** comes from a third-party aggregator and was not checked against OFAC. The "Acpekt" copyright holder is unidentified.
- **Aria stable in v22** comes from secondary coverage of the Angular v22 blog post, which I did not open directly.
- **ng-zorro / Taiga zoneless claims** were not verified in depth.
- **Commit-author counts** come from default-branch commit authors (bots filtered by name). Unlinked commit names such as "Leonidas" or "Merlin" may double-count people, so treat them as indicative.
- **PrimeNG 21 lifetime:** PrimeTek states "PrimeNG LTS will be sunset for new sales". The last 21.x release was 21.1.10 (2026-09-09), and nothing promises further 21.x patches. Angular 21 itself is LTS until **2027-06**. Migration should land well before then, ideally in Q4 2026 to Q1 2027, so the Angular 22 upgrade can follow.
