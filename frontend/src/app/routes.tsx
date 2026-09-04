import { Navigate, Route, Routes } from 'react-router';
import { AppShell } from '@/components/layout/app-shell';
import { AuthGate } from './auth-gate';
import { HomePage } from '@/routes/home';
import { ProjectsPage } from '@/routes/projects';
import { ProjectLayout } from '@/routes/project/layout';
import { ProjectOverviewPage } from '@/routes/project/overview';
import { WorkspacesPage } from '@/routes/project/workspaces';
import { JobsPage } from '@/routes/project/jobs';
import { AppsPage } from '@/routes/project/apps';
import { DashboardsPage } from '@/routes/project/dashboards';
import { ProjectDataPage } from '@/routes/project/data';
import { ProjectMembersPage } from '@/routes/project/members';
import { ProjectSettingsPage } from '@/routes/project/settings';
import { AccountPage } from '@/routes/account';
import { CatalogPage } from '@/routes/catalog';
import { ProductionPage } from '@/routes/production';
import { AdminPage } from '@/routes/admin';
import { NotFoundPage } from '@/routes/not-found';

/**
 * Hybrid information architecture.
 *
 * Global level: Home, Projects, Catalog, Production, Administration.
 * Project level: everything under /projects/:projectId, so the project a
 * resource belongs to is in the URL rather than in hidden localStorage state.
 * The catalog stays global (Databricks Unity Catalog shape) because datasets,
 * data sources and ontologies are shared across projects.
 */
export function AppRoutes() {
  return (
    <Routes>
      <Route element={<AuthGate />}>
        <Route element={<AppShell />}>
          <Route index element={<HomePage />} />
          <Route path="projects" element={<ProjectsPage />} />

          <Route path="projects/:projectId" element={<ProjectLayout />}>
            <Route index element={<ProjectOverviewPage />} />
            <Route path="workspaces" element={<WorkspacesPage />} />
            <Route path="jobs" element={<JobsPage />} />
            <Route path="apps" element={<AppsPage />} />
            <Route path="dashboards" element={<DashboardsPage />} />
            <Route path="data" element={<ProjectDataPage />} />
            <Route path="data/:section" element={<ProjectDataPage />} />
            {/* Environments moved to the catalogue: they are platform-wide,
                not owned by a project. Existing links keep working. */}
            <Route path="environments" element={<Navigate to="/catalog/environments" replace />} />
            <Route path="members" element={<ProjectMembersPage />} />
            <Route path="settings" element={<ProjectSettingsPage />} />
          </Route>

          <Route path="account" element={<AccountPage />} />

          <Route path="catalog" element={<CatalogPage />} />
          <Route path="catalog/:section" element={<CatalogPage />} />
          <Route path="catalog/:section/:resourceId" element={<CatalogPage />} />

          <Route path="production" element={<ProductionPage />} />

          <Route path="admin" element={<AdminPage />} />
          <Route path="admin/:section" element={<AdminPage />} />

          {/* Legacy tab ids from the previous single-page UI, so links and
              bookmarks people already have keep working. */}
          <Route path="workspaces" element={<Navigate to="/projects" replace />} />
          <Route path="environments" element={<Navigate to="/catalog/environments" replace />} />
          <Route path="data-datasets" element={<Navigate to="/catalog/datasets" replace />} />
          <Route path="data-datasources" element={<Navigate to="/catalog/datasources" replace />} />
          <Route path="data-ontology" element={<Navigate to="/catalog/ontologies" replace />} />
          <Route path="data-git" element={<Navigate to="/catalog/repositories" replace />} />
          <Route path="data-secrets" element={<Navigate to="/catalog/secrets" replace />} />
          <Route path="ops" element={<Navigate to="/admin" replace />} />
          <Route path="govern-rbac" element={<Navigate to="/admin/rbac" replace />} />
          <Route path="govern-network" element={<Navigate to="/admin/network" replace />} />
          <Route path="workloads" element={<Navigate to="/admin/activity" replace />} />

          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Route>
    </Routes>
  );
}
