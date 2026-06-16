// LanguageSwitcher — header dropdown that flips the UI between
// English and Simplified Chinese. Per docs/i18n/SPEC.md §4.
//
// On change, calls i18n.changeLanguage() which:
//   1. Updates i18n.language
//   2. Persists the choice to localStorage[devops-toolkit-lang]
//   3. Fires the 'languageChanged' event (handled in i18n/index.ts
//      to update <html lang="...">)
//
// All registered components re-render because react-i18next subscribes
// to the language change event internally.
import { useTranslation } from 'react-i18next';
import {
  SUPPORTED_LANGUAGES,
  LANGUAGE_DISPLAY,
  type SupportedLanguage,
} from '../../i18n/config';

export function LanguageSwitcher() {
  const { i18n, t } = useTranslation('common');
  const current = i18n.language as SupportedLanguage;

  const onChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const next = e.target.value as SupportedLanguage;
    void i18n.changeLanguage(next);
  };

  return (
    <label
      title={t('language.switch-to', { lang: current === 'en' ? '简体中文' : 'English' })}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 'var(--sp-2)',
        fontSize: 'var(--fs-small)',
        color: 'var(--color-text-secondary)',
      }}
    >
      <span style={{ color: 'var(--color-text-muted)' }}>{t('language.label')}:</span>
      <select
        value={current}
        onChange={onChange}
        style={{
          background: 'var(--color-surface-elevated)',
          color: 'var(--color-text-primary)',
          border: '1px solid var(--color-border)',
          borderRadius: 'var(--radius-sm)',
          padding: '4px 8px',
          fontSize: 'var(--fs-small)',
          fontFamily: 'inherit',
          cursor: 'pointer',
        }}
      >
        {SUPPORTED_LANGUAGES.map((lng) => (
          <option key={lng} value={lng}>
            {LANGUAGE_DISPLAY[lng]}
          </option>
        ))}
      </select>
    </label>
  );
}
