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
  /** What a screen shows for the owner: a username, or an organization's name
   *  rather than its identifier. Resolved by the platform, because a member
   *  who is not an administrator cannot list organizations to resolve it. */
  ownerName?: string;
  canManageOwner?: boolean;
  /** The caller's effective role, personal and organization grants combined. */
  role?: ProjectRole | '';
  /** Whether the caller may change membership. Decided by the backend, which
   *  is also the thing that enforces it. */
  canManageMembers?: boolean;
  /** Volume every workspace of this project gets, as a Kubernetes quantity.
   *  Absent means the project follows the platform default. */
  workspaceStorageSize?: string;
  createdAt: string;
  updatedAt: string;
  lastActivityAt: string;
  runningApps: number;
  runningJobs: number;
  runningWorkspaces: number;
  /** True when this row is on your screen only because you administer the
   *  platform - you hold no grant on it. */
  adminVisible?: boolean;
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
  /** What a screen shows: an organization is stored by its identifier, which
   *  reads as nothing at all on a list. */
  ownerName?: string;
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
  /** True when this row is on your screen only because you administer the
   *  platform - you hold no grant on it. */
  adminVisible?: boolean;
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
  ownerName?: string;
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
  /** True when this row is on your screen only because you administer the
   *  platform - you hold no grant on it. */
  adminVisible?: boolean;
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
  /** What this repository's token can do beyond cloning, most alarming first.
   *  Empty when the provider does not say - a fine-grained GitHub token
   *  reports nothing, and silence is not evidence of anything. */
  tokenExcessScopes?: string[];
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
  /** What a person refers to: revision 3 of "training", not a449755a. Counted
   *  per environment, oldest first, so a pinned revision keeps its meaning. */
  number: number;
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
  /** What the registry found in this environment's image. Absent when the
   *  registry does not scan - an empty report must never read as a clean
   *  one, so the screen shows nothing rather than zero. */
  vulnerabilities?: {
    critical: number;
    high: number;
    medium: number;
    low: number;
    unknown: number;
    total: number;
    severity?: string;
    scannedAt?: string;
  };
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

/** What an administrator edits. Requests never reach the tier list a user
 *  picks from: they are how the cluster is packed, not what the machine can
 *  do. */
/** What an account owns personally, by name. Listed rather than counted: an
 *  administrator deciding who inherits a project needs to know which one. */
/** What a project may run at the same time. 0 on a dimension means no limit -
 *  which is what every project has until somebody sets one. */
export interface ProjectQuota {
  projectId: string;
  maxVcpu: number;
  maxMemoryGib: number;
  maxWorkspaces: number;
  maxJobs: number;
}

/** What it is running now. Always read together with the quota: a limit
 *  without the current usage says nothing about whether anything can start. */
export interface ProjectQuotaUsage {
  vcpu: number;
  memoryGib: number;
  workspaces: number;
  jobs: number;
}

export interface ProjectQuotaState {
  quota: ProjectQuota;
  usage: ProjectQuotaUsage;
  limited: boolean;
}

/** What a project consumed over a window. vCPU-hours is the number that means
 *  something across machines of different sizes: two cores for three hours is
 *  six. */
export interface UsageTotal {
  projectId: string;
  from: string;
  to: string;
  vcpuHours: number;
  memoryGibHours: number;
  /** How many measurements the total rests on: three samples and a month of
   *  them should not read the same. */
  samples: number;
  peakVcpu: number;
  peakMemoryGib: number;
}

export interface UsageSample {
  projectId: string;
  at: string;
  vcpu: number;
  memoryGib: number;
  workspaces: number;
  jobs: number;
}

export interface OwnedResources {
  projects?: string[];
  datasets?: string[];
  ontologies?: string[];
  apps?: string[];
  datasources?: string[];
  repositories?: string[];
}

export interface SmtpSettings {
  host: string;
  port: string;
  from: string;
  fromDisplayName: string;
  replyTo: string;
  user: string;
  auth: boolean;
  starttls: boolean;
  ssl: boolean;
  /** Whether a password is stored. The password itself never comes back. */
  passwordSet: boolean;
}

export interface SmtpState {
  settings: SmtpSettings;
  /** A host and a sender: what it takes for the platform to actually send. */
  configured: boolean;
}

