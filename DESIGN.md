# Design System — DevOps Toolkit

## Product Context
- **What this is:** Internal SRE/DevOps dashboard for managing dual-datacenter infrastructure (Containerlab-based test environment)
- **Who it's for:** DevOps/SRE engineers managing 8+ nodes across DC1 and DC2
- **Space/industry:** Network infrastructure management, ops tooling
- **Project type:** Data-dense dashboard / internal tool

## Aesthetic Direction
- **Direction:** Industrial/Utilitarian — function-first, terminal-inspired, no decorative fluff
- **Decoration level:** Minimal — typography and color do all the work, subtle surface hierarchy
- **Mood:** Feels like an extension of the command line. Data-dense for fast scanning during incidents. Technical credibility over polish.
- **Reference sites:** Grafana, Datadog, k9s — dark ops dashboards

## Typography
- **Display/Hero:** Geist 700 — gradient text for brand moments only (header title)
- **Body:** Geist 400/500 — all UI text, labels, body copy
- **UI/Labels:** Geist 500/600 — headings, section titles, button labels
- **Data/Tables:** JetBrains Mono 400/500 — device IDs, IP addresses, SSH commands, metrics values (must support tabular-nums)
- **Code:** JetBrains Mono — log entries, config snippets
- **Loading:** Google Fonts CDN — Geist + JetBrains Mono
- **Scale:**
  - Hero: 3rem (gradient text, brand use only)
  - H1: 2rem / H2: 1.5rem / H3: 1.1rem
  - Body: 1rem / Small: 0.875rem / Caption: 0.8rem
  - Mono data: 0.85rem

## Color
- **Approach:** Restrained — cyan accent is rare and meaningful, not everywhere
- **Primary:** `#22d3ee` — cyan, used for active states, primary actions, links, key data
- **Primary hover:** `#06b6d4`
- **Primary muted:** `rgba(34, 211, 238, 0.15)` — backgrounds behind cyan elements
- **Background:** `#0c1220` — near-black blue-tinted dark
- **Surface:** `#151d2e` — card/panel backgrounds
- **Surface elevated:** `#1c2940` — hover states, elevated cards
- **Border:** `#2a3a54` — standard borders
- **Border subtle:** `#1e2d42` — inside cards, dividers
- **Text primary:** `#f1f5f9`
- **Text secondary:** `#94a3b8` — labels, descriptions
- **Text muted:** `#64748b` — timestamps, captions
- **Semantic:**
  - Success: `#22c55e`
  - Warning: `#f59e0b`
  - Error: `#ef4444`
  - Info: `#3b82f6`
- **Dark mode:** Dark-first (no light mode redesign needed yet — dark is the primary product)

## Spacing
- **Base unit:** 8px
- **Density:** Comfortable — not cramped for data scanning, not airy
- **Scale:**
  - 1: 4px — micro gaps, badge padding
  - 2: 8px — icon margins, standard gaps
  - 3: 12px — card inner padding
  - 4: 16px — section padding, component spacing
  - 5: 20px — form element spacing
  - 6: 24px — card padding, section gaps
  - 7: 32px — major section spacing
  - 8: 48px — page section spacing

## Layout
- **Approach:** Grid-disciplined — strict columns, predictable alignment, max ~1400px content width
- **Grid:** 4-column stats grid, 1-column device lists, 2-column detail grids
- **Max content width:** 1400px
- **Border radius:**
  - sm: 4px — badges, tags
  - md: 8px — buttons, inputs
  - lg: 12px — cards, panels
  - xl: 16px — modals, large cards

## Motion
- **Approach:** Minimal-functional — only transitions that aid comprehension
- **Easing:** ease-out on enter, ease-in on exit, ease-in-out on move
- **Duration:**
  - Micro: 50-100ms — hover states, button feedback
  - Short: 150-250ms — panel transitions, tab switches
  - Medium: 250-400ms — modals, drawers
- **No choreography** — no entrance animations, no staggered lists, no scroll-driven effects

## Decisions Log
| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-04-19 | Initial design system created | Created by /design-consultation based on product context (SRE dashboard, dark theme, dual-datacenter infrastructure) |
