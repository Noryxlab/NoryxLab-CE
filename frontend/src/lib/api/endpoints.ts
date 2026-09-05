import { api, encodeObjectPath, downloadFile, request } from './client';
import type {
  AdminHardwareTier,
  OwnedResources,
  SmtpSettings,
  SmtpState,
  SoftwareInventory,
  CreatedUser,
  ProjectOrganizationRole,
  ApiToken,
  HealthHistory,
  AdminOverview,
  EffectiveSetting,
  SearchResult,
  HealthReport,
  App,
  AppRevision,
  AppUsage,
  AuditEvent,
  BackupConfigStatus,
  BackupRun,
  Build,
  CronJob,
  Dataset,
  DatasetAccess,
  Datasource,
  DatasourceDefinition,
  AdminInventory,
  DataUsageReport,
  EgressProfile,
  EgressRule,
  Environment,
  Execution,
  HardwareTier,
  Job,
  ModuleInfo,
  Ontology,
  OntologyAccess,
  Organization,
  OrganizationMember,
  PlatformOverview,
  PlatformUser,
  PodInfo,
  Project,
  ProjectMember,
  RbacMatrixReport,
  Repository,
  Secret,
  StorageEndpoint,
  StorageObject,
  UserPreferences,
  VersionInfo,
  Workspace,
} from './types';

const V1 = '/api/v1';

export const platformApi = {
  version: () => api.get<VersionInfo>(`${V1}/version`),
  overview: () => api.get<PlatformOverview>(`${V1}/platform/overview`),
  hardwareTiers: () => api.list<HardwareTier>(`${V1}/hardware-tiers`),
  preferences: () => api.get<UserPreferences>(`${V1}/user/preferences`),
  apiTokens: () => api.list<ApiToken>(`${V1}/user/api-tokens`),
  createApiToken: (input: { name: string; expiresInDays?: number; scopes?: string[] }) =>
    api.post<{ token: ApiToken; secret: string; note: string }>(`${V1}/user/api-tokens`, input),
  revokeApiToken: (tokenId: string) =>
    api.delete<void>(`${V1}/user/api-tokens/${encodeURIComponent(tokenId)}`),
  organizations: () => api.list<Organization>(`${V1}/organizations`),
};

export const searchApi = {
  /** Results are scoped server-side by the same rules the listing endpoints
   *  use, so this can never surface anything the caller could not open. */
  search: (query: string) => api.list<SearchResult>(`${V1}/search`, { params: { q: query } }),
};

export const projectsApi = {
  list: () => api.list<Project>(`${V1}/projects`),
  create: (input: { name: string; description?: string }) => api.post<Project>(`${V1}/projects`, input),
  update: (
    projectId: string,
    input: { name?: string; description?: string; workspaceStorageSize?: string },
  ) =>
    api.put<Project>(`${V1}/projects/${projectId}`, input),
  remove: (projectId: string) => api.delete<void>(`${V1}/projects/${projectId}`),
  setOwner: (projectId: string, input: { ownerType: string; ownerId: string }) =>
    api.put<Project>(`${V1}/projects/${projectId}/ownership`, input),
  setMemberRole: (projectId: string, userId: string, role: string) =>
    api.put<void>(`${V1}/projects/${projectId}/members/${userId}/role`, { role }),
  invite: (projectId: string, input: { userId: string; role: string }) =>
    api.post<ProjectMember>(`${V1}/projects/${projectId}/invitations`, input),

  datasets: (projectId: string) => api.list<Dataset>(`${V1}/projects/${projectId}/datasets`),
  attachDataset: (projectId: string, datasetId: string) =>
    api.put<void>(`${V1}/projects/${projectId}/datasets/${datasetId}`),
  detachDataset: (projectId: string, datasetId: string) =>
    api.delete<void>(`${V1}/projects/${projectId}/datasets/${datasetId}`),

  datasources: (projectId: string) => api.list<Datasource>(`${V1}/projects/${projectId}/datasources`),
  attachDatasource: (projectId: string, datasourceId: string) =>
    api.put<void>(`${V1}/projects/${projectId}/datasources/${datasourceId}`),
  detachDatasource: (projectId: string, datasourceId: string) =>
    api.delete<void>(`${V1}/projects/${projectId}/datasources/${datasourceId}`),

  ontologies: (projectId: string) => api.list<Ontology>(`${V1}/projects/${projectId}/ontologies`),
  attachOntology: (projectId: string, ontologyId: string) =>
    api.put<void>(`${V1}/projects/${projectId}/ontologies/${ontologyId}`),
  detachOntology: (projectId: string, ontologyId: string) =>
    api.delete<void>(`${V1}/projects/${projectId}/ontologies/${ontologyId}`),
  ontology: (projectId: string) => api.get<Ontology>(`${V1}/projects/${projectId}/ontology`),
  scanOntology: (projectId: string) => api.post<Ontology>(`${V1}/projects/${projectId}/ontology/scans`),

  repositories: (projectId: string) => api.list<Repository>(`${V1}/projects/${projectId}/repositories`),
  attachRepository: (projectId: string, repositoryId: string) =>
    api.put<void>(`${V1}/projects/${projectId}/repositories/${repositoryId}`),
  detachRepository: (projectId: string, repositoryId: string) =>
    api.delete<void>(`${V1}/projects/${projectId}/repositories/${repositoryId}`),

  files: (projectId: string, path?: string) =>
    api.list<StorageObject>(
      path
        ? `${V1}/projects/${projectId}/files/${encodeObjectPath(path)}`
        : `${V1}/projects/${projectId}/files`,
    ),
  createFolder: (projectId: string, path: string) =>
    api.post<void>(`${V1}/projects/${projectId}/folders`, { path }),
  deleteFile: (projectId: string, path: string) =>
    api.delete<void>(`${V1}/projects/${projectId}/files/${encodeObjectPath(path)}`),

  egressRules: (projectId: string) => api.list<EgressRule>(`${V1}/projects/${projectId}/egress/rules`),
};