export interface AdminHardwareTier extends HardwareTier {
  cpuRequest: string;
  memoryRequest: string;
  ephemeralStorageRequest: string;
  position: number;
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




export interface SearchResult {
  kind: string;
  id: string;
  label: string;
  sublabel?: string;
  projectId?: string;
}

export interface EffectiveSetting {
  key: string;
  envVar: string;
  kind: 'duration' | 'string' | 'url' | 'enum';
  label: string;
  description: string;
  values?: string[];
  fallback: string;
  secret: boolean;
  /** A fact rather than a setting: determined by the build or the deployment,
   *  shown for visibility and refused for writing. */
  readOnly: boolean;
  value: string;
  /** Where the current value comes from, so an operator can tell why it is
   *  what it is rather than guessing between manifest and environment. */
  source: 'stored' | 'environment' | 'default' | 'build';
  overridable: boolean;
}

export type HealthSeverity = 'critical' | 'warning' | 'info';

export interface HealthAlert {
  severity: HealthSeverity;
  /**
   * Whose problem this is. A failed job belongs to whoever ran it and has
   * context on their own screen; a platform condition belongs to whoever keeps
   * the installation alive. Only platform conditions are alerted on and kept
   * in the history.
   */
  scope: 'platform' | 'user';
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

/** One condition, from the moment it was observed to the moment it cleared. */
export interface HealthEvent {
  id: string;
  key: string;
  source: string;
  severity: HealthSeverity;
  summary: string;
  detail?: string;
  raisedAt: string;
  /** Absent while the condition is still current. */
  resolvedAt?: string;
}

export interface HealthHistory {
  items: HealthEvent[];
  since?: string;
  /**
   * False when no store is configured. Distinguished from an empty history on
   * purpose: "nothing recorded" and "nothing happened" are different answers,
   * and conflating them makes a silent platform read as a healthy one.
   */
  recording: boolean;
}

/** A credential a user presents instead of a browser session. */
export interface ApiToken {
  id: string;
  userId: string;
  /** Named by its owner, so revoking the right one does not require guessing. */
  name: string;
  createdAt: string;
  expiresAt?: string;
  revokedAt?: string;
  lastUsedAt?: string;
  /** What the token may do, below what its owner may do. Absent or ["full"]
   *  means unrestricted, which is what tokens were before scopes existed. */
  scopes?: string[];
}

/** A project role held by every member of an organization. */
export interface ProjectOrganizationRole {
  organizationId: string;
  organizationName?: string;
  role: 'viewer' | 'editor' | 'admin';
}

/** What the platform returns after creating an account or resetting a password. */
export interface CreatedUser {
  userId: string;
  username?: string;
  /** Shown once and not recoverable; the user must change it at first sign-in. */
  temporaryPassword: string;
  note?: string;
}

/** One piece of software the platform ships. */
export interface InventoryItem {
  name: string;
  version: string;
  licence: string;
  /** platform, backend, frontend, infrastructure, or build tooling. */
  component: string;
  /** Where the licence came from: read from the dependency, stated by us, or unresolved. */
  origin: 'detected' | 'declared' | 'unresolved';
  role?: string;
  upstream?: string;
}

export interface SoftwareInventory {
  generatedAt: string;
  note: string;
  counts: { total: number; unknown: number };
  items: InventoryItem[];
}

export interface ModuleInfo {
  id: string;
  name: string;
  enabled: boolean;
  version?: string;
}


/** What `GET /builds/{id}/dockerfile` answers: the file, and where it was read
 *  from. A system environment reads it from the repository the platform was
 *  built from, so `sourceUrl` points at that file rather than at a build. */
export interface DockerfileResponse {
  buildId: string;
  projectId?: string;
  gitRepository?: string;
  gitRef?: string;
  dockerfilePath?: string;
  sourceUrl?: string;
  content: string;
}

/** A project's environment variable: what the work needs, as opposed to what a
 *  person carries. `value` is absent for a member who may not read it. */
export interface ProjectVariable {
  name: string;
  value?: string;
  description?: string;
  updatedBy?: string;
  updatedAt: string;
}

/** One row of an ontology filter: the object, what it is, and what it holds.
 *  The API answers with these; it does not answer with columns and rows. */
export interface OntologyQueryItem {
  object: string;
  type: string;
  parent: string;
  attributes?: string[];
  references?: string[];
  links?: string[];
  count: number;
  bytes: number;
}

/** Whether an ontology still describes its source. An ontology is a
 *  photograph presented as a fact: the June scan of a study said "18,738
 *  objects" with the same confidence as September's 24,179, and nothing on
 *  screen said the study had gained eleven subjects in between. */
export interface OntologyFreshness {
  generatedAt: string;
  ageDays: number;
  manifestObjects: number;
  sourceObjects: number;
  drift: number;
  stale: boolean;
  checkedAt: string;
}

/** Who the study covers, and who a cohort assembled by modality would leave
 *  out. Computed from the stored manifest, so it answers for ontologies
 *  scanned months ago too. */
export interface OntologyModalityCoverage {
  name: string;
  subjects: number;
  objects: number;
  missingSubjects: string[];
}

export interface OntologyCompleteness {
  subjects: number;
  modalities: OntologyModalityCoverage[];
  completeSubjects: number;
}

/** A named selection of files, frozen when it is declared. It duplicates
 *  nothing: the paths point into the dataset where the data already lives, and
 *  a mount builds a tree of links over them. */
export interface Cohort {
  id: string;
  ontologyId: string;
  projectId: string;
  ownerUserId: string;
  name: string;
  description: string;
  subjects: string[];
  modalities: string[];
  visits: string[];
  objectCount: number;
  totalBytes: number;
  createdAt: string;
  updatedAt: string;
}

/** What a scan reports back about what it just read. */
export interface OntologyManifestSummary {
  study?: string;
  summary?: {
    subjects?: number;
    visits?: number;
    modalities?: number;
    objects?: number;
    unrecognised?: number;
  };
}

/** How big a dataset is. Nothing stores it - an object store knows its size
 *  only by being listed - so it is measured on demand and cached. */
export interface DatasetUsage {
  objects: number;
  totalBytes: number;
  truncated: boolean;
  measuredAt: string;
}

/** What every log endpoint answers with. The payload carries the identifiers
 *  alongside the text, so a caller that wants the text has to reach for it. */
export interface LogsResponse {
  logs?: string;
  pending?: boolean;
  persisted?: boolean;
}

