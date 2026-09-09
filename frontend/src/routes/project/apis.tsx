import * as React from 'react';
import { useParams } from 'react-router';
import { Plus } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { Button } from '@/components/ui/button';
import { useApis } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';
import { AppList } from '@/features/apps/app-list';
import { CreateAppSheet } from '@/features/apps/app-form';
import { ApiCallPanel } from '@/features/apps/api-call-panel';

/**
 * Endpoints, listed like applications and presented differently.
 *
 * The workload is the same; what changes is that nobody opens an API in a
 * browser. So the page shows, for each deployed endpoint, the address and the
 * two snippets somebody actually pastes — which is the difference between an
 * endpoint that gets used and one that gets rebuilt elsewhere.
 */
export function ApisPage() {
  const t = useT();
  const { projectId } = useParams<{ projectId: string }>();
  const [creating, setCreating] = React.useState(false);
  const apis = useApis(projectId);

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('apis.title')}
        description={t('apis.subtitle')}
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            <Plus aria-hidden />
            {t('apis.create')}
          </Button>
        }
      />
      {projectId ? (
        <>
          <AppList
            projectId={projectId}
            variant="api"
            data={apis.data}
            isLoading={apis.isLoading}
            isError={apis.isError}
            error={apis.error}
            onRetry={() => void apis.refetch()}
            onCreate={() => setCreating(true)}
          />
          {apis.data?.map((api) => <ApiCallPanel key={api.id} api={api} />)}
          <CreateAppSheet
            projectId={projectId}
            variant="api"
            open={creating}
            onOpenChange={setCreating}
          />
        </>
      ) : null}
    </div>
  );
}
