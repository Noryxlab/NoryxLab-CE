import * as React from 'react';
import { Link, Outlet } from 'react-router';
import { BookOpen, Check, Code2, KeyRound, LogOut, Menu, Moon, Monitor, Sun, User } from 'lucide-react';
import { Sidebar } from './sidebar';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent } from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useI18n, useT } from '@/lib/i18n';
import { useTheme, type ThemePreference } from '@/lib/theme';
import { useAuth } from '@/lib/auth';
import { useVersion } from '@/lib/api/queries';
import { config } from '@/lib/config';
import { cn } from '@/lib/utils';
import { useExtensions } from '@/lib/extensions';
import { ExtensionSlot } from '@/components/common/extension-slot';
import { HealthIndicator } from '@/features/admin/health-indicator';
import { CommandPalette } from '@/features/search/command-palette';

const SIDEBAR_KEY = 'noryx.sidebar.collapsed';

function useCollapsedSidebar(): [boolean, () => void] {
  const [collapsed, setCollapsed] = React.useState(() => {
    try {
      return localStorage.getItem(SIDEBAR_KEY) === '1';
    } catch {
      return false;
    }
  });
  const toggle = React.useCallback(() => {
    setCollapsed((current) => {
      const next = !current;
      try {
        localStorage.setItem(SIDEBAR_KEY, next ? '1' : '0');
      } catch {
        /* ignore */
      }
      return next;
    });
  }, []);
  return [collapsed, toggle];
}

function ThemeMenuItem({ value, icon: Icon, label }: { value: ThemePreference; icon: React.ComponentType<{ className?: string }>; label: string }) {
  const { preference, setPreference } = useTheme();
  return (
    <DropdownMenuItem onSelect={() => setPreference(value)}>
      <Icon aria-hidden />
      <span className="flex-1">{label}</span>
      {preference === value ? <Check className="size-3.5 text-brand" aria-hidden /> : null}
    </DropdownMenuItem>
  );
}

function AccountMenu() {
  const t = useT();
  const { locale, setLocale } = useI18n();
  const { identity, logout, login, status } = useAuth();
  const { data: version } = useVersion();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" className="max-w-56 gap-2">
          <User aria-hidden />
          <span className="truncate">{identity?.displayName ?? t('nav.signIn')}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-64">
        {identity ? (
          <>
            <DropdownMenuLabel className="normal-case">
              <span className="block truncate text-sm font-medium text-foreground">
                {identity.displayName}
              </span>
              {identity.email ? (
                <span className="block truncate text-xs font-normal text-muted-foreground">
                  {identity.email}
                </span>
              ) : null}
              {identity.organizations.length > 0 ? (
                <span className="mt-1 block truncate text-xs font-normal text-muted-foreground">
                  {identity.organizations.join(', ')}
                </span>
              ) : null}
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
          </>
        ) : null}

        <DropdownMenuItem asChild>
          <Link to="/account">
            <KeyRound aria-hidden />
            {t('account.title')}
          </Link>
        </DropdownMenuItem>

        <DropdownMenuSeparator />
        <DropdownMenuLabel>{t('nav.theme')}</DropdownMenuLabel>
        <ThemeMenuItem value="light" icon={Sun} label={t('nav.themeLight')} />
        <ThemeMenuItem value="dark" icon={Moon} label={t('nav.themeDark')} />
        <ThemeMenuItem value="system" icon={Monitor} label={t('nav.themeSystem')} />

        <DropdownMenuSeparator />
        <DropdownMenuLabel>{t('nav.language')}</DropdownMenuLabel>
        <DropdownMenuItem onSelect={() => setLocale('fr')}>
          <span className="flex-1">Français</span>
          {locale === 'fr' ? <Check className="size-3.5 text-brand" aria-hidden /> : null}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => setLocale('en')}>
          <span className="flex-1">English</span>
          {locale === 'en' ? <Check className="size-3.5 text-brand" aria-hidden /> : null}
        </DropdownMenuItem>

        <DropdownMenuSeparator />
        {config.links.documentation ? (
          <DropdownMenuItem asChild>
            <a href={config.links.documentation} target="_blank" rel="noopener noreferrer">
              <BookOpen aria-hidden />
              {t('nav.documentation')}
            </a>
          </DropdownMenuItem>
        ) : null}
        {config.links.apiReference ? (
          <DropdownMenuItem asChild>
            <a href={config.links.apiReference} target="_blank" rel="noopener noreferrer">
              <Code2 aria-hidden />
              {t('nav.apiReference')}
            </a>
          </DropdownMenuItem>
        ) : null}

        <DropdownMenuSeparator />
        <DropdownMenuItem
          onSelect={() => (status === 'authenticated' ? logout() : login())}
          destructive={status === 'authenticated'}
        >
          <LogOut aria-hidden />
          {status === 'authenticated' ? t('nav.signOut') : t('nav.signIn')}
        </DropdownMenuItem>

        {version ? (
          <p className="px-2 pb-1 pt-2 text-[0.6875rem] tabular-nums text-muted-foreground">
            {config.brand.productName} {version.backendVersion}
          </p>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function AppShell() {
  const t = useT();
  const [collapsed, toggle] = useCollapsedSidebar();
  const [mobileOpen, setMobileOpen] = React.useState(false);
  const overlays = useExtensions('shell.overlay');

  return (
    <div className="min-h-dvh bg-background">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:left-3 focus:top-3 focus:z-100 focus:rounded-md focus:bg-surface-raised focus:px-3 focus:py-2 focus:text-sm focus:shadow-lg"
      >
        {t('nav.skipToContent')}
      </a>

      <div
        className={cn(
          'grid min-h-dvh transition-[grid-template-columns] duration-200',
          collapsed ? 'lg:grid-cols-[4rem_1fr]' : 'lg:grid-cols-[16rem_1fr]',
        )}
      >
        <aside className="sticky top-0 hidden h-dvh lg:block">
          <Sidebar collapsed={collapsed} onToggle={toggle} />
        </aside>

        <div className="flex min-w-0 flex-col">
          <header className="sticky top-0 z-40 flex h-14 items-center gap-2 border-b border-border bg-surface/85 px-3 backdrop-blur-sm sm:px-5">
            <Button
              variant="ghost"
              size="icon-sm"
              className="lg:hidden"
              aria-label={t('nav.toggleSidebar')}
              onClick={() => setMobileOpen(true)}
            >
              <Menu aria-hidden />
            </Button>
            <CommandPalette />
            <div className="flex-1" />
            <HealthIndicator />
            <AccountMenu />
          </header>

          <main id="main" className="min-w-0 flex-1 px-3 py-5 sm:px-5 lg:px-6">
            <Outlet />
          </main>
        </div>
      </div>

      {/* Mobile navigation reuses the same tree rather than a second markup path. */}
      <Dialog open={mobileOpen} onOpenChange={setMobileOpen}>
        <DialogContent
          size="sm"
          className="left-0 top-0 h-dvh max-h-dvh w-72 max-w-[85vw] translate-x-0 translate-y-0 rounded-none border-l-0 p-0"
          aria-label={t('nav.projects')}
        >
          <Sidebar collapsed={false} onToggle={() => undefined} onNavigate={() => setMobileOpen(false)} />
        </DialogContent>
      </Dialog>

      {overlays.map((module) => (
        <ExtensionSlot key={module.id} module={module} />
      ))}
    </div>
  );
}
