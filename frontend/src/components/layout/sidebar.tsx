import * as React from 'react';
import { NavLink, useLocation, useParams } from 'react-router';
import {
  Activity,
  AppWindow,
  Bot,
  Archive,
  ArrowLeft,
  Boxes,
  ChevronLeft,
  Database,
  Gauge,
  Home,
  LayoutDashboard,
  Puzzle,
  Library,
  Network,
  PanelLeft,
  Rocket,
  ScrollText,
  Settings,
  Share2,
  Shield,
  Terminal,
  Users,
  Webhook,
  KeyRound,
} from 'lucide-react';
import { useExtensions } from '@/lib/extensions';
import { useI18n, useT } from '@/lib/i18n';
import { useAuth } from '@/lib/auth';
import { useAIServices, useProject } from '@/lib/api/queries';
import { config, isEnterprise } from '@/lib/config';
import { cn } from '@/lib/utils';
import { Skeleton } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import type { TranslationKey } from '@/lib/i18n';

interface NavItem {
  /** Un libelle deja resolu, pour les entrees qu'un module apporte a
   *  l'execution et qui n'ont donc pas de clef de traduction. */
  label?: string;
  to: string;
  labelKey: TranslationKey;
  icon: React.ComponentType<{ className?: string }>;
  end?: boolean;
  adminOnly?: boolean;
  /** Enterprise modules: the section exists only where the module is
   *  deployed, so the link is absent rather than leading to a refusal. */
  enterpriseOnly?: boolean;
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
    { to: `${base}/agents`, labelKey: 'nav.agents', icon: Bot, enterpriseOnly: true },
    { to: `${base}/dashboards`, labelKey: 'nav.dashboards', icon: LayoutDashboard },
    { to: `${base}/apis`, labelKey: 'nav.apis', icon: Webhook },
    { to: `${base}/data`, labelKey: 'nav.data', icon: Database },
    { to: `${base}/members`, labelKey: 'nav.members', icon: Users },
    { to: `${base}/settings`, labelKey: 'nav.settings', icon: Settings },
  ];
}

/**
 * Administration, grouped.
 *
 * Twelve sections sat on one horizontal tab bar, which ran off the side of the
 * screen and put "Audit" beside "Réseau" as though they were the same kind of
 * thing. They are not: some are about people, some about what the platform is
 * running, some about what an auditor will ask for. The groups say which is
 * which, and the sidebar is where this application already puts a second level
 * of navigation - a project does exactly this.
 */
const ADMIN_GROUPS: { labelKey: TranslationKey; items: NavItem[] }[] = [
  {
    labelKey: 'adminNav.people',
    items: [
      // La vue d'ensemble etait le seul onglet horizontal restant a cote des
      // modules : deux barres de navigation pour un ecran, dont aucune ne
      // disait ce que l'autre contenait. Elle vit ici comme les autres.
      { to: '/admin', labelKey: 'admin.overview', icon: LayoutDashboard, end: true },
      { to: '/admin/identity', labelKey: 'nav.identity', icon: Users },
      { to: '/admin/rbac', labelKey: 'nav.rbac', icon: Shield },
    ],
  },
  {
    labelKey: 'adminNav.resources',
    items: [
      { to: '/admin/storage', labelKey: 'nav.storage', icon: Database },
      { to: '/admin/network', labelKey: 'nav.network', icon: Network },
    ],
  },
  {
    labelKey: 'adminNav.operations',
    items: [
      { to: '/admin/activity', labelKey: 'nav.activity', icon: Activity },
      // Un compte de service n'est pas une personne : il vit ici, dans les
      // operations, parce que celui qui en cree un est en train d'automatiser
      // une production ou une maintenance, pas de gerer un annuaire.
      { to: '/admin/service-accounts', labelKey: 'nav.serviceAccounts', icon: Bot },
      // Deux objets, deux entrees. Un compte de service est un principal qui
      // appartient a un client ; un identifiant de composant est la plomberie
      // interne de la plateforme. Les confondre etait l'erreur d'origine.
      { to: '/admin/component-credentials', labelKey: 'nav.componentCredentials', icon: KeyRound },
      { to: '/admin/usage', labelKey: 'usage.title', icon: Gauge },
      { to: '/admin/backups', labelKey: 'nav.backups', icon: Archive, enterpriseOnly: true },
    ],
  },
  {
    labelKey: 'adminNav.compliance',
    items: [
      { to: '/admin/audit', labelKey: 'nav.audit', icon: ScrollText },
      { to: '/admin/inventory', labelKey: 'nav.inventory', icon: Boxes },
      { to: '/admin/data', labelKey: 'nav.dataGovernance', icon: Share2, enterpriseOnly: true },
      // Les agents figurent ici et non dans les ressources : ce qu'on vient y
      // chercher n'est pas combien il y en a, c'est ce qu'ils ont le droit de
      // faire et ce qu'ils ont fait.
      { to: '/admin/agents', labelKey: 'nav.agents', icon: Bot, enterpriseOnly: true },
    ],
  },
  {
    labelKey: 'adminNav.configuration',
    items: [{ to: '/admin/settings', labelKey: 'common.settings', icon: Settings }],
  },
];