export interface CreateWorkspaceInput {
  projectId: string;
  name?: string;
  /** jupyter | vscode | rstudio. Selects the default image when `image` is
   *  omitted; the API rejects any other value. */
  ide?: string;
  /** Full image reference, taken from the chosen environment. */
  image?: string;
  hardwareTier?: string;
  storageSize?: string;
}

export const workspacesApi = {
  list: (projectId?: string) =>
    api.list<Workspace>(`${V1}/workspaces`, projectId ? { params: { projectId } } : undefined),
  create: (input: CreateWorkspaceInput) => api.post<Workspace>(`${V1}/workspaces`, input),
  remove: (workspaceId: string) => api.delete<void>(`${V1}/workspaces/${workspaceId}`),
};

export interface CreateJobInput {
  projectId: string;
  name?: string;
  /** Required by the API: a job with no image is rejected with 400. */
  image: string;
  command: string[];
  args?: string[];
  hardwareTier?: string;
}

export const jobsApi = {
  list: (projectId?: string) =>
    api.list<Job>(`${V1}/jobs`, projectId ? { params: { projectId } } : undefined),
  create: (input: CreateJobInput) => api.post<Job>(`${V1}/jobs`, input),
  remove: (jobId: string) => api.delete<void>(`${V1}/jobs/${jobId}`),
  logs: (jobId: string) => api.get<string>(`${V1}/jobs/${jobId}/logs`),
};

export interface CreateCronJobInput extends CreateJobInput {
  schedule: string;
  timeZone: string;
}

export const cronJobsApi = {
  list: (projectId?: string) =>
    api.list<CronJob>(`${V1}/cronjobs`, projectId ? { params: { projectId } } : undefined),
  create: (input: CreateCronJobInput) => api.post<CronJob>(`${V1}/cronjobs`, input),
  remove: (cronJobId: string) => api.delete<void>(`${V1}/cronjobs/${cronJobId}`),
};

/** The API validates this set exactly; anything else is rejected with 400. */
export type AppAccessMode = 'private' | 'organization' | 'users' | 'public';

export interface CreateAppInput {
  projectId: string;
  name?: string;
  slug?: string;
  image: string;
  command: string[];
  args?: string[];
  port: number;
  hardwareTier?: string;
  accessMode?: AppAccessMode;
  /** Required when accessMode is 'organization'. */
  allowedOrganizations?: string[];
  /** Required when accessMode is 'users'. */
  allowedUsers?: string[];
}

