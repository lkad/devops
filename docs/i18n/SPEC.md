# Internationalization (i18n) Spec — DevOps Toolkit

**Status:** Active
**Last updated:** 2026-06-16
**Scope:** This spec covers BOTH the documentation translation system and the React frontend UI translation system. They share a language matrix and a "source of truth" policy, but live in different repos of the project.

---

## 1. Background

The project is built by a small internal team but is intended to be deployable by developers in different countries. Two audiences need bilingual (and eventually multilingual) coverage:

1. **Documentation readers** — engineers who clone the repo, read the README / DEPLOY / CONTRIBUTING / DESIGN docs before deciding to use it. They land here from GitHub or via a `git clone`, not from a running UI.
2. **UI users** — operators on call and engineers shipping features who interact with the React frontend at `http://localhost:5173` (dev) or the deployed SPA. They switch language via a header dropdown.

These two audiences have different format constraints (markdown vs JSON), different update cadences (per release vs per text change), and different tooling. They share this spec so that "English is canonical, all other languages track" applies to both.

---

## 2. Language matrix

| Locale | Code | BCP-47 | Doc status | UI status | Doc file | UI file | Owner |
|---|---|---|---|---|---|---|---|
| English (default) | `en` | `en` | source | source | `README.md` | `frontend/src/i18n/locales/en/*.json` | @maintainers |
| Simplified Chinese | `zh-CN` | `zh-CN` | **required** | **required** | `README.zh.md` | `frontend/src/i18n/locales/zh-CN/*.json` | @maintainers |
| Japanese | `ja` | `ja` | future | future | (TBD) | (TBD) | TBD |
| Traditional Chinese | `zh-TW` | `zh-TW` | future | future | (TBD) | (TBD) | TBD |
| Korean | `ko` | `ko` | future | future | (TBD) | (TBD) | TBD |

**Required languages** (`en` + `zh-CN`) ship in every release. A new language moves from `future` → `required` only after it's been translated in full (both doc and UI) and reviewed by a native speaker.

---

## 3. Documentation i18n

### 3.1 File naming

- Source-of-truth English files live at the canonical path: `README.md`, `DEPLOY.md`, `CONTRIBUTING.md`, `DESIGN.md`, `CHANGELOG.md`, etc.
- Translations live at `<NAME>.<LANG>.md` next to the English file. Example: `README.md` (en) + `README.zh.md` (zh-CN).
- For files inside `docs/`: `docs/<NAME>.md` + `docs/<NAME>.<LANG>.md`.
- Locale code uses **BCP-47 format**: `en`, `zh-CN`, `zh-TW`, `ja`, `ko`. No underscores, no POSIX-style `zh_CN`.

### 3.2 Translation rules (docs)

- **Terminology stays in English** when it's a domain term of art: Pipeline, RBAC, LDAP, JWT, blue-green, canary, SLO, WebSocket, RBAC matrix, AutoMigrate, in-memory fake, audit trail. First appearance can be glossed in parentheses, but the English term is canonical.
- **Don't translate:** product names (DevOps Toolkit, containerlab, Prometheus, Grafana, Loki, InfluxDB), repo paths, file paths, CLI commands, env var names, code blocks, JSON keys, PromQL / SQL queries, badge URLs, link URLs.
- **Markdown structural fidelity:** H2/H3 headings must be 1:1 between source and translation. A `## Quick Start` in English must be `## Quick start` or `## 快速开始` in Chinese — but it must exist and be at the same nesting level. This is enforceable by the verification script (`grep -c "^## "`).
- **Punctuation:** Chinese text uses Chinese full-width punctuation (`，。：；？！「」`). Code blocks and inline code stay ASCII. Numbers and English tokens adjacent to Chinese use a half-width space separator (per Chinese typography convention).
- **Tone:** direct, technical, no marketing copy. The English README is the reference tone.

### 3.3 Sync policy

- English is the **source of truth**. Every change to `README.md` (or any other canonical doc) MUST be paired with a translation update in the same PR.
- If a translation update isn't feasible in the same PR (e.g., massive diff, contributor doesn't speak the target language), the PR must include a `TODO(i18n): translate <commit>` line in `CHANGELOG.md` and a follow-up GitHub issue.
- A translation is considered "out of date" when the H2/H3 headings differ or when 5+ sentences are missing compared to the source. Stale translations get a `🚧` prefix on the docs index and a `i18n-lag` label on the related issue.

---

## 4. UI i18n

### 4.1 Library

The frontend uses **i18next + react-i18next + i18next-browser-languagedetector**.

- `i18next` ^23.x — core runtime
- `react-i18next` ^14.x — React bindings (hooks: `useTranslation`)
- `i18next-browser-languagedetector` ^7.x — auto-detect `navigator.language`, read URL `?lang=`, read `localStorage` key `devops-toolkit-lang`

Backend loading is **not** used — all locale JSON ships in the bundle. This is fine for a SPA where the total locale payload is <50KB.

### 4.2 File layout

