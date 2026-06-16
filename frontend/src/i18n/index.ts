// i18n init — per docs/i18n/SPEC.md §4. Wires i18next + react-i18next
// + i18next-browser-languagedetector and loads the 14 namespace JSON
// files for each of the two supported languages.
//
// IMPORTANT: this file is imported once in main.tsx BEFORE the React
// tree mounts, so components can call useTranslation() synchronously.

import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import {
  SUPPORTED_LANGUAGES,
  FALLBACK_LANGUAGE,
  DEFAULT_NAMESPACE,
  LANG_STORAGE_KEY,
  HTML_LANG_ATTR,
} from './config';

// Eagerly imported locale files. Each namespace × language pair is a
// separate JSON file under locales/<lang>/<ns>.json.
import enCommon from './locales/en/common.json';
import enDashboard from './locales/en/dashboard.json';
import enServices from './locales/en/services.json';
import enPhysicalHosts from './locales/en/physical-hosts.json';
import enPipelines from './locales/en/pipelines.json';
import enK8s from './locales/en/k8s.json';
import enLogs from './locales/en/logs.json';
import enAudit from './locales/en/audit.json';
import enProjects from './locales/en/projects.json';
import enDevices from './locales/en/devices.json';
import enAlerts from './locales/en/alerts.json';
import enDiscovery from './locales/en/discovery.json';
import enMetrics from './locales/en/metrics.json';
import enTrace from './locales/en/trace.json';
import enLogin from './locales/en/login.json';

import zhCNCommon from './locales/zh-CN/common.json';
import zhCNDashboard from './locales/zh-CN/dashboard.json';
import zhCNServices from './locales/zh-CN/services.json';
import zhCNPhysicalHosts from './locales/zh-CN/physical-hosts.json';
import zhCNPipelines from './locales/zh-CN/pipelines.json';
import zhCNK8s from './locales/zh-CN/k8s.json';
import zhCNLogs from './locales/zh-CN/logs.json';
import zhCNAudit from './locales/zh-CN/audit.json';
import zhCNProjects from './locales/zh-CN/projects.json';
import zhCNDevices from './locales/zh-CN/devices.json';
import zhCNAlerts from './locales/zh-CN/alerts.json';
import zhCNDiscovery from './locales/zh-CN/discovery.json';
import zhCNMetrics from './locales/zh-CN/metrics.json';
import zhCNTrace from './locales/zh-CN/trace.json';
import zhCNLogin from './locales/zh-CN/login.json';

void i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    // All 14 namespaces × 2 languages registered eagerly. Total payload
    // ~80KB uncompressed; well within reason for a SPA bundle.
    resources: {
      en: {
        common: enCommon,
        dashboard: enDashboard,
        services: enServices,
        'physical-hosts': enPhysicalHosts,
        pipelines: enPipelines,
        k8s: enK8s,
        logs: enLogs,
        audit: enAudit,
        projects: enProjects,
        devices: enDevices,
        alerts: enAlerts,
        discovery: enDiscovery,
        metrics: enMetrics,
        trace: enTrace,
        login: enLogin,
      },
      'zh-CN': {
        common: zhCNCommon,
        dashboard: zhCNDashboard,
        services: zhCNServices,
        'physical-hosts': zhCNPhysicalHosts,
        pipelines: zhCNPipelines,
        k8s: zhCNK8s,
        logs: zhCNLogs,
        audit: zhCNAudit,
        projects: zhCNProjects,
        devices: zhCNDevices,
        alerts: zhCNAlerts,
        discovery: zhCNDiscovery,
        metrics: zhCNMetrics,
        trace: zhCNTrace,
        login: zhCNLogin,
      },
    },
    fallbackLng: FALLBACK_LANGUAGE,
    supportedLngs: [...SUPPORTED_LANGUAGES],
    defaultNS: DEFAULT_NAMESPACE,
    ns: [
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
    ],
    // React already escapes strings, no double-escape.
    interpolation: { escapeValue: false },
    // Per SPEC §4.7: querystring first (debug), then localStorage,
    // then navigator, then <html lang="...">.
    detection: {
      order: ['querystring', 'localStorage', 'navigator', 'htmlTag'],
      lookupQuerystring: 'lang',
      lookupLocalStorage: LANG_STORAGE_KEY,
      caches: ['localStorage'],
    },
    // Don't warn on missing keys during dev — we fall back to the
    // English string, which is acceptable. Production builds will
    // catch this via a future i18next-parser check.
    saveMissing: false,
    returnEmptyString: false,
  })
  .then(() => {
    // Sync <html lang="..."> with the active language. The detector
    // runs synchronously, so by the time the .then callback fires
    // i18n.language is the resolved value.
    if (typeof document !== 'undefined') {
      document.documentElement.setAttribute(HTML_LANG_ATTR, i18n.language);
    }
  });

// Update the <html lang> attribute on every language change so
// screen readers and browser translation tools see the active language.
i18n.on('languageChanged', (lng) => {
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute(HTML_LANG_ATTR, lng);
  }
});

export default i18n;