export const appsApi = {
  list: (projectId?: string) =>
    api.list<App>(`${V1}/apps`, projectId ? { params: { projectId } } : undefined),
  create: (input: CreateAppInput) => api.post<App>(`${V1}/apps`, input),
  remove: (appId: string) => api.delete<void>(`${V1}/apps/${appId}`),
  restart: (appId: string) => api.post<App>(`${V1}/apps/${appId}/restart`),
  stop: (appId: string) => api.post<App>(`${V1}/apps/${appId}/stop`),
  publish: (appId: string, input?: { accessMode?: string; allowedUsers?: string[]; allowedOrganizations?: string[] }) =>
    api.post<App>(`${V1}/apps/${appId}/publish`, input ?? {}),
  logs: (appId: string) => api.get<string>(`${V1}/apps/${appId}/logs`),
  revisions: (appId: string) => api.list<AppRevision>(`${V1}/apps/${appId}/revisions`),
  rollback: (appId: string, revisionId: string) =>
    api.post<App>(`${V1}/apps/${appId}/revisions/${revisionId}/rollback`),
  usage: (appId: string) => api.get<AppUsage>(`${V1}/apps/${appId}/usage`),
};

export const dashboardsApi = {
  list: (projectId?: string) =>
    api.list<App>(`${V1}/dashboards`, projectId ? { params: { projectId } } : undefined),
  create: (input: CreateAppInput & { slug: string }) => api.post<App>(`${V1}/dashboards`, input),
  remove: (dashboardId: string) => api.delete<void>(`${V1}/dashboards/${dashboardId}`),
};

export const productionApi = {
  apps: () => api.list<App>(`${V1}/production/apps`),
};

export interface CreateDatasetInput {
  name: string;
  description?: string;
  classification?: string;
  provider?: string;
  endpoint?: string;
  region?: string;
  bucket?: string;
  prefix?: string;
  accessKey?: string;
  secretKey?: string;
}

export const datasetsApi = {
  list: () => api.list<Dataset>(`${V1}/datasets`),
  create: (input: CreateDatasetInput) => api.post<Dataset>(`${V1}/datasets`, input),
  update: (datasetId: string, input: { name?: string; description?: string }) =>
    api.put<Dataset>(`${V1}/datasets/${datasetId}`, input),
  remove: (datasetId: string) => api.delete<void>(`${V1}/datasets/${datasetId}`),
  setOwner: (datasetId: string, input: { ownerType: string; ownerId: string }) =>
    api.put<Dataset>(`${V1}/datasets/${datasetId}/ownership`, input),

  access: (datasetId: string) => api.list<DatasetAccess>(`${V1}/datasets/${datasetId}/access`),
  grant: (datasetId: string, subjectType: string, subjectId: string, role: string) =>
    api.put<DatasetAccess>(`${V1}/datasets/${datasetId}/access/${subjectType}/${encodeURIComponent(subjectId)}`, {
      role,
    }),
  revoke: (datasetId: string, subjectType: string, subjectId: string) =>
    api.delete<void>(`${V1}/datasets/${datasetId}/access/${subjectType}/${encodeURIComponent(subjectId)}`),

  objects: (datasetId: string, prefix?: string) =>
    api.list<StorageObject>(
      prefix
        ? `${V1}/datasets/${datasetId}/objects/${encodeObjectPath(prefix)}`
        : `${V1}/datasets/${datasetId}/objects`,
    ),
  createFolder: (datasetId: string, path: string) =>
    api.post<void>(`${V1}/datasets/${datasetId}/folders`, { path }),
  deleteObject: (datasetId: string, path: string) =>
    api.delete<void>(`${V1}/datasets/${datasetId}/objects/${encodeObjectPath(path)}`),
  downloadUrl: (datasetId: string, keys: string[]) =>
    api.post<{ url: string }>(`${V1}/datasets/${datasetId}/download-url`, { keys }),
  downloadArchive: (datasetId: string, keys: string[], filename: string) =>
    downloadFile(`${V1}/datasets/${datasetId}/download`, filename, {
      method: 'POST',
      body: { keys },
    }),

  /** Uploads with progress, which fetch cannot report. */
  upload: (
    datasetId: string,
    path: string,
    file: File,
    handlers: {
      onProgress?: (percent: number) => void;
      headers: Record<string, string>;
      signal?: AbortSignal;
    },
  ): Promise<void> =>
    new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open('PUT', `${V1}/datasets/${datasetId}/objects/${encodeObjectPath(path)}`);
      for (const [key, value] of Object.entries(handlers.headers)) xhr.setRequestHeader(key, value);
      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable) handlers.onProgress?.(Math.round((event.loaded / event.total) * 100));
      };
      xhr.onload = () =>
        xhr.status >= 200 && xhr.status < 300
          ? resolve()
          : reject(new Error(xhr.responseText || `Échec du téléversement (${xhr.status})`));
      xhr.onerror = () => reject(new Error('Échec du téléversement : la plateforme est injoignable.'));
      xhr.onabort = () => reject(new Error('Téléversement annulé.'));
      handlers.signal?.addEventListener('abort', () => xhr.abort());
      xhr.send(file);
    }),

  readObject: (datasetId: string, path: string) =>
    request<Response>(`${V1}/datasets/${datasetId}/objects/${encodeObjectPath(path)}`, { raw: true }),
};

