import * as React from 'react';
import { NavLink, useParams } from 'react-router';
import {
  Activity,
  AppWindow,
  ArrowLeft,
  Boxes,
  ChevronLeft,
  Database,
  FolderGit2,
  Gauge,
  Home,
  LayoutDashboard,
  Library,
  PanelLeft,
  Rocket,
  Settings,
  Shield,
  Terminal,
  Users,
} from 'lucide-react';
import { useT } from '@/lib/i18n';
import { useAuth } from '@/lib/auth';
import { useProject } from '@/lib/api/queries';
import { config } from '@/lib/config';
import { cn } from '@/lib/utils';
import { Skeleton } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import type { TranslationKey } from '@/lib/i18n';

interface NavItem {
  to: string;
  labelKey: TranslationKey;
  icon: React.ComponentType<{ className?: string }>;
  end?: boolean;
  adminOnly?: boolean;
}

const GLOBAL_ITEMS: NavItem[] = [
  { to: '/', labelKey: 'nav.home', icon: Home, end: true },
  { to: '/projects', labelKey: 'nav.projects', icon: Boxes },
  { to: '/catalog', labelKey: 'nav.catalog', icon: Library },
  { to: '/production', labelKey: 'nav.production', icon: Rocket },
  { to: '/admin', labelKey: 'nav.administration', icon: Shield, adminOnly: true },
];

function projectItems(projectId: string): NavItem[] {
  const base = `/projects/${projectId}`;
  return [
    { to: base, labelKey: 'nav.overview', icon: Gauge, end: true },
    { to: `${base}/workspaces`, labelKey: 'nav.workspaces', icon: Terminal },
    { to: `${base}/jobs`, labelKey: 'nav.jobs', icon: Activity },
    { to: `${base}/apps`, labelKey: 'nav.apps', icon: AppWindow },
    { to: `${base}/dashboards`, labelKey: 'nav.dashboards', icon: LayoutDashboard },
    { to: `${base}/data`, labelKey: 'nav.data', icon: Database },
    { to: `${base}/environments`, labelKey: 'nav.environments', icon: FolderGit2 },
    { to: `${base}/members`, labelKey: 'nav.members', icon: Users },
    { to: `${base}/settings`, labelKey: 'nav.settings', icon: Settings },
  ];
}

function NavRow({ item, collapsed }: { item: NavItem; collapsed: boolean }) {
  const t = useT();
  const label = t(item.labelKey);
  const Icon = item.icon;
  return (
    <NavLink
      to={item.to}
      end={item.end}
      title={collapsed ? label : undefined}
      className={({ isActive }) =>
        cn(
          'flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm font-medium transition-colors',
          'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
          collapsed && 'justify-center px-2',
          isActive
            ? 'bg-sidebar-active text-sidebar-active-foreground'
            : 'text-sidebar-muted hover:bg-surface-muted hover:text-sidebar-foreground',
        )
      }
    >
      <Icon className="size-4 shrink-0" aria-hidden />
      {collapsed ? <span className="sr-only">{label}</span> : <span className="truncate">{label}</span>}
    </NavLink>
  );
}

/**
 * Two-level navigation.
 *
 * The previous sidebar listed 16 flat entries grouped as Data / Develop /
 * Production / Govern — a 1:1 mapping of the API onto the menu. The active
 * project was hidden global state, surfaced only as a "Aucun projet" chip in
 * each page title, so a user could land on Workspaces and see an empty screen
 * with no explanation. Here the project is a place you enter: its resources
 * are nested under /projects/:id, and the sidebar shows where you are.
 */
export function Sidebar({
  collapsed,
  onToggle,
  onNavigate,
}: {
  collapsed: boolean;
  onToggle: () => void;
  onNavigate?: () => void;
}) {
  const t = useT();
  const { isAdmin } = useAuth();
  const { projectId } = useParams<{ projectId: string }>();
  const { data: project, isLoading: projectLoading } = useProject(projectId);

  const globalItems = GLOBAL_ITEMS.filter((item) => !item.adminOnly || isAdmin);

  return (
    <div
      className="flex h-full flex-col border-r border-sidebar-border bg-sidebar"
      onClick={onNavigate ? (event) => {
        if ((event.target as HTMLElement).closest('a')) onNavigate();
      } : undefined}
    >
      <div className={cn('flex items-center gap-2 px-3 py-3.5', collapsed && 'justify-center px-2')}>
        {config.brand.logoUrl ? (
          <img src={config.brand.logoUrl} alt="" aria-hidden className="size-7 shrink-0 rounded-md" />
        ) : null}
        {collapsed ? null : (
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-semibold tracking-tight">{config.brand.productName}</p>
            {config.brand.editionLabel ? (
              <p className="truncate text-[0.6875rem] uppercase tracking-wide text-sidebar-muted">
                {config.brand.editionLabel}
              </p>
            ) : null}
          </div>
        )}
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={onToggle}
          aria-label={t('nav.toggleSidebar')}
          aria-expanded={!collapsed}
          className={cn('hidden lg:inline-flex', collapsed && 'absolute right-1 top-3.5')}
        >
          {collapsed ? <PanelLeft aria-hidden /> : <ChevronLeft aria-hidden />}
        </Button>
      </div>

      <nav
        aria-label={t('nav.projects')}
        className="scrollbar-thin flex-1 space-y-1 overflow-y-auto px-2 pb-3"
      >
        {globalItems.map((item) => (
          <NavRow key={item.to} item={item} collapsed={collapsed} />
        ))}

        {projectId ? (
          <div className="pt-3">
            <div className="mb-1 border-t border-sidebar-border pt-3">
              {collapsed ? null : (
                <div className="px-2.5 pb-1.5">
                  <p className="text-[0.6875rem] font-semibold uppercase tracking-wide text-sidebar-muted">
                    {t('nav.projectSection')}
                  </p>
                  {projectLoading ? (
                    <Skeleton className="mt-1 h-4 w-32" />
                  ) : (
                    <p className="truncate text-sm font-semibold" title={project?.name}>
                      {project?.name ?? projectId}
                    </p>
                  )}
                </div>
              )}
            </div>
            <div className="space-y-1">
              {projectItems(projectId).map((item) => (
                <NavRow key={item.to} item={item} collapsed={collapsed} />
              ))}
            </div>
            <NavLink
              to="/projects"
              title={collapsed ? t('nav.backToProjects') : undefined}
              className={cn(
                'mt-2 flex items-center gap-2.5 rounded-md px-2.5 py-2 text-xs font-medium text-sidebar-muted transition-colors hover:bg-surface-muted hover:text-sidebar-foreground',
                'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
                collapsed && 'justify-center px-2',
              )}
            >
              <ArrowLeft className="size-3.5 shrink-0" aria-hidden />
              {collapsed ? (
                <span className="sr-only">{t('nav.backToProjects')}</span>
              ) : (
                t('nav.backToProjects')
              )}
            </NavLink>
          </div>
        ) : null}
      </nav>
    </div>
  );
}
