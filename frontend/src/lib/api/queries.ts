import { useQuery, useQueryClient, type UseQueryOptions } from '@tanstack/react-query';
import {
  projectOrganizationRolesApi,
  adminApi,
  appsApi,
  cronJobsApi,
  dashboardsApi,
  datasetsApi,
  datasourcesApi,
  egressApi,
  environmentsApi,
  jobsApi,
  ontologiesApi,
  platformApi,
  productionApi,
  projectsApi,
  repositoriesApi,
  secretsApi,
  workspacesApi,
} from './endpoints';
import { describeStatus } from '@/components/ui/badge';

/**
 * Query keys, centralised so mutations can invalidate precisely instead of
 * refetching everything. `qk.project(id)` is a prefix of every project-scoped
 * key, so invalidating it drops the whole project subtree.
 */
export const qk = {
  version: ['version'] as const,
  platformOverview: ['platform', 'overview'] as const,
  hardwareTiers: ['hardware-tiers'] as const,
  preferences: ['user', 'preferences'] as const,
  organizations: ['organizations'] as const,

  projects: ['projects'] as const,
  project: (projectId: string) => ['projects', projectId] as const,
  projectDatasets: (projectId: string) => ['projects', projectId, 'datasets'] as const,
  projectDatasources: (projectId: string) => ['projects', projectId, 'datasources'] as const,
  projectOntologies: (projectId: string) => ['projects', projectId, 'ontologies'] as const,
  projectRepositories: (projectId: string) => ['projects', projectId, 'repositories'] as const,
  projectFiles: (projectId: string, path: string) => ['projects', projectId, 'files', path] as const,
  projectEgress: (projectId: string) => ['projects', projectId, 'egress'] as const,

  workspaces: (projectId?: string) => ['workspaces', projectId ?? 'all'] as const,
  jobs: (projectId?: string) => ['jobs', projectId ?? 'all'] as const,
  jobLogs: (jobId: string) => ['jobs', 'logs', jobId] as const,
  cronJobs: (projectId?: string) => ['cronjobs', projectId ?? 'all'] as const,
  apps: (projectId?: string) => ['apps', projectId ?? 'all'] as const,
  appLogs: (appId: string) => ['apps', 'logs', appId] as const,
  appRevisions: (appId: string) => ['apps', appId, 'revisions'] as const,
  dashboards: (projectId?: string) => ['dashboards', projectId ?? 'all'] as const,
  production: ['production', 'apps'] as const,

  datasets: ['datasets'] as const,
  datasetAccess: (datasetId: string) => ['datasets', datasetId, 'access'] as const,
  datasetObjects: (datasetId: string, prefix: string) =>
    ['datasets', datasetId, 'objects', prefix] as const,

  datasources: ['datasources'] as const,
  datasourceDefinitions: ['datasource-definitions'] as const,
  ontologies: ['ontologies'] as const,
  repositories: ['repositories'] as const,
  secrets: ['secrets'] as const,

  environments: (projectId?: string) => ['environments', projectId ?? 'all'] as const,
  builds: (projectId?: string) => ['builds', projectId ?? 'all'] as const,
  dockerfile: (buildId: string) => ['builds', buildId, 'dockerfile'] as const,

  egressProfiles: ['egress', 'profiles'] as const,
  egressRules: ['admin', 'egress', 'rules'] as const,

  adminOverview: ['admin', 'overview'] as const,
  adminHealth: ['admin', 'health'] as const,
  softwareInventory: ['admin', 'software-inventory'] as const,
  adminHardwareTiers: ['admin', 'hardware-tiers'] as const,
  apiTokens: ['user', 'api-tokens'] as const,
  projectOrganizationRoles: (projectId: string) =>
    ['projects', projectId, 'organization-roles'] as const,
  adminHealthHistory: (days: number) => ['admin', 'health', 'history', days] as const,
  adminSettings: ['admin', 'settings'] as const,
  adminUsers: ['admin', 'users'] as const,
  adminExecutions: ['admin', 'executions'] as const,
  adminPods: ['admin', 'pods'] as const,
  adminOrganizations: ['admin', 'organizations'] as const,
  adminOrganizationMembers: (organizationId: string) =>
    ['admin', 'organizations', organizationId, 'members'] as const,
  adminAudit: ['admin', 'audit'] as const,
  adminDataUsage: ['admin', 'data-usage'] as const,
  adminRbacMatrix: ['admin', 'rbac-matrix'] as const,
  adminStorageEndpoints: ['admin', 'storage-endpoints'] as const,
  adminBackupStatus: ['admin', 'backups', 'status'] as const,
  adminBackupRuns: ['admin', 'backups', 'runs'] as const,
  adminModules: ['admin', 'modules'] as const,
};

