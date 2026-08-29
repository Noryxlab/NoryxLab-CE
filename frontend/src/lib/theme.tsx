import * as React from 'react';
import { platformApi } from './api/endpoints';

export type ThemePreference = 'light' | 'dark' | 'system';
type ResolvedTheme = 'light' | 'dark';

const STORAGE_KEY = 'noryx.theme';

interface ThemeContextValue {
  preference: ThemePreference;
  resolved: ResolvedTheme;
  setPreference: (preference: ThemePreference) => void;
}

const ThemeContext = React.createContext<ThemeContextValue | null>(null);

export function useTheme(): ThemeContextValue {
  const ctx = React.useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used inside <ThemeProvider>');
  return ctx;
}

function readStored(): ThemePreference {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === 'light' || stored === 'dark') return stored;
  } catch {
    /* storage unavailable: fall through to system */
  }
  return 'system';
}

function systemTheme(): ResolvedTheme {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

/**
 * Dark mode.
 *
 * The backend already persisted a `theme` user preference and the previous UI
 * called `PUT /api/v1/user/preferences` with it, but the stylesheet had zero
 * `prefers-color-scheme` rules, so the setting had no visual effect. Here the
 * preference drives `data-theme` on <html>, and the server value is used as
 * the cross-device default when the browser has no local choice.
 */
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [preference, setPreferenceState] = React.useState<ThemePreference>(readStored);
  const [systemResolved, setSystemResolved] = React.useState<ResolvedTheme>(systemTheme);

  React.useEffect(() => {
    const query = window.matchMedia('(prefers-color-scheme: dark)');
    const listener = (event: MediaQueryListEvent) => setSystemResolved(event.matches ? 'dark' : 'light');
    query.addEventListener('change', listener);
    return () => query.removeEventListener('change', listener);
  }, []);

  const resolved: ResolvedTheme = preference === 'system' ? systemResolved : preference;

  React.useEffect(() => {
    document.documentElement.dataset['theme'] = resolved;
  }, [resolved]);

  // Adopt the server-side default only when the user has made no local choice.
  React.useEffect(() => {
    let cancelled = false;
    if (readStored() !== 'system') return;
    void platformApi
      .preferences()
      .then((preferences) => {
        if (cancelled) return;
        if (preferences.theme === 'light' || preferences.theme === 'dark') {
          setPreferenceState(preferences.theme);
        }
      })
      .catch(() => {
        /* preferences are a nicety; never block rendering on them */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const setPreference = React.useCallback((next: ThemePreference) => {
    setPreferenceState(next);
    try {
      if (next === 'system') localStorage.removeItem(STORAGE_KEY);
      else localStorage.setItem(STORAGE_KEY, next);
    } catch {
      /* private browsing: the choice applies for this session only */
    }
    void platformApi.savePreferences({ theme: next }).catch(() => {
      /* the local choice already applied; a failed sync is not worth a toast */
    });
  }, []);

  const value = React.useMemo<ThemeContextValue>(
    () => ({ preference, resolved, setPreference }),
    [preference, resolved, setPreference],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}