```
frontend/src/
├── i18n/
│   ├── index.ts              # i18next init + LanguageDetector + react binding
│   ├── config.ts             # supportedLngs, fallbackLng, defaultNS, namespaces
│   ├── i18n.test.ts          # unit test: t() resolves, LanguageSwitcher works
│   └── locales/
│       ├── en/
│       │   ├── common.json       # nav, buttons, status words, common errors
│       │   ├── dashboard.json    # Dashboard page
│       │   ├── services.json     # Services page
│       │   ├── physical-hosts.json
│       │   ├── pipelines.json
│       │   ├── k8s.json
│       │   ├── logs.json
│       │   ├── audit.json
│       │   ├── projects.json
│       │   ├── devices.json
│       │   ├── alerts.json
│       │   ├── discovery.json
│       │   ├── metrics.json
│       │   ├── trace.json
│       │   └── login.json
│       └── zh-CN/
│           └── (mirror the same 14 namespaces)
```

14 namespaces correspond to 14 pages. Each namespace is loaded eagerly at init (small enough to bundle).

### 4.3 Naming conventions (UI strings)

- **Key format:** dotted path, lowercase, kebab-case for multi-word: `nav.dashboard`, `physical-hosts.state.online`, `common.button.save`, `errors.network.timeout`.
- **Namespace by page:** the page a string belongs to determines its namespace. Cross-page strings (nav, common buttons, status) go in `common`.
- **No duplication:** if two pages need the same string, it goes in `common` and both import it.
- **No concatenation in code:** `"Hello, " + name` is forbidden. Use `t('greeting', { name })` with a locale file containing `"greeting": "Hello, {{name}}"`. This is the only way pluralization and gender work later.

### 4.4 Interpolation

```typescript
t('physical-hosts.last-check', { minutes: 3 })
// en: "Last checked 3 minutes ago"
// zh-CN: "3 分钟前检查"
```

i18next interpolation `{{var}}` syntax. React already escapes strings, so `escapeValue: false` is set in init.

### 4.5 Pluralization

```typescript
t('physical-hosts.host-count', { count: 6 })
// en: "{{count}} host" / "{{count}} hosts" (plural forms)
// zh-CN: "{{count}} 个主机" (no plural form in Chinese)
```

Use the `_one` / `_other` suffix in en locale files. zh-CN uses a single form (Chinese has no grammatical plural).

### 4.6 Numbers, dates, and currency

`Intl.NumberFormat` and `Intl.DateTimeFormat` are preferred over custom formatters:

```typescript
new Intl.NumberFormat(i18n.language, { notation: 'compact' }).format(12400)  // "12K"
new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium', timeStyle: 'short' })
  .format(new Date())  // "Jun 16, 2026, 10:42 AM" / "2026年6月16日 10:42"
```

Backend returns ISO 8601 strings + raw numbers; the frontend is responsible for locale formatting. Don't hardcode `'%Y-%m-%d %H:%M'` anywhere.

### 4.7 Language detection order

The `i18next-browser-languagedetector` plugin's `detection.order` is:

```
['querystring', 'localStorage', 'navigator', 'htmlTag']
```

