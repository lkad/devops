// i18n config — single source of truth for supported languages,
// namespaces, and detector keys. Per docs/i18n/SPEC.md §4.

export const SUPPORTED_LANGUAGES = ['en', 'zh-CN'] as const;
export type SupportedLanguage = (typeof SUPPORTED_LANGUAGES)[number];

export const FALLBACK_LANGUAGE: SupportedLanguage = 'en';
export const DEFAULT_NAMESPACE = 'common';

// 14 namespaces mirror 14 page-level files in src/pages/. Cross-page
// strings (nav, common buttons, status) live in 'common'.
export const NAMESPACES = [
  'common',
  'dashboard',
  'services',
  'physical-hosts',
  'pipelines',
  'k8s',
  'logs',
  'audit',
  'projects',
  'devices',
  'alerts',
  'discovery',
  'metrics',
  'trace',
  'login',
] as const;
export type Namespace = (typeof NAMESPACES)[number];

// Display name for the language switcher dropdown.
export const LANGUAGE_DISPLAY: Record<SupportedLanguage, string> = {
  en: 'English',
  'zh-CN': '简体中文',
};

// localStorage key for persisting the user's manual language choice.
// Per SPEC §4.7 (detection.order: querystring → localStorage → navigator → htmlTag).
export const LANG_STORAGE_KEY = 'devops-toolkit-lang';

// HTML <html lang> attribute is synced to this on init and on change.
export const HTML_LANG_ATTR = 'lang';
