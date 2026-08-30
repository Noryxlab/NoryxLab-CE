import * as React from 'react';
import { fr, type Catalogue } from './fr';
import { en } from './en';
import { config } from '@/lib/config';

export type Locale = 'fr' | 'en';

const CATALOGUES = { fr, en } as const;
const STORAGE_KEY = 'noryx.locale';

/** Dotted key path into the catalogue, e.g. `workspaces.createTitle`. */
export type TranslationKey = {
  [Section in keyof Catalogue]: `${Section & string}.${keyof Catalogue[Section] & string}`;
}[keyof Catalogue];

interface I18nContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: TranslationKey, values?: Record<string, string | number>) => string;
}

const I18nContext = React.createContext<I18nContextValue | null>(null);

export function useI18n(): I18nContextValue {
  const ctx = React.useContext(I18nContext);
  if (!ctx) throw new Error('useI18n must be used inside <I18nProvider>');
  return ctx;
}

/** Shorthand for components that only need the translate function. */
export function useT(): I18nContextValue['t'] {
  return useI18n().t;
}

function readStoredLocale(): Locale | null {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    return stored === 'fr' || stored === 'en' ? stored : null;
  } catch {
    return null;
  }
}

function detectLocale(): Locale {
  const stored = readStoredLocale();
  if (stored) return stored;
  const browser = navigator.language?.slice(0, 2).toLowerCase();
  if (browser === 'fr' || browser === 'en') return browser;
  return config.defaultLocale;
}

function interpolate(template: string, values?: Record<string, string | number>): string {
  if (!values) return template;
  return template.replace(/\{(\w+)\}/g, (match, name: string) =>
    name in values ? String(values[name]) : match,
  );
}

export function I18nProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocaleState] = React.useState<Locale>(detectLocale);

  React.useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const setLocale = React.useCallback((next: Locale) => {
    setLocaleState(next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      /* private browsing: the choice simply does not persist */
    }
  }, []);

  const t = React.useCallback<I18nContextValue['t']>(
    (key, values) => {
      const [section, entry] = key.split('.') as [keyof Catalogue, string];
      const catalogue = CATALOGUES[locale] as Record<string, Record<string, string>>;
      const fallback = CATALOGUES.fr as unknown as Record<string, Record<string, string>>;
      const template = catalogue[section]?.[entry] ?? fallback[section]?.[entry] ?? key;
      return interpolate(template, values);
    },
    [locale],
  );

  const value = React.useMemo<I18nContextValue>(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}