/**
 * Refetch interval for collections that contain converging resources.
 *
 * `onyxia-lessons.md` records that "open service" actions must be tied to real
 * readiness signals rather than optimistic timing. So the UI polls while
 * anything in the list is still starting, and stops once everything has
 * settled, instead of running a fixed 5s timer forever like the previous UI's
 * `workspacesAutoRefreshTimer`.
 */
function pollWhilePending<T extends { status?: string }>(items: T[] | undefined): number | false {
  if (!items || items.length === 0) return false;
  const converging = items.some((item) => describeStatus(item.status).pending);
  return converging ? 4000 : false;
}

type Options<T> = Omit<UseQueryOptions<T, unknown, T, readonly unknown[]>, 'queryKey' | 'queryFn'>;

/* -- platform -------------------------------------------------------------- */

export const useVersion = () => useQuery({ queryKey: qk.version, queryFn: platformApi.version, staleTime: 300_000 });

export const usePlatformOverview = () =>
  useQuery({ queryKey: qk.platformOverview, queryFn: platformApi.overview, refetchInterval: 30_000 });

export const useHardwareTiers = () =>
  useQuery({ queryKey: qk.hardwareTiers, queryFn: platformApi.hardwareTiers, staleTime: 600_000 });

export const useOrganizations = () =>
  useQuery({ queryKey: qk.organizations, queryFn: platformApi.organizations, staleTime: 120_000 });

/* -- projects -------------------------------------------------------------- */

export const useProjects = (options?: Options<Awaited<ReturnType<typeof projectsApi.list>>>) =>
  useQuery({ queryKey: qk.projects, queryFn: projectsApi.list, ...options });

export function useProject(projectId: string | undefined) {
  const query = useProjects({ enabled: Boolean(projectId) });
  return {
    ...query,
    data: projectId ? query.data?.find((project) => project.id === projectId) : undefined,
  };
}

export const useProjectDatasets = (projectId: string | undefined) =>
  useQuery({
    queryKey: qk.projectDatasets(projectId ?? ''),
    queryFn: () => projectsApi.datasets(projectId as string),
    enabled: Boolean(projectId),
  });

export const useProjectDatasources = (projectId: string | undefined) =>
  useQuery({
    queryKey: qk.projectDatasources(projectId ?? ''),
    queryFn: () => projectsApi.datasources(projectId as string),
    enabled: Boolean(projectId),
  });

export const useProjectOntologies = (projectId: string | undefined) =>
  useQuery({
    queryKey: qk.projectOntologies(projectId ?? ''),
    queryFn: () => projectsApi.ontologies(projectId as string),
    enabled: Boolean(projectId),
  });

export const useProjectRepositories = (projectId: string | undefined) =>
  useQuery({
    queryKey: qk.projectRepositories(projectId ?? ''),
    queryFn: () => projectsApi.repositories(projectId as string),
    enabled: Boolean(projectId),
  });

export const useProjectFiles = (projectId: string | undefined, path: string) =>
  useQuery({
    queryKey: qk.projectFiles(projectId ?? '', path),
    queryFn: () => projectsApi.files(projectId as string, path),
    enabled: Boolean(projectId),
  });

/* -- workloads ------------------------------------------------------------- */

export function useWorkspaces(projectId: string | undefined) {
  return useQuery({
    queryKey: qk.workspaces(projectId),
    queryFn: () => workspacesApi.list(projectId),
    enabled: Boolean(projectId),
    refetchInterval: ({ state }) => pollWhilePending(state.data),
  });
}

export function useJobs(projectId: string | undefined) {
  return useQuery({
    queryKey: qk.jobs(projectId),
    queryFn: () => jobsApi.list(projectId),
    enabled: Boolean(projectId),
    refetchInterval: ({ state }) => pollWhilePending(state.data),
  });
}