export const datasourcesApi = {
  list: () => api.list<Datasource>(`${V1}/datasources`),
  definitions: () => api.list<DatasourceDefinition>(`${V1}/datasource-definitions`),
  create: (input: Record<string, unknown>) => api.post<Datasource>(`${V1}/datasources`, input),
  remove: (datasourceId: string) => api.delete<void>(`${V1}/datasources/${datasourceId}`),
  validate: (datasourceId: string) =>
    api.post<{ reachable: boolean; error?: string }>(`${V1}/datasources/${datasourceId}/validate`),
  restart: (datasourceId: string) => api.post<Datasource>(`${V1}/datasources/${datasourceId}/restart`),
  logs: (datasourceId: string) => api.get<string>(`${V1}/datasources/${datasourceId}/logs`),
  createService: (input: Record<string, unknown>) => api.post<Datasource>(`${V1}/dataservices`, input),
};

export const ontologiesApi = {
  list: () => api.list<Ontology>(`${V1}/ontologies`),
  update: (ontologyId: string, input: Record<string, unknown>) =>
    api.put<Ontology>(`${V1}/ontologies/${ontologyId}`, input),
  remove: (ontologyId: string) => api.delete<void>(`${V1}/ontologies/${ontologyId}`),
  setOwner: (ontologyId: string, input: { ownerType: string; ownerId: string }) =>
    api.put<Ontology>(`${V1}/ontologies/${ontologyId}/ownership`, input),
  access: (ontologyId: string) => api.list<OntologyAccess>(`${V1}/ontologies/${ontologyId}/access`),
  grant: (ontologyId: string, subjectType: string, subjectId: string, role: string) =>
    api.put<OntologyAccess>(
      `${V1}/ontologies/${ontologyId}/access/${subjectType}/${encodeURIComponent(subjectId)}`,
      { role },
    ),
  revoke: (ontologyId: string, subjectType: string, subjectId: string) =>
    api.delete<void>(`${V1}/ontologies/${ontologyId}/access/${subjectType}/${encodeURIComponent(subjectId)}`),
  query: (ontologyId: string, query: string) =>
    api.post<{ columns?: string[]; rows?: unknown[][]; error?: string }>(
      `${V1}/ontologies/${ontologyId}/query`,
      { query },
    ),
};

export const repositoriesApi = {
  list: () => api.list<Repository>(`${V1}/repositories`),
  create: (input: Record<string, unknown>) => api.post<Repository>(`${V1}/repositories`, input),
  update: (repositoryId: string, input: Record<string, unknown>) =>
    api.put<Repository>(`${V1}/repositories/${repositoryId}`, input),
  remove: (repositoryId: string) => api.delete<void>(`${V1}/repositories/${repositoryId}`),
  validate: (repositoryId: string) =>
    api.post<{ reachable: boolean; error?: string }>(`${V1}/repositories/${repositoryId}/validate`),
};

export const secretsApi = {
  list: () => api.list<Secret>(`${V1}/secrets`),
  get: (name: string) => api.get<Secret>(`${V1}/secrets/${encodeURIComponent(name)}`),
  create: (input: { name: string; value: string; type?: string; expiresAt?: string | null }) =>
    api.post<Secret>(`${V1}/secrets`, input),
  remove: (name: string) => api.delete<void>(`${V1}/secrets/${encodeURIComponent(name)}`),
};