Meaning:
1. `?lang=zh-CN` URL param wins (debug / screenshot use)
2. `localStorage['devops-toolkit-lang']` (user's previous choice)
3. `navigator.language` (browser default)
4. `<html lang="...">` (last-resort)

When the detected language is not in `supportedLngs` (`['en', 'zh-CN']`), it falls back to `en`. The `fallbackLng: 'en'` is set explicitly in init.

### 4.8 Component pattern

```typescript
import { useTranslation } from 'react-i18next';

export function Header() {
  const { t, i18n } = useTranslation('common');
  return (
    <header>
      <h1>{t('app.title')}</h1>
      <button>{t('common.button.refresh')}</button>
    </header>
  );
}
```

Pages declare their namespace via `useTranslation('dashboard')` etc. Cross-page strings use `useTranslation('common')` and an explicit namespace prefix in the key.

### 4.9 TypeScript type safety

```typescript
import 'i18next';
import type enCommon from './locales/en/common.json';
import type enDashboard from './locales/en/dashboard.json';
// ...

declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'common';
    resources: {
      common: typeof enCommon;
      dashboard: typeof enDashboard;
      // ... one entry per namespace
    };
  }
}
```

This makes `t('nav.dashboard')` type-checked: a typo like `t('nav.dashbord')` is a compile error.

---

## 5. Translation rules (UI)

- **Terminology stays in English** when it's a domain term, technical concept, or product name. The UI mirrors the doc translation rules.
- **Code-like strings stay ASCII:** "ID", "IP", "URL", "OAuth", "JWT", "RBAC", etc.
- **Plural / case forms:** follow `Intl.PluralRules` for English. Chinese has no plural inflection — use a single form.
- **Tone:** professional, terse, no exclamation marks, no emoji. The dark ops theme already conveys "this is a tool."
- **No machine-translated fragments:** if a phrase is awkward in the target language, rewrite it, don't transcode it. Native-speaker review is required for `required` languages (per the matrix in §2).

---

## 6. What's NOT translated (UI)

Some text in the frontend is intentionally not subject to i18n:

- **Internal log messages** in dev-mode console output (these are for developers, not end users)
- **Component-test snapshots** in `*.test.tsx` (test fixtures, English)
- **API error keys** returned from the Go backend (`{ "error": "validation_failed" }`) — these are stable machine-readable codes, NOT messages. The frontend maps codes to localized messages via `errors.<code>` in the `common` namespace.
- **Grafana dashboards** — Grafana has its own i18n; we don't ship localized dashboards.
- **Tooltips on raw JSON / SQL strings** in the metrics/pipeline execution pages.

---

## 7. Adding a new language

Four-step PR flow:

1. **Add a row** to the language matrix table in §2 of this spec, status `in-progress`, owner filled in.
2. **Translate docs:** copy `README.md` (or any canonical doc) to `README.<LANG>.md` and translate section by section. Add cross-link from the English doc's `> 📖 Languages:` block to the new translation.
3. **Translate UI:** create `frontend/src/i18n/locales/<LANG>/` with the 14 namespace JSON files. Start with `common.json` (nav + buttons + status) — that's the highest-leverage namespace. Add to `supportedLngs` in `i18n/config.ts`.
4. **PR description** must link to the SPEC line item and the new translation. Native-speaker review is required before merge for `required` languages.

---

## 8. CI enforcement (follow-up)

These checks are not yet wired but are the target state:

- **`i18next-parser`:** scans `frontend/src/**/*.{ts,tsx}` for `t('...')` calls and detects keys that are referenced but missing from the locale files. Runs in `.github/workflows/ci.yml` on every PR; fails the build if a key is missing in any `required` locale.
- **`lychee`:** validates that all `localStorage://` / `i18n://` links resolve, including the language switcher's locale options.
- **Heading parity:** shell script that extracts `## ` headings from each translated doc and the English source, asserts that the sets are equal.
- **Translation freshness badge:** the README's `> 📖 Languages:` block shows a "🟢 current / 🟡 <N> commits behind / 🔴 missing" badge per language, computed from `git log -1 --format=%ct <file>`.

---

## 9. Roadmap

Current state (v0.5.1.0):

- [x] `en` doc + UI (canonical)
- [x] `zh-CN` doc (full translation of `README.md`)
- [x] `zh-CN` UI: `common.json` + `dashboard.json` + `physical-hosts.json` (14 namespaces, ~150 keys translated)
- [x] `LanguageSwitcher` component in header
- [x] i18next + react-i18next + browser-languagedetector integrated; `main.tsx` imports the i18n init

Follow-up (next 1-2 releases):

- [ ] Translate remaining 11 UI namespaces: `services`, `pipelines`, `k8s`, `logs`, `audit`, `projects`, `devices`, `alerts`, `discovery`, `metrics`, `trace`, `login`
- [ ] `DEPLOY.md` + `CONTRIBUTING.md` + `DESIGN.md` → Chinese translations
- [ ] Add `i18next-parser` to CI
- [ ] Add `Intl.NumberFormat` / `Intl.DateTimeFormat` to all `Date` / number displays in components
- [ ] Add a third language: `ja` or `ko` (TBD based on community feedback)
- [ ] Header bar: translate the brand title "DevOps Toolkit" in zh-CN to "运维工具集" — currently brand name stays English
- [ ] Replace `<html lang="en">` static attribute with a runtime sync to `i18n.language` so screen readers and browser translation tools see the active language

Future:

- [ ] URL-based locale routing (`/zh-CN/dashboard`) for shareable localized links
- [ ] RTL support if Arabic / Hebrew is ever added
- [ ] Translation memory / glossary file for terminology consistency (`docs/i18n/GLOSSARY.md`)

---

## 10. Out of scope

These are explicitly **not** part of i18n, even though they look related:

- **Backend Go error messages** (`internal/handler/...`) — JSON error keys are machine-readable. Localizing them would break log scraping and integration tests. The frontend maps codes to localized messages.
- **Spec documents** (`openspec/specs/*.md`) — formal technical specs. English is the contract language. Translating specs creates version-skew where the spec drifts from implementation.
- **CHANGELOG.md** — version-history log; bilingual versions double maintenance with no readability benefit. Keep English.
- **Database seed data** (`tests/fixtures/`) — test fixtures.
- **Prometheus alert rule files** (`deploy/prometheus/rules/*.yml`) — these are read by the alertmanager, not humans in this project's normal use. The `summary:` / `description:` fields are English-only.

---

## 11. References

- [i18next documentation](https://www.i18next.com/)
- [react-i18next documentation](https://react.i18next.com/)
- [BCP-47 language tags (IETF)](https://www.w3.org/International/articles/language-tags/)
- [Intl.NumberFormat / Intl.DateTimeFormat (MDN)](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl)
- [Chinese typography conventions (中文排版指南)](https://github.com/sparanoid/chinese-copywriting-guidelines)
- [DESIGN.md](../../DESIGN.md) — the project's design system (colors, type, motion); i18n integrates with the same dark ops theme
