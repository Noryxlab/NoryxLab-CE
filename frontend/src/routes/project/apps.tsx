import * as React from 'react';
import { useParams } from 'react-router';
import { Plus } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { Button } from '@/components/ui/button';
import { useApps } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';
import { AppList } from '@/features/apps/app-list';
import { CreateAppSheet } from '@/features/apps/app-form';

export function AppsPage() {
  const t = useT();
  const { projectId } = useParams<{ projectId: string }>();
  const [creating, setCreating] = React.useState(false);
  const apps = useApps(projectId);

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('apps.title')}
        description={t('apps.subtitle')}
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            <Plus aria-hidden />
            {t('apps.create')}
          </Button>
        }
      />
      {projectId ? (
        <>
          <AppList
            projectId={projectId}
            variant="app"
            data={apps.data}
            isLoading={apps.isLoading}
            isError={apps.isError}
            error={apps.error}
            onRetry={() => void apps.refetch()}
            onCreate={() => setCreating(true)}
          />
          <CreateAppSheet
            projectId={projectId}
            variant="app"
            open={creating}
            onOpenChange={setCreating}
          />
        </>
      ) : null}
    </div>
  );
}