export const environmentsApi = {
  list: (projectId?: string) =>
    api.list<Environment>(`${V1}/environments`, projectId ? { params: { projectId } } : undefined),
  remove: (environmentId: string) => api.delete<void>(`${V1}/environments/${environmentId}`),
  builds: (projectId?: string) =>
    api.list<Build>(`${V1}/builds`, projectId ? { params: { projectId } } : undefined),
  createBuild: (input: Record<string, unknown>) => api.post<Build>(`${V1}/builds`, input),
  cancelBuild: (buildId: string) => api.delete<void>(`${V1}/builds/${buildId}`),
  dockerfile: (buildId: string) => api.get<string>(`${V1}/builds/${buildId}/dockerfile`),
};

export const projectOrganizationRolesApi = {
  list: (projectId: string) =>
    api.list<ProjectOrganizationRole>(`${V1}/projects/${projectId}/organization-roles`),
  grant: (projectId: string, organizationId: string, role: string) =>
    api.put<ProjectOrganizationRole>(
      `${V1}/projects/${projectId}/organization-roles/${encodeURIComponent(organizationId)}`,
      { role },
    ),
  revoke: (projectId: string, organizationId: string) =>
    api.delete<void>(
      `${V1}/projects/${projectId}/organization-roles/${encodeURIComponent(organizationId)}`,
    ),
};

export const egressApi = {
  profiles: () => api.list<EgressProfile>(`${V1}/egress/profiles`),
  request: (input: Record<string, unknown>) => api.post<EgressRule>(`${V1}/egress/rules`, input),
  adminList: () => api.list<EgressRule>(`${V1}/admin/egress/rules`),
  decide: (ruleId: string, input: { status: string; decisionNote?: string }) =>
    api.put<EgressRule>(`${V1}/admin/egress/rules/${ruleId}`, input),
};