export const useJobLogs = (jobId: string | undefined) =>
  useQuery({
    queryKey: qk.jobLogs(jobId ?? ''),
    queryFn: () => jobsApi.logs(jobId as string),
    enabled: Boolean(jobId),
  });

export const useCronJobs = (projectId: string | undefined) =>
  useQuery({
    queryKey: qk.cronJobs(projectId),
    queryFn: () => cronJobsApi.list(projectId),
    enabled: Boolean(projectId),
  });

export function useApps(projectId: string | undefined) {
  return useQuery({
    queryKey: qk.apps(projectId),
    queryFn: () => appsApi.list(projectId),
    enabled: Boolean(projectId),
    refetchInterval: ({ state }) => pollWhilePending(state.data),
  });
}

export const useAppLogs = (appId: string | undefined) =>
  useQuery({
    queryKey: qk.appLogs(appId ?? ''),
    queryFn: () => appsApi.logs(appId as string),
    enabled: Boolean(appId),
  });

export const useAppRevisions = (appId: string | undefined) =>
  useQuery({
    queryKey: qk.appRevisions(appId ?? ''),
    queryFn: () => appsApi.revisions(appId as string),
    enabled: Boolean(appId),
  });

export function useDashboards(projectId: string | undefined) {
  return useQuery({
    queryKey: qk.dashboards(projectId),
    queryFn: () => dashboardsApi.list(projectId),
    enabled: Boolean(projectId),
    refetchInterval: ({ state }) => pollWhilePending(state.data),
  });
}

export const useProductionApps = () =>
  useQuery({ queryKey: qk.production, queryFn: productionApi.apps, refetchInterval: 30_000 });

/* -- data ------------------------------------------------------------------ */

export const useDatasets = () => useQuery({ queryKey: qk.datasets, queryFn: datasetsApi.list });

export const useDatasetAccess = (datasetId: string | undefined) =>
  useQuery({
    queryKey: qk.datasetAccess(datasetId ?? ''),
    queryFn: () => datasetsApi.access(datasetId as string),
    enabled: Boolean(datasetId),
  });

export const useDatasetObjects = (datasetId: string | undefined, prefix: string) =>
  useQuery({
    queryKey: qk.datasetObjects(datasetId ?? '', prefix),
    queryFn: () => datasetsApi.objects(datasetId as string, prefix),
    enabled: Boolean(datasetId),
  });

export const useDatasources = () => useQuery({ queryKey: qk.datasources, queryFn: datasourcesApi.list });

export const useDatasourceDefinitions = () =>
  useQuery({
    queryKey: qk.datasourceDefinitions,
    queryFn: datasourcesApi.definitions,
    staleTime: 600_000,
  });

export const useOntologies = () => useQuery({ queryKey: qk.ontologies, queryFn: ontologiesApi.list });

export const useRepositories = () => useQuery({ queryKey: qk.repositories, queryFn: repositoriesApi.list });

export const useSecrets = () => useQuery({ queryKey: qk.secrets, queryFn: secretsApi.list });

/* -- environments ---------------------------------------------------------- */

export function useEnvironments(projectId?: string) {
  return useQuery({
    queryKey: qk.environments(projectId),
    queryFn: () => environmentsApi.list(projectId),
    refetchInterval: ({ state }) =>
      pollWhilePending(state.data?.map((item) => ({ status: item.latestStatus }))),
  });
}

export const useBuilds = (projectId?: string) =>
  useQuery({ queryKey: qk.builds(projectId), queryFn: () => environmentsApi.builds(projectId) });

export const useDockerfile = (buildId: string | undefined) =>
  useQuery({
    queryKey: qk.dockerfile(buildId ?? ''),
    queryFn: () => environmentsApi.dockerfile(buildId as string),
    enabled: Boolean(buildId),
  });

/* -- governance ------------------------------------------------------------ */

export const useEgressProfiles = () =>
  useQuery({ queryKey: qk.egressProfiles, queryFn: egressApi.profiles, staleTime: 600_000 });

export const useEgressRules = () => useQuery({ queryKey: qk.egressRules, queryFn: egressApi.adminList });

