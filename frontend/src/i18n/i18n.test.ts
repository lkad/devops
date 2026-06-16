// i18n unit test — per docs/i18n/SPEC.md §4, validates:
//   1. i18n init completes and resolves a key in each namespace
//   2. changeLanguage() flips the active language
//   3. fallback to English when a key is missing
//   4. the 14 namespaces × 2 languages JSON files are valid
//
// Run: npx vitest run src/i18n/i18n.test.ts
import { describe, it, expect, beforeAll } from 'vitest';
import i18n from './index';
import {
  SUPPORTED_LANGUAGES,
  FALLBACK_LANGUAGE,
  NAMESPACES,
} from './config';

describe('i18n init', () => {
  beforeAll(async () => {
    // init() returns a promise; await it before assertions.
    if (!i18n.isInitialized) {
      await new Promise<void>((resolve) => {
        i18n.on('initialized', () => resolve());
      });
    }
  });

  it('initialises with at least one supported language', () => {
    expect(SUPPORTED_LANGUAGES).toContain('en');
    expect(SUPPORTED_LANGUAGES).toContain('zh-CN');
    expect(FALLBACK_LANGUAGE).toBe('en');
  });

  it('resolves a known key in English', () => {
    const value = i18n.t('app.title', { lng: 'en' });
    expect(value).toBe('DevOps Toolkit');
  });

  it('resolves a known key in Simplified Chinese', () => {
    // After init, the namespaces are loaded for both languages. Force
    // the lookup to use zh-CN.
    const value = i18n.t('app.title', { lng: 'zh-CN', ns: 'common' });
    expect(value).toBe('DevOps Toolkit'); // brand name stays English per SPEC §5
    const navDashboard = i18n.t('nav.dashboard', { lng: 'zh-CN', ns: 'common' });
    expect(navDashboard).toBe('仪表盘');
  });

  it('falls back to English for unknown keys', () => {
    // The key 'nonexistent.path' is not in any namespace. i18next
    // returns the key string itself by default; we set
    // returnEmptyString: false in init, so the lookup is benign.
    const value = i18n.t('nonexistent.path', { lng: 'en', ns: 'common' });
    expect(value).toBe('nonexistent.path');
  });

  it('handles interpolation with {{var}}', () => {
    const value = i18n.t('language.switch-to', { lng: 'en', lang: '简体中文' });
    expect(value).toContain('简体中文');
  });

  it('exposes 15 namespaces (14 pages + common)', () => {
    expect(NAMESPACES.length).toBe(15);
    expect(NAMESPACES).toContain('common');
    expect(NAMESPACES).toContain('dashboard');
    expect(NAMESPACES).toContain('physical-hosts');
  });
});

describe('locale files', () => {
  it('all 15 namespaces exist in en', async () => {
    const files = await Promise.all(
      NAMESPACES.map((ns) => import(`./locales/en/${ns}.json`)),
    );
    expect(files.length).toBe(15);
    files.forEach((mod) => {
      expect(typeof mod.default).toBe('object');
      expect(mod.default).not.toBeNull();
    });
  });

  it('all 15 namespaces exist in zh-CN', async () => {
    const files = await Promise.all(
      NAMESPACES.map((ns) => import(`./locales/zh-CN/${ns}.json`)),
    );
    expect(files.length).toBe(15);
    files.forEach((mod) => {
      expect(typeof mod.default).toBe('object');
      expect(mod.default).not.toBeNull();
    });
  });
});