export const adminApi = {
  overview: () => api.get<AdminOverview>(`${V1}/admin/overview`),
  softwareInventory: () => api.get<SoftwareInventory>(`${V1}/admin/software-inventory`),
  smtp: () => api.get<SmtpState>(`${V1}/admin/smtp`),
  updateSmtp: (input: Partial<SmtpSettings> & { password?: string }) =>
    api.put<SmtpState>(`${V1}/admin/smtp`, input),
  testSmtp: (input: Partial<SmtpSettings> & { password?: string; testRecipient: string }) =>
    api.post<{ sent: boolean; recipient: string }>(`${V1}/admin/smtp/tests`, input),
  ownedBy: (userId: string) =>
    api.get<{ owns: OwnedResources; count: number }>(
      `${V1}/admin/users/${encodeURIComponent(userId)}/owned`,
    ),
  deactivateUser: (userId: string, successorUserId: string) =>
    api.post<{ disabled: string; transferred: OwnedResources; tokensRevoked: number }>(
      `${V1}/admin/users/${encodeURIComponent(userId)}/deactivation`,
      { successorUserId },
    ),
  reactivateUser: (userId: string) =>
    api.delete<void>(`${V1}/admin/users/${encodeURIComponent(userId)}/deactivation`),
  sendPasswordResetEmail: (userId: string) =>
    api.post<{ sent: boolean }>(`${V1}/admin/users/${encodeURIComponent(userId)}/password-reset-email`, {}),
  hardwareTiers: () => api.list<AdminHardwareTier>(`${V1}/admin/hardware-tiers`),
  saveHardwareTier: (tier: AdminHardwareTier) =>
    api.put<AdminHardwareTier>(`${V1}/admin/hardware-tiers/${encodeURIComponent(tier.id)}`, tier),
  removeHardwareTier: (tierId: string) =>
    api.delete<void>(`${V1}/admin/hardware-tiers/${encodeURIComponent(tierId)}`),
  healthHistory: (days?: number) =>
    api.get<HealthHistory>(`${V1}/admin/health/history`, days ? { params: { days: String(days) } } : undefined),
  health: () => api.get<HealthReport>(`${V1}/admin/health`),
  settings: () => api.list<EffectiveSetting>(`${V1}/admin/settings`),
  updateSetting: (key: string, value: string) =>
    api.put<{ items: EffectiveSetting[] }>(`${V1}/admin/settings/${encodeURIComponent(key)}`, { value }),
  users: () => api.list<PlatformUser>(`${V1}/admin/users`),
  createUser: (input: {
    username: string;
    email?: string;
    firstName?: string;
    lastName?: string;
    organizationId?: string;
  }) => api.post<CreatedUser>(`${V1}/admin/users`, input),
  resetUserPassword: (userId: string) =>
    api.post<CreatedUser>(`${V1}/admin/users/${encodeURIComponent(userId)}/password`, undefined),
  inventory: () => api.get<AdminInventory>(`${V1}/admin/inventory`),
  modules: () => api.list<ModuleInfo>(`${V1}/admin/modules`),
  executions: () => api.list<Execution>(`${V1}/admin/executions`),
  killExecution: (kind: string, executionId: string) =>
    api.delete<void>(`${V1}/admin/executions/${kind}/${executionId}`),
  pods: () => api.list<PodInfo>(`${V1}/pods`),

  organizations: () => api.list<Organization>(`${V1}/admin/organizations`),
  createOrganization: (input: { name: string; alias?: string }) =>
    api.post<Organization>(`${V1}/admin/organizations`, input),
  removeOrganization: (organizationId: string) =>
    api.delete<void>(`${V1}/admin/organizations/${organizationId}`),
  organizationMembers: (organizationId: string) =>
    api.list<OrganizationMember>(`${V1}/admin/organizations/${organizationId}/members`),
  addOrganizationMember: (organizationId: string, userId: string) =>
    api.put<void>(`${V1}/admin/organizations/${organizationId}/members/${encodeURIComponent(userId)}`),
  removeOrganizationMember: (organizationId: string, userId: string) =>
    api.delete<void>(`${V1}/admin/organizations/${organizationId}/members/${encodeURIComponent(userId)}`),

  audit: (params?: Record<string, string>) => api.list<AuditEvent>(`${V1}/admin/audit`, { params }),
  downloadAudit: () => downloadFile(`${V1}/admin/audit.csv`, 'noryx-audit.csv'),

  dataUsage: () => api.get<DataUsageReport>(`${V1}/admin/data-usage`),
  downloadDataUsage: () => downloadFile(`${V1}/admin/data-usage.csv`, 'noryx-data-usage.csv'),

  rbacMatrix: () => api.get<RbacMatrixReport>(`${V1}/admin/rbac-matrix`),
  downloadRbacMatrix: () => downloadFile(`${V1}/admin/rbac-matrix.csv`, 'noryx-rbac-matrix.csv'),
  rbacPolicy: () => api.get<Record<string, unknown>>(`${V1}/admin/rbac-policy`),
  saveRbacPolicy: (policy: Record<string, unknown>) =>
    api.put<Record<string, unknown>>(`${V1}/admin/rbac-policy`, policy),

  storageEndpoints: () => api.list<StorageEndpoint>(`${V1}/admin/storage-endpoints`),
  createStorageEndpoint: (input: Record<string, unknown>) =>
    api.post<StorageEndpoint>(`${V1}/admin/storage-endpoints`, input),
  updateStorageEndpoint: (endpointId: string, input: Record<string, unknown>) =>
    api.put<StorageEndpoint>(`${V1}/admin/storage-endpoints/${endpointId}`, input),
  removeStorageEndpoint: (endpointId: string) =>
    api.delete<void>(`${V1}/admin/storage-endpoints/${endpointId}`),
  testStorageEndpoint: (endpointId: string) =>
    api.post<{ reachable: boolean; error?: string }>(`${V1}/admin/storage-endpoints/${endpointId}/test`),

  backupStatus: () => api.get<BackupConfigStatus>(`${V1}/admin/backups/config/status`),
  saveBackupConfig: (input: Record<string, unknown>) =>
    api.put<BackupConfigStatus>(`${V1}/admin/backups/config`, input),
  backupRuns: () => api.list<BackupRun>(`${V1}/admin/backups/runs`),
  runBackup: () => api.post<BackupRun>(`${V1}/admin/backups/runs`),
  backupReport: (runId: string) => api.get<string>(`${V1}/admin/backups/runs/${runId}/report`),
};

