import { Outlet, useParams } from 'react-router';
import { FolderX } from 'lucide-react';
import { Breadcrumbs } from '@/components/common/breadcrumb';
import { EmptyState, ErrorState } from '@/components/common/states';
import { Card } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useProject } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';

/**
 * Project scope.
 *
 * Everything rendered below this point belongs to one project, identified by
 * the URL. The previous UI kept the "active project" in localStorage and
 * showed a `Aucun projet` chip on pages that needed one, which meant a user
 * could sit on an empty Workspaces screen with no way to tell what was wrong.
 */
export function ProjectLayout() {
  const t = useT();
  const { projectId } = useParams<{ projectId: string }>();
  const { data: project, isLoading, isError, error, refetch } = useProject(projectId);

  if (isError) {
    return <ErrorState error={error} onRetry={() => void refetch()} />;
  }

  if (!isLoading && !project) {
    return (
      <Card className="mx-auto max-w-lg">
        <EmptyState
          icon={FolderX}
          title={t('errors.projectNotFound')}
          description={t('errors.projectNotFoundHint')}
        />
      </Card>
    );
  }

  return (
    <div className="space-y-5">
      {isLoading ? (
        <Skeleton className="h-4 w-56" />
      ) : (
        <Breadcrumbs
          items={[
            { label: t('nav.projects'), to: '/projects' },
            { label: project?.name ?? projectId ?? '' },
          ]}
        />
      )}
      <Outlet />
    </div>
  );
}
