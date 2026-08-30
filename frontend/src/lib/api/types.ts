/**
 * Domain types, mirrored from the Go structs in `backend/internal/domain`
 * and the handler response shapes. Kept hand-written rather than generated
 * because `api/openapi.yaml` currently documents 3 of the ~150 endpoints;
 * when the spec catches up (roadmap phase 5, "freeze API surface"), this
 * file is the natural thing to replace with generated output.
 */

export interface ListResponse<T> {
  items: T[] | null;
}

export type OwnerType = 'user' | 'organization';
export type AccessRole = 'reader' | 'writer' | 'admin';
export type ProjectRole = 'viewer' | 'editor' | 'admin';

export interface Project {
  id: string;
  name: string;
  description: string;
  ownerType: OwnerType | '';
  ownerId: string;
  canManageOwner?: boolean;
  createdAt: string;
  updatedAt: string;
  lastActivityAt: string;
  runningApps: number;
  runningJobs: number;
  runningWorkspaces: number;
}

export interface ProjectMember {
  userId: string;
  role: ProjectRole;
  email?: string;
  displayName?: string;
}

export interface Workspace {
  id: string;
  projectId: string;
  kind: string;
  name: string;
  image: string;
  podName: string;
  serviceName: string;
  pvcName: string;
  pvcClass: string;
  pvcSize: string;
  pvcMountPath: string;
  cpu: string;
  memory: string;
  status: string;
  accessUrl: string;
  createdAt: string;
}

export interface Job {
  id: string;
  projectId: string;
  name: string;
  image: string;
  command: string[] | null;
  args: string[] | null;
  jobName: string;
  status: string;
  createdAt: string;
  completedAt?: string | null;
  resultAvailable: boolean;
}

export interface CronJob {
  id: string;
  projectId: string;
  name: string;
  /** Underlying Kubernetes CronJob name. */
  cronJobName: string;
  schedule: string;
  timeZone: string;
  /** A suspended schedule stays declared but produces no run. */
  suspended: boolean;
  image: string;
  createdAt: string;
}

export interface App {
  id: string;
  projectId: string;
  ownerUserId: string;
  kind: string;
  name: string;
  slug: string;
  image: string;
  command: string[] | null;
  args: string[] | null;
  port: number;
  podName: string;
  serviceName: string;
  status: string;
  accessUrl: string;
  accessMode: string;
  allowedUsers?: string[];
  allowedOrganizations?: string[];
  createdAt: string;
  healthMessage?: string;
  restartCount: number;
  startedAt?: string | null;
  published: boolean;
  activeRevision?: number;
  publishedAt?: string | null;
}

export interface AppRevision {
  id: string;
  appId: string;
  number: number;
  snapshot: App;
  publishedBy: string;
  publishedAt: string;
  active: boolean;
}

export interface AppUsage {
  appId: string;
  views?: number;
  lastAccessedAt?: string | null;
  uniqueUsers?: number;
}

export type DatasetClassification = 'hds' | 'non-hds';
export type DatasetProvider = 'minio' | 's3' | string;

export interface Dataset {
  id: string;
  ownerUserId: string;
  ownerType: OwnerType | '';
  ownerId: string;
  name: string;
  description: string;
  bucket: string;
  prefix: string;
  provider: DatasetProvider;
  classification: DatasetClassification;
  endpoint?: string;
  region?: string;
  accessRole?: AccessRole;
  createdAt: string;
  updatedAt: string;
}

export interface DatasetAccess {
  datasetId: string;
  userId?: string;
  subjectType: OwnerType;
  subjectId: string;
  role: AccessRole;
  createdAt: string;
  updatedAt: string;
}

export interface StorageObject {
  key: string;
  name?: string;
  size: number;
  lastModified: string;
  isPrefix?: boolean;
  contentType?: string;
}

export interface Datasource {
  id: string;
  ownerUserId: string;
  name: string;
  type: string;
  source: string;
  host: string;
  port: number;
  database: string;
  username: string;
  passwordSecret: string;
  sslMode: string;
  serviceDefinitionId?: string;
  image?: string;
  dockerfile?: string;
  system: boolean;
  status?: string;
  podName?: string;
  serviceName?: string;
  pvcName?: string;
  storageSize?: string;
  hardwareTier?: string;
  statusReason?: string;
  statusMessage?: string;
  restartCount?: number;
  startedAt?: string;
  attachedProjectIds?: string[];
  createdAt: string;
  updatedAt: string;
}