function NavRow({ item, collapsed }: { item: NavItem; collapsed: boolean }) {
  const t = useT();
  const label = item.label ?? t(item.labelKey);
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
  const { locale } = useI18n();
  const { isAdmin } = useAuth();
  const adminModules = useExtensions('admin.section');
  const { projectId } = useParams<{ projectId: string }>();
  const location = useLocation();
  const inAdministration = location.pathname.startsWith('/admin');
  const { data: project, isLoading: projectLoading } = useProject(projectId);

  // Une entree Enterprise n'apparait que la ou le module est deploye. Le
  // filtre ne regardait que adminOnly, donc enterpriseOnly etait inerte ici :
  // un lien vers une section absente mene a un refus, pas a une decouverte.
  const aiServices = useAIServices();
  const globalItems = GLOBAL_ITEMS.filter((item) => {
    if (item.adminOnly && !isAdmin) return false;
    if (item.enterpriseOnly && !isEnterprise()) return false;
    // Les agents demandent en plus que la plateforme ait de quoi les faire
    // tourner. L'edition dit ce qui est vendu, pas ce qui est installe.
    if (item.to.endsWith('/agents') && !aiServices.data?.agents) return false;
    return true;
  });

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

        {inAdministration ? (
          <div className="pt-3">
            <div className="mb-1 border-t border-sidebar-border pt-3">
              {collapsed ? null : (
                <p className="px-2.5 pb-1.5 text-[0.6875rem] font-semibold uppercase tracking-wide text-sidebar-muted">
                  {t('nav.administration')}
                </p>
              )}
            </div>
            {([
              ...ADMIN_GROUPS,
              // Les modules d'entreprise arrivent par le point d'extension et
              // avaient leur propre barre d'onglets, au-dessus, sans que rien
              // ne dise pourquoi eux et pas les autres. Une seule navigation :
              // ils prennent leur groupe ici, comme le reste.
              ...(adminModules.length > 0
                ? [
                    {
                      labelKey: 'adminNav.modules' as TranslationKey,
                      items: adminModules.map((module) => ({
                        to: `/admin/${module.id}`,
                        labelKey: 'adminNav.modules' as TranslationKey,
                        label: module.title[locale],
                        icon: Puzzle,
                      })),
                    },
                  ]
                : []),
            ] as { labelKey: TranslationKey; items: NavItem[] }[]).map((group) => {
              const items = group.items.filter((item) => !item.enterpriseOnly || isEnterprise());
              if (items.length === 0) return null;
              return (
                <div key={`${group.labelKey}:${items[0]?.to ?? ''}`} className="mb-2 space-y-1">
                  {collapsed ? null : (
                    <p className="px-2.5 pt-1 text-[0.625rem] uppercase tracking-wide text-sidebar-muted/70">
                      {t(group.labelKey)}
                    </p>
                  )}
                  {items.map((item) => (
                    <NavRow key={item.to} item={item} collapsed={collapsed} />
                  ))}
                </div>
              );
            })}
          </div>
        ) : null}

        {projectId ? (
          <div className="pt-3">
            <div className="mb-1 border-t border-sidebar-border pt-3">
              {collapsed ? null : (
                <div className="px-2.5 pb-1.5">
                  <p className="text-[0.6875rem] font-semibold uppercase tracking-wide text-sidebar-muted">
                    {t('common.project')}
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
