import * as React from 'react';
import { useParams } from 'react-router';
import { Plus } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { Button } from '@/components/ui/button';
import { useDashboards } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';
import { AppList } from '@/features/apps/app-list';
import { CreateAppSheet } from '@/features/apps/app-form';

export function DashboardsPage() {
  const t = useT();
  const { projectId } = useParams<{ projectId: string }>();
  const [creating, setCreating] = React.useState(false);
  const dashboards = useDashboards(projectId);

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('dashboards.title')}
        description={t('dashboards.subtitle')}
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            <Plus aria-hidden />
            {t('dashboards.create')}
          </Button>
        }
      />
      {projectId ? (
        <>
          <AppList
            projectId={projectId}
            variant="dashboard"
            data={dashboards.data}
            isLoading={dashboards.isLoading}
            isError={dashboards.isError}
            error={dashboards.error}
            onRetry={() => void dashboards.refetch()}
            onCreate={() => setCreating(true)}
          />
          <CreateAppSheet
            projectId={projectId}
            variant="dashboard"
            open={creating}
            onOpenChange={setCreating}
          />
        </>
      ) : null}
    </div>
  );
}