export interface DatasourceDefinition {
  id: string;
  name: string;
  type: string;
  image: string;
  dockerfile: string;
  system: boolean;
  description: string;
  defaultPort: number;
}

export interface Ontology {
  id: string;
  ownerUserId: string;
  ownerType: OwnerType | '';
  ownerId: string;
  name: string;
  description: string;
  sourceType: string;
  sourceId: string;
  sourceName: string;
  inferenceProfile: string;
  status: string;
  manifest: unknown;
  accessRole?: AccessRole;
  createdAt: string;
  updatedAt: string;
}

export interface OntologyAccess {
  ontologyId: string;
  userId?: string;
  subjectType: OwnerType;
  subjectId: string;
  role: AccessRole;
  createdAt: string;
  updatedAt: string;
}

export interface Repository {
  id: string;
  ownerUserId: string;
  name: string;
  url: string;
  defaultRef: string;
  authSecretName?: string;
  authType: string;
  gitAuthorName?: string;
  gitAuthorEmail?: string;
  reachable: boolean;
  validationError?: string;
  lastValidatedAt?: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface Secret {
  id: string;
  userId: string;
  name: string;
  type: string;
  expiresAt?: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface EnvironmentRevision {
  buildId: string;
  jobName: string;
  status: string;
  gitRepository: string;
  gitRef: string;
  dockerfilePath: string;
  contextPath: string;
  destinationImage: string;
  createdAt: string;
}

export interface Environment {
  id: string;
  projectId: string;
  name: string;
  category: string;
  workspaceIdes: string[] | null;
  destinationImage: string;
  latestBuildId: string;
  latestStatus: string;
  latestGitRepository: string;
  latestGitRef: string;
  latestDockerfilePath: string;
  latestImageSizeGiB?: string;
  updatedAt: string;
  revisions: EnvironmentRevision[] | null;
}

export interface Build {
  id: string;
  projectId: string;
  gitRepository: string;
  gitRef: string;
  dockerfilePath: string;
  dockerfileContent?: string;
  contextPath: string;
  destinationImage: string;
  jobName: string;
  status: string;
  createdAt: string;
}

export interface HardwareTier {
  id: string;
  name: string;
  description?: string;
  cpuLimit: string;
  memoryLimit: string;
  ephemeralStorageLimit: string;
  default: boolean;
}

export interface EgressRule {
  id: string;
  projectId: string;
  requesterId: string;
  subjectType: OwnerType;
  subjectId: string;
  profile: string;
  destination: string;
  port: number;
  protocol: string;
  workloadTypes: string[] | null;
  justification: string;
  status: string;
  reviewerId: string;
  decisionNote: string;
  expiresAt?: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface EgressProfile {
  id: string;
  name: string;
  description: string;
  default: boolean;
  hdsAllowed: boolean;
  adminOnly: boolean;
}

export interface StorageEndpoint {
  id: string;
  name: string;
  provider: string;
  endpoint: string;
  region: string;
  classification: string;
  purpose: string;
  useSSL: boolean;
  defaultBackup: boolean;
  defaultDataset: boolean;
  status: string;
  statusMessage?: string;
  lastCheckedAt?: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

/** Parsed contents of BackupRun.report, which the API returns as a JSON
 *  string. `warnings` is where an incomplete backup declares itself. */
export interface BackupReport {
  bytes?: number;
  status?: string;
  warnings?: string[];
  objectKey?: string;
  manifestSha256?: string;
}

export interface BackupRun {
  id: string;
  status: string;
  createdBy: string;
  bucket: string;
  prefix: string;
  objectKey: string;
  report?: string;
  error?: string;
  startedAt: string;
  endedAt?: string | null;
}

export interface BackupConfigStatus {
  configured: boolean;
  bucket?: string;
  prefix?: string;
  region?: string;
  endpoint?: string;
  updatedAt?: string | null;
}

export interface AuditEvent {
  id: string;
  occurredAt: string;
  actorUserId: string;
  actorIp: string;
  actorUserAgent: string;
  action: string;
  resourceType: string;
  resourceId: string;
  projectId?: string;
  outcome: string;
  errorCode?: string;
  details?: Record<string, unknown>;
}

export interface PlatformUser {
  id: string;
  username: string;
  email: string;
  enabled: boolean;
}

export interface Organization {
  id: string;
  name: string;
  alias: string;
  enabled: boolean;
}

export interface OrganizationMember {
  userId: string;
  username?: string;
  email?: string;
}

export interface PodInfo {
  name: string;
  namespace?: string;
  status: string;
  cpu?: string;
  memory?: string;
  projectId?: string;
  kind?: string;
  createdAt?: string;
}

export interface Execution {
  id: string;
  kind: string;
  name: string;
  projectId: string;
  /** Resolved server-side, so the UI does not need to join against projects. */
  projectName: string;
  runtimeName: string;
  status: string;
  createdAt: string;
}

export interface WorkloadMetrics {
  pods: number;
  running: number;
  pending: number;
  cpuRequestMillicores: number;
  memoryRequestBytes: number;
}

export interface AdminOverview {
  counts: {
    active: number;
    apps: number;
    builds: number;
    datasets: number;
    jobs: number;
    projects: number;
    users: number;
    workspaces: number;
  };
  workloadMetrics: WorkloadMetrics;
}

export interface PlatformOverview {
  counts: {
    active: number;
    datasets: number;
    projects: number;
    users: number;
  };
  sampledAt: string;
  storage: {
    bytes: number;
    datasetsMeasured: number;
    datasetsTotal: number;
  };
  workloadMetrics: WorkloadMetrics;
}

export interface AdminInventory {
  datasets: Dataset[];
  projects: Project[];
  users: PlatformUser[];
}

export interface DataUsageNode {
  id: string;
  kind: string;
  label: string;
  subLabel: string;
  class: string;
}

export interface DataUsageEdge {
  from: string;
  to: string;
  relation: string;
  projectId: string;
}

export interface DataUsageReport {
  generatedAt: string;
  summary: {
    datasets: number;
    hdsDatasets: number;
    projects: number;
    users: number;
    organizations: number;
    workloads: number;
    edges: number;
  };
  nodes: DataUsageNode[];
  edges: DataUsageEdge[];
}

export interface RbacSubject {
  type: string;
  id: string;
  name: string;
}

export interface RbacResource {
  type: string;
  id: string;
  name: string;
  ownerType: string;
  ownerId: string;
  ownerName: string;
  classification: string;
}

export interface RbacCell {
  subjectType: string;
  subjectId: string;
  subjectName: string;
  resourceType: string;
  resourceId: string;
  resourceName: string;
  role: string;
  source: string;
  inherited: boolean;
}

export interface RbacMatrixReport {
  generatedAt: string;
  summary: {
    users: number;
    organizations: number;
    projects: number;
    datasets: number;
    ontologies: number;
    datasources: number;
    grants: number;
    inherited: number;
  };
  subjects: RbacSubject[];
  resources: RbacResource[];
  cells: RbacCell[];
}

export interface VersionInfo {
  version: string;
  backendVersion: string;
  edition: string;
  defaultTheme: string;
}

export interface UserPreferences {
  organizations: Organization[];
}

export interface AssistantMessage {
  role: 'user' | 'assistant' | 'system';
  content: string;
}

export interface AssistantChatResponse {
  reply?: string;
  message?: string;
  content?: string;
  draft?: AssistantIssueDraft | null;
}

export interface AssistantIssueDraft {
  title: string;
  body: string;
  labels?: string[];
  repository?: string;
  url?: string;
}

export type HealthSeverity = 'critical' | 'warning' | 'info';

export interface HealthAlert {
  severity: HealthSeverity;
  source: string;
  summary: string;
  detail?: string;
  since?: string;
  /** Administration section an operator should open to act on this. */
  action?: string;
}

export interface HealthReport {
  generatedAt: string;
  status: 'healthy' | 'degraded' | 'critical';
  alerts: HealthAlert[];
}

export interface ModuleInfo {
  id: string;
  name: string;
  enabled: boolean;
  version?: string;
}