export const useProjectEgressRules = (projectId: string | undefined) =>
  useQuery({
    queryKey: qk.projectEgress(projectId ?? ''),
    queryFn: () => projectsApi.egressRules(projectId as string),
    enabled: Boolean(projectId),
  });

/* -- administration -------------------------------------------------------- */

export const useAdminOverview = () =>
  useQuery({ queryKey: qk.adminOverview, queryFn: adminApi.overview, refetchInterval: 30_000 });

export const usePlatformSettings = () =>
  useQuery({ queryKey: qk.adminSettings, queryFn: adminApi.settings });

export const useAdminUsers = () => useQuery({ queryKey: qk.adminUsers, queryFn: adminApi.users });

/** Platform health, polled so a condition that appears between two visits is
 *  still noticed. Failures are swallowed: an unreachable health endpoint must
 *  not itself render as a platform alert. */
export const usePlatformHealthHistory = (days: number, enabled: boolean) =>
  useQuery({
    queryKey: qk.adminHealthHistory(days),
    queryFn: () => adminApi.healthHistory(days),
    enabled,
    retry: false,
  });

export const useApiTokens = () =>
  useQuery({ queryKey: qk.apiTokens, queryFn: platformApi.apiTokens });

export const useProjectOrganizationRoles = (projectId: string | undefined) =>
  useQuery({
    queryKey: qk.projectOrganizationRoles(projectId ?? ''),
    queryFn: () => projectOrganizationRolesApi.list(projectId ?? ''),
    enabled: Boolean(projectId),
  });

export const useSoftwareInventory = () =>
  useQuery({
    queryKey: qk.softwareInventory,
    queryFn: adminApi.softwareInventory,
    // Generated at build time: it cannot change while the page is open.
    staleTime: Infinity,
  });

export const usePlatformHealth = (enabled: boolean) =>
  useQuery({
    queryKey: qk.adminHealth,
    queryFn: adminApi.health,
    enabled,
    refetchInterval: 60_000,
    retry: false,
  });

export const useAdminExecutions = () =>
  useQuery({ queryKey: qk.adminExecutions, queryFn: adminApi.executions, refetchInterval: 15_000 });

export const useAdminPods = () =>
  useQuery({ queryKey: qk.adminPods, queryFn: adminApi.pods, refetchInterval: 15_000 });

export const useAdminOrganizations = () =>
  useQuery({ queryKey: qk.adminOrganizations, queryFn: adminApi.organizations });

export const useOrganizationMembers = (organizationId: string | undefined) =>
  useQuery({
    queryKey: qk.adminOrganizationMembers(organizationId ?? ''),
    queryFn: () => adminApi.organizationMembers(organizationId as string),
    enabled: Boolean(organizationId),
  });

export const useAuditEvents = () => useQuery({ queryKey: qk.adminAudit, queryFn: () => adminApi.audit() });

export const useDataUsage = () => useQuery({ queryKey: qk.adminDataUsage, queryFn: adminApi.dataUsage });

export const useAdminHardwareTiers = () =>
  useQuery({ queryKey: qk.adminHardwareTiers, queryFn: adminApi.hardwareTiers });

export const useRbacMatrix = () => useQuery({ queryKey: qk.adminRbacMatrix, queryFn: adminApi.rbacMatrix });

export const useStorageEndpoints = () =>
  useQuery({ queryKey: qk.adminStorageEndpoints, queryFn: adminApi.storageEndpoints });

export const useBackupStatus = () =>
  useQuery({ queryKey: qk.adminBackupStatus, queryFn: adminApi.backupStatus });

export const useBackupRuns = () =>
  useQuery({
    queryKey: qk.adminBackupRuns,
    queryFn: adminApi.backupRuns,
    refetchInterval: ({ state }) => pollWhilePending(state.data),
  });

export const useModules = () =>
  useQuery({ queryKey: qk.adminModules, queryFn: adminApi.modules, staleTime: 600_000 });

/** Invalidates every query under a key prefix. */
export function useInvalidate() {
  const client = useQueryClient();
  return (...keys: readonly (readonly unknown[])[]) => {
    for (const key of keys) void client.invalidateQueries({ queryKey: key });
  };
}
