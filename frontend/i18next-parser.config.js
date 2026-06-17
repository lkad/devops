/**
 * i18next-parser configuration
 *
 * Scans frontend/src/**\/*.{ts,tsx} for t('...') calls and emits one
 * JSON file per (locale, namespace) pair under src/i18n/locales/.
 *
 * `namespaceSeparator: ":"` is required so a key like t('nav:dashboard')
 * is detected as a namespace boundary; with the default separator a
 * missing namespace would silently fall back to `common` and hide the
 * bug from CI.
 *
 * `keepRemoved: true` is the key design decision. The locale files
 * contain hand-curated content (status words, table headers, empty
 * states) that is NOT yet wired into t() calls in source. Default
 * parser behavior would delete those keys on every run. keepRemoved
 * preserves them and only ADDS new keys (or fills in defaultValue
 * for new ones). The trade-off: unused keys accumulate. Periodic
 * manual cleanup is required.
 *
 * See SPEC.md §8 (CI enforcement) for the surrounding workflow.
 */
export default {
  contextSeparator: '_',
  // Default namespace used when a t() call has no explicit namespace prefix.
  defaultNamespace: 'common',
  // Dotted-path key separator inside a namespace JSON file.
  keySeparator: '.',
  // Char that separates namespace from key when both appear in one call:
  //   t('nav:dashboard')  -> namespace "nav", key "dashboard".
  namespaceSeparator: ':',
  // sort: false — preserve insertion order. With sort: true, the parser
  // alphabetically reorders keys on every run, and since hand-curated
  // files (e.g. `common.json`'s nav section ordered Dashboard, Projects,
  // Services, etc. — matching SideNav) are not alphabetical, the
  // diff is large but content-identical. Turning sort off means a
  // re-run of i18next-parser only changes content when a new t() is
  // added or removed, not just on every execution.
  sort: false,

  // Skip writing the source-locale default value into non-default locales.
  // This is the only setting that lets us enforce parity via git diff
  // (otherwise the parser would happily copy en -> zh-CN on every run).
  skipDefaultValues: true,

  // Use the modern key format with verbose comments marking the
  // source location. `legacy: false` is the current recommended mode
  // for i18next v21+.
  legacy: false,
  // Emit `@key` references at top of each file for IDE navigation.
  verbose: true,

  // **The key setting:** preserve keys that the parser didn't find
  // in source code this run. Without this, hand-curated content
  // (status words, table headers, empty states) gets wiped on
  // every `npm run i18n:extract` until source code is wired up
  // to use them via t(). Trade-off: orphaned keys accumulate and
  // require periodic manual cleanup.
  keepRemoved: true,

  // No `defaultValue: ''` here — that previously caused the parser
  // to OVERWRITE hand-translated values (e.g. "DevOps Toolkit"
  // became "" for `app.title`) because the source t() call has
  // no literal string. The default behavior (omit the key) is
  // better: when the parser finds a new key in source, the locale
  // file gets the new key with no value, and i18next falls back
  // to the English string at runtime. The developer sees the
  // missing key in the file diff and translates it.

  // Don't write <locale>_old.json next to each extracted file. The
  // _old files were a leftover from the default mode; we use git
  // for history now.
  createOldCatalogs: false,

  locales: ['en', 'zh-CN'],
  input: [
    'src/**/*.{ts,tsx}',
    '!src/**/*.test.{ts,tsx}',
    '!src/i18n/**',
  ],
  output: 'src/i18n/locales/$LOCALE/$NAMESPACE.json',
};
