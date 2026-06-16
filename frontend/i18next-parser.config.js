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
  // Lexicographic sort so CI diffs are minimal / stable across runs.
  sort: true,

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

  locales: ['en', 'zh-CN'],
  input: [
    'src/**/*.{ts,tsx}',
    '!src/**/*.test.{ts,tsx}',
    '!src/i18n/**',
  ],
  output: 'src/i18n/locales/$LOCALE/$NAMESPACE.json',
};
