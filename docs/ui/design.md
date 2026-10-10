# KeyLint look (approved 2026-10-10)

Mockup: https://claude.ai/artifact/Fps2iy91AoucB2DhKmktT1 (Fix, Pyramidize, Settings › AI providers, tokens and components). Approved by Michael as the target for E11 (see [`primeng-replacement.md`](primeng-replacement.md) and `docs/roadmap.md`).

## Idea

One thing is loud, everything else is quiet. The loud thing is the corrections: what KeyLint changed is shown like proofreading marks — the new word underlined in orange, the old one struck through in grey. Chrome stays grey and calm.

## Tokens

| Token | Value | Use |
|---|---|---|
| rail | `#18181B` | sidebar |
| ground | `#1F1F23` | page background |
| sheet | `#27272A` | text surfaces, cards, the in-use block |
| sheet-alt | `#2B2B30` | the result half of the Fix sheet |
| line | `#36363C` / `#3F3F46` | dividers / control borders |
| text | `#F4F4F5` | body |
| muted | `#A1A1AA` | hints, labels (≥4.5:1 on sheet) |
| orange | `#F97316` | primary buttons, active marks, focus ring base |
| orange-text | `#FDBA74` / `#FB923C` | orange text on dark / links and focus ring |
| on-orange | `#1C1209` | text on orange — white on `#F97316` is 2.8:1 and fails |
| info / success / warn / error | `#60A5FA` / `#2DD4BF` / `#FBBF24` / `#F87171` | messages: tinted background (~8%), border (~35–40%), icon in the colour, light text |

Radii: sheets 10px, controls 6px, menus 8px, tags pill. Shadows only on overlays.

## Type

- Interface: **Schibsted Grotesk** — 24 (page title, 700), 18 (section, 700), 14 (labels, buttons), 13/12 (hints).
- The user's text (Fix sheets, Pyramidize editor): **Literata**, 15.5px, line-height 1.7.
- Both must be **bundled with the app** (OFL, offline desktop app) — no Google Fonts at runtime.

## Patterns

- Active nav item: grey fill with an orange icon, not an orange fill.
- Buttons: primary (orange, dark text), quiet (grey fill, border), ghost (text only), danger (red outline). Loading = spinner replaces the icon, label says what is happening ("Fixing…").
- Dropdown options with secondary text use two lines (label, then muted description) — never two strings run together.
- Corrections: `ins` = orange text-tint with a 2px orange underline, `del` = grey with strike-through.
- Visible keyboard focus everywhere: 2px `#FB923C` outline, 2px offset.

## Proposed in the mockup, not decided as features

Fix page "Paste back" and "Undo", "Detect automatically" instead of "AUTO (detect)", Send Back naming the target window. Each needs its own go-ahead when that screen is built.
