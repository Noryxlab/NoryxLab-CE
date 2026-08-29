import * as React from 'react';
import { Link } from 'react-router';
import { Activity, Boxes, HardDrive, Library, Plus, Users } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { Stat, StatGrid } from '@/components/common/stat';
import { Card, CardContent, CardHeader, CardHeaderText, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { EmptyState, ErrorState } from '@/components/common/states';
import { SkeletonCards } from '@/components/ui/skeleton';
import { usePlatformOverview, useProjects } from '@/lib/api/queries';
import { useAuth } from '@/lib/auth';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatNumber, formatRelative } from '@/lib/format';
import { CreateProjectSheet } from '@/features/projects/project-form';

export function HomePage() {
  const t = useT();
  const { locale } = useI18n();
  const { identity } = useAuth();
  const [creating, setCreating] = React.useState(false);

  const projects = useProjects();
  const overview = usePlatformOverview();

  const recent = React.useMemo(
    () =>
      [...(projects.data ?? [])]
        .sort(
          (a, b) =>
            new Date(b.lastActivityAt || b.updatedAt).getTime() -
            new Date(a.lastActivityAt || a.updatedAt).getTime(),
        )
        .slice(0, 6),
    [projects.data],
  );

  const firstName = identity?.displayName?.split(' ')[0] ?? identity?.username ?? '';

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('home.title', { name: firstName })}
        description={t('home.subtitle')}
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            <Plus aria-hidden />
            {t('home.newProject')}
          </Button>
        }
      />

      <section aria-labelledby="platform-activity" className="space-y-3">
        <h2 id="platform-activity" className="text-sm font-semibold">
          {t('home.platformActivity')}
        </h2>
        <StatGrid>
          <Stat
            icon={Users}
            label={t('home.users')}
            hint={t('home.usersHint')}
            loading={overview.isLoading}
            value={formatNumber(overview.data?.users, locale)}
          />
          <Stat
            icon={Boxes}
            label={t('home.projects')}
            hint={t('home.projectsHint')}
            loading={overview.isLoading}
            value={formatNumber(overview.data?.projects ?? projects.data?.length, locale)}
          />
          <Stat
            icon={Activity}
            label={t('home.workloads')}
            hint={t('home.workloadsHint')}
            loading={overview.isLoading}
            value={formatNumber(overview.data?.workloads, locale)}
          />
          <Stat
            icon={HardDrive}
            label={t('home.storage')}
            hint={t('home.storageHint')}
            loading={overview.isLoading}
            value={formatBytes(overview.data?.storageBytes, locale)}
          />
        </StatGrid>
      </section>

      <section aria-labelledby="recent-projects" className="space-y-3">
        <div className="flex items-end justify-between gap-3">
          <div>
            <h2 id="recent-projects" className="text-sm font-semibold">
              {t('home.recentProjects')}
            </h2>
            <p className="text-xs text-muted-foreground">{t('home.recentProjectsHint')}</p>
          </div>
          <Button variant="ghost" size="sm" asChild>
            <Link to="/projects">{t('nav.projects')}</Link>
          </Button>
        </div>

        {projects.isError ? (
          <ErrorState error={projects.error} onRetry={() => void projects.refetch()} />
        ) : projects.isLoading ? (
          <SkeletonCards count={3} />
        ) : recent.length === 0 ? (
          <Card>
            <EmptyState
              icon={Boxes}
              title={t('home.noProjects')}
              description={t('home.noProjectsHint')}
              action={
                <Button variant="primary" onClick={() => setCreating(true)}>
                  <Plus aria-hidden />
                  {t('home.newProject')}
                </Button>
              }
            />
          </Card>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {recent.map((project) => (
              <Card key={project.id} className="group relative transition-colors hover:border-brand/50">
                <CardContent className="space-y-1.5">
                  <h3 className="text-sm font-semibold">
                    <Link
                      to={`/projects/${project.id}`}
                      className="after:absolute after:inset-0 after:rounded-lg focus-visible:outline-none group-focus-within:after:outline-2 group-focus-within:after:outline-offset-2 group-focus-within:after:outline-ring"
                    >
                      {project.name}
                    </Link>
                  </h3>
                  {project.description ? (
                    <p className="line-clamp-2 text-xs leading-relaxed text-muted-foreground">
                      {project.description}
                    </p>
                  ) : null}
                  <p className="pt-1 text-xs text-muted-foreground">
                    {formatRelative(project.lastActivityAt || project.updatedAt, locale)}
                  </p>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </section>

      <section aria-labelledby="quick-actions" className="space-y-3">
        <h2 id="quick-actions" className="text-sm font-semibold">
          {t('home.quickActions')}
        </h2>
        <div className="grid gap-3 sm:grid-cols-2">
          <Card className="group relative transition-colors hover:border-brand/50">
            <CardHeader className="border-0">
              <CardHeaderText>
                <CardTitle>
                  <button
                    type="button"
                    onClick={() => setCreating(true)}
                    className="text-left after:absolute after:inset-0 after:rounded-lg focus-visible:outline-none group-focus-within:after:outline-2 group-focus-within:after:outline-offset-2 group-focus-within:after:outline-ring"
                  >
                    {t('home.newProject')}
                  </button>
                </CardTitle>
                <CardDescription>{t('home.newProjectHint')}</CardDescription>
              </CardHeaderText>
              <Plus className="size-4 shrink-0 text-muted-foreground" aria-hidden />
            </CardHeader>
          </Card>
          <Card className="group relative transition-colors hover:border-brand/50">
            <CardHeader className="border-0">
              <CardHeaderText>
                <CardTitle>
                  <Link
                    to="/catalog"
                    className="after:absolute after:inset-0 after:rounded-lg focus-visible:outline-none group-focus-within:after:outline-2 group-focus-within:after:outline-offset-2 group-focus-within:after:outline-ring"
                  >
                    {t('home.browseCatalog')}
                  </Link>
                </CardTitle>
                <CardDescription>{t('home.browseCatalogHint')}</CardDescription>
              </CardHeaderText>
              <Library className="size-4 shrink-0 text-muted-foreground" aria-hidden />
            </CardHeader>
          </Card>
        </div>
      </section>

      <CreateProjectSheet open={creating} onOpenChange={setCreating} />
    </div>
  );
}
