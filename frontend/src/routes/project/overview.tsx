import * as React from 'react';
import { Link, useParams } from 'react-router';
import { Activity, AppWindow, Database, Plus, Terminal } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { Stat, StatGrid } from '@/components/common/stat';
import { Card, CardContent, CardHeader, CardHeaderText, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { StatusBadge } from '@/components/ui/badge';
import { EmptyState } from '@/components/common/states';
import { SkeletonText } from '@/components/ui/skeleton';
import {
  useApps,
  useJobs,
  useProject,
  useProjectDatasets,
  useWorkspaces,
} from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import { describeStatus } from '@/components/ui/badge';
import { LaunchWorkspaceSheet } from '@/features/workspaces/launch-sheet';

export function ProjectOverviewPage() {
  const t = useT();
  const { locale } = useI18n();
  const { projectId } = useParams<{ projectId: string }>();
  const [launching, setLaunching] = React.useState(false);

  const project = useProject(projectId);
  const workspaces = useWorkspaces(projectId);
  const jobs = useJobs(projectId);
  const apps = useApps(projectId);
  const datasets = useProjectDatasets(projectId);

  const activeWorkspaces = (workspaces.data ?? []).filter(
    (workspace) => describeStatus(workspace.status).tone === 'success',
  );
  const runningJobs = (jobs.data ?? []).filter((job) => describeStatus(job.status).pending);
  const runningApps = (apps.data ?? []).filter(
    (app) => describeStatus(app.status).tone === 'success',
  );

  // A single merged feed answers "what happened here recently" without asking
  // the user to visit four tabs.
  const recent = React.useMemo(() => {
    const entries = [
      ...(workspaces.data ?? []).map((item) => ({
        id: `w-${item.id}`,
        label: item.name || item.kind,
        kind: t('nav.workspaces'),
        status: item.status,
        at: item.createdAt,
        to: `/projects/${projectId}/workspaces`,
      })),
      ...(jobs.data ?? []).map((item) => ({
        id: `j-${item.id}`,
        label: item.name || item.jobName,
        kind: t('nav.jobs'),
        status: item.status,
        at: item.createdAt,
        to: `/projects/${projectId}/jobs`,
      })),
      ...(apps.data ?? []).map((item) => ({
        id: `a-${item.id}`,
        label: item.name || item.slug,
        kind: t('nav.apps'),
        status: item.status,
        at: item.startedAt ?? item.createdAt,
        to: `/projects/${projectId}/apps`,
      })),
    ];
    return entries
      .sort((a, b) => new Date(b.at).getTime() - new Date(a.at).getTime())
      .slice(0, 8);
  }, [workspaces.data, jobs.data, apps.data, projectId, t]);

  const loading = workspaces.isLoading || jobs.isLoading || apps.isLoading;

  return (
    <div className="space-y-5">
      <PageHeader
        title={project.data?.name ?? t('projectOverview.title')}
        description={project.data?.description || t('projectOverview.subtitle')}
        actions={
          <Button variant="primary" onClick={() => setLaunching(true)}>
            <Plus aria-hidden />
            {t('projectOverview.quickLaunch')}
          </Button>
        }
      />

      <StatGrid>
        <Stat
          icon={Terminal}
          label={t('projectOverview.activeWorkspaces')}
          loading={workspaces.isLoading}
          value={activeWorkspaces.length}
        />
        <Stat
          icon={Activity}
          label={t('projectOverview.runningJobs')}
          loading={jobs.isLoading}
          value={runningJobs.length}
        />
        <Stat
          icon={AppWindow}
          label={t('projectOverview.runningApps')}
          loading={apps.isLoading}
          value={runningApps.length}
        />
        <Stat
          icon={Database}
          label={t('projectOverview.attachedDatasets')}
          loading={datasets.isLoading}
          value={datasets.data?.length ?? 0}
        />
      </StatGrid>

      <Card>
        <CardHeader>
          <CardHeaderText>
            <CardTitle>{t('projectOverview.recentActivity')}</CardTitle>
            <CardDescription>{t('projects.lastActivity')}</CardDescription>
          </CardHeaderText>
        </CardHeader>
        {loading ? (
          <CardContent>
            <SkeletonText lines={5} />
          </CardContent>
        ) : recent.length === 0 ? (
          <EmptyState compact icon={Activity} title={t('projectOverview.noActivity')} />
        ) : (
          <ul className="divide-y divide-border">
            {recent.map((entry) => (
              <li key={entry.id}>
                <Link
                  to={entry.to}
                  className="flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-surface-muted focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring"
                >
                  <span className="min-w-0 flex-1 truncate text-sm font-medium">{entry.label}</span>
                  <span className="shrink-0 text-xs text-muted-foreground">{entry.kind}</span>
                  <StatusBadge status={entry.status} locale={locale} showRaw={false} />
                  <span className="w-28 shrink-0 text-right text-xs text-muted-foreground">
                    {formatRelative(entry.at, locale)}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </Card>

      {projectId ? (
        <LaunchWorkspaceSheet projectId={projectId} open={launching} onOpenChange={setLaunching} />
      ) : null}
    </div>
  );
}
