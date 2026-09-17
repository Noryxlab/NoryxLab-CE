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

/** Who consulted an application, and when.
 *
 *  This was declared by hand and never met its endpoint: it named `views`,
 *  `uniqueUsers` and `lastAccessedAt` while the API answers `totalViews`,
 *  `identifiedVisitors` and `lastViewedAt`. Nothing rendered, so nobody wired
 *  it, so nobody noticed - a type that lies is worse than no type. */
export interface AppUsageVisitor {
  userId: string;
  views: number;
  lastViewAt: string;
}

export interface AppUsageDay {
  date: string;
  views: number;
}

export interface AppUsage {
  appId: string;
  periodDays: number;
  totalViews: number;
  identifiedVisitors: number;
  anonymousViews: number;
  lastViewedAt?: string | null;
  visitors: AppUsageVisitor[];
  daily: AppUsageDay[];
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
  /** Qui porte cet identifiant. Resolu par le serveur : l'ecran affichait
   *  l'identifiant brut, donc verifier qui accede a un dataset de sante
   *  revenait a lire quatre UUID et a savoir lequel etait lequel. */
  subjectName?: string;
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
  apps?: number;
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
  /** How long an emailed password link stays valid, in hours. Read from the
   *  platform rather than written into the copy: a translated string that
   *  names its own number goes stale the day an operator changes the setting,
   *  and then the screen is telling the reader something untrue. */
  passwordLinkLifetimeHours?: number;
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
  /** Derniere connexion reussie, absente quand le compte ne s'est jamais
   *  connecte - ce qui est une reponse, et souvent la plus interessante :
   *  l'annuaire dit qui a le droit d'entrer, la plateforme dit qui est entre. */
  lastSeenAt?: string;
  /** Nombre de connexions sur ce que la retention de l'audit conserve. Une
   *  visite et deux cents ne disent pas la meme chose de la meme date. */
  signIns?: number;
}

export interface Organization {
  id: string;
  name: string;
  alias: string;
  enabled: boolean;
}

/** Un membre d'organisation, tel que l'annuaire le renvoie.
 *
 *  Le champ s'appelle `id` et non `userId` : le type declarait `userId`, que
 *  la reponse ne porte pas, donc la suppression partait vers
 *  `.../members/undefined`. La ligne s'affichait quand meme - elle retombe sur
 *  le nom d'utilisateur - et seule l'action echouait, sur un message qui
 *  parlait de l'organisation. */
export interface OrganizationMember {
  id: string;
  username?: string;
  email?: string;
  enabled?: boolean;
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
    /** Non comptés parce qu'ils sont réglementés : la plateforme n'énumère
     *  pas les clés d'un bucket de santé pour produire un chiffre d'accueil. */
    datasetsRegulated?: number;
    /** Non comptés parce qu'ils n'ont pas répondu. Autre fait, autre remède. */
    datasetsUnreadable?: number;
    /** La mesure a dépassé son délai : le chiffre est un plancher. */
    truncated?: boolean;
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
  /** Set when the pod is gone and its output cannot be recovered. */
  unavailable?: string;
}

/** Un jeton qui appartient au projet et non à une personne : il survit au
 *  départ de celui qui l'a créé, et ne sait qu'appeler les applications de son
 *  projet. */
export interface ProjectToken {
  id: string;
  userId: string;
  projectId: string;
  name: string;
  scopes?: string[];
  createdAt: string;
  expiresAt?: string | null;
  revokedAt?: string | null;
  lastUsedAt?: string | null;
}

/** Où en est le démarrage d'un workspace, et où il s'est arrêté. Cinq étapes
 *  plutôt qu'une traduction de Kubernetes : personne ne devrait avoir à lire
 *  des événements de pod pour savoir que c'est le stockage qui coince. */
export interface WorkspaceStartupStep {
  key: 'requested' | 'scheduled' | 'image' | 'storage' | 'environment';
  state: 'done' | 'running' | 'failed' | 'waiting';
  detail?: string;
  technical?: string;
}

export interface WorkspaceStartup {
  steps: WorkspaceStartupStep[];
  /** Le démarrage ne progressera pas tout seul : inutile d'attendre. */
  stuck: boolean;
}

/** L'état des services d'IA : l'assistant, l'assistant de code et les agents
 *  répondent tous par la même passerelle de modèles, donc ils partagent un état. */
/** Une instruction permanente laissee a la plateforme, ecrite par son auteur
 *  dans ses propres mots. La mission n'est jamais interpretee par la
 *  plateforme : elle part telle quelle au modele. */
/** Un groupe d'agents sur un meme sujet, avec le but ecrit par son
 *  proprietaire. Rien n'interprete ce but : c'est du texte, comme une mission. */
export interface AgentTeam {
  id: string;
  ownerUserId: string;
  projectId?: string;
  name: string;
  purpose: string;
  createdAt: string;
  updatedAt: string;
}

/** Une ligne de l'organisation : qui peut demander quoi a qui.
 *
 *  Ecrite d'avance par une personne, jamais decidee par un modele pendant
 *  qu'il travaille. On lit l'organisation pour savoir ce qui est permis,
 *  au lieu de depouiller des journaux d'execution. */
export interface AgentMandate {
  id: string;
  teamId: string;
  leadId: string;
  memberId: string;
  action: string;
  grantedByUserId: string;
  createdAt: string;
}

/** Ce que les agents d'une installation ont le droit de faire, et ce qu'ils ont
 *  fait. Lu par quelqu'un qui ne les a pas construits - sa direction des
 *  risques, son juridique, son qualite - donc tout y est nomme. */
export interface AgentGovernanceReport {
  generatedAt: string;
  windowDays: number;
  agents: AgentGovernanceRow[];
  mandates: AgentGovernanceMandate[];
  summary: {
    agents: number;
    /** Combien peuvent changer quelque chose. Ce n'est pas le meme chiffre que
     *  le nombre d'agents, et c'est celui qu'on demande en premier. */
    canAct: number;
    teams: number;
    mandates: number;
    runs: number;
    actions: number;
  };
}

export interface AgentGovernanceRow {
  agentId: string;
  name: string;
  owner: string;
  projectId: string;
  project: string;
  team?: string;
  role: Agent['role'];
  schedule: Agent['schedule'];
  enabled: boolean;
  mayDo: string[];
  runs: number;
  /** Compte les actions relevees par la plateforme, jamais celles que le
   *  rapport d'un agent affirme avoir faites. */
  acted: number;
  lastRunAt?: string;
}

export interface AgentGovernanceMandate {
  teamId: string;
  team: string;
  lead: string;
  member: string;
  action: string;
  grantedBy: string;
  since: string;
}

export interface Agent {
  id: string;
  ownerUserId: string;
  projectId?: string;
  /** L'equipe dans laquelle il travaille. Vide veut dire seul, ce que
   *  faisaient tous les agents avant qu'il y ait des equipes. */
  teamId?: string;
  /** Ce qu'il est dans cette equipe, et le plafond de ce qu'il peut detenir.
   *  Un observateur ne garde aucune action, meme si on lui en accorde une. */
  role: 'observer' | 'operator' | 'lead';
  name: string;
  mission: string;
  schedule: 'manual' | 'hourly' | 'daily';
  /** Ce que l'agent a le droit de changer. Vide veut dire qu'il regarde et
   *  rapporte, ce qui est le defaut. */
  actions: string[];
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
  lastRunAt?: string;
  lastReport?: string;
  /** Le dernier passage n'a rien trouve a signaler. Distingue d'un agent
   *  casse, qui ressemble exactement au meme silence. */
  lastQuiet: boolean;
}

export interface AgentRun {
  id: string;
  agentId: string;
  /** Ce qu'une personne a demandé, quand ce passage vient d'une question et
   *  non de l'horaire. Même fil que les passages programmés : le carnet est
   *  le récit de ce que l'agent a fait, et la première question qu'on pose
   *  sur une action est ce qui l'a déclenchée. */
  question?: string;
  report: string;
  quiet: boolean;
  /** Ce qui a reellement eu lieu, releve des appels qui ont abouti et non lu
   *  dans le rapport, qui est la seule partie qui peut se tromper avec aplomb. */
  actions: string[];
  error?: string;
  startedAt: string;
  finishedAt?: string;
}

export interface AgentInput {
  name: string;
  mission: string;
  schedule: Agent['schedule'];
  projectId?: string;
  actions: string[];
  enabled?: boolean;
  teamId?: string;
  role?: Agent['role'];
}

export interface AIServicesStatus {
  /** Faux là où aucune passerelle n'est déployée : l'interface masque alors la
   *  brique au lieu d'afficher une panne pour ce qui n'a jamais été installé. */
  configured: boolean;
  /** Vrai la ou la plateforme a de quoi faire tourner un agent : une memoire et
   *  un moteur. L'edition dit ce qui est vendu, pas ce qui est installe. */
  agents: boolean;
  mode?: 'full' | 'degraded' | 'down';
  capabilities?: Record<string, boolean>;
  detail?: string;
  checkedAt?: string;
}

/** Un role tel qu'il peut etre donne a quelqu'un ici. */
export interface AssignableRole {
  key: string;
  name: string;
  description?: string;
  /** Ce que la plateforme accorde d'elle-meme. Affiche sur un role maison
   *  parce que c'est ce que la personne obtient reellement tant que la matrice
   *  n'a rien de plus a dire. */
  basedOn: string;
  builtin: boolean;
}

/** Une ligne de la matrice : un role, et ce qu'il peut faire colonne par
 *  colonne. Les lignes verrouillees decrivent la plateforme elle-meme et ne
 *  s'editent pas - c'est la ou une installation dit ce qu'elle veut. */
export interface RbacPolicyRow {
  role: string;
  key: string;
  locked?: boolean;
  basedOn?: string;
  description?: string;
  project: string;
  dataset: string;
  ontology: string;
  datasource: string;
  environment: string;
  workload: string;
  governance: string;
}

export interface RbacPolicyResponse {
  rows: RbacPolicyRow[];
  /** Combien de sujets portent chaque role. Un role porte ne peut pas etre
   *  supprime, et le nombre dit pourquoi. */
  assignmentCounts: Record<string, number>;
  updatedAt: string;
}

/** Ce qu'un noeud peut encore accepter comme volume. */
export interface StorageCapacityNode {
  name: string;
  /** Ce que les volumes existants ont deja reserve. */
  claimed: number;
  /** Ce qu'un nouveau volume peut encore reserver ici. */
  schedulable: number;
  allocatable: number;
  /** Le disque reellement inutilise - une autre question, et un nombre en
   *  general bien plus grand. Les deux sont montres cote a cote parce que leur
   *  ecart est tout le piege. */
  freeDisk: number;
  headroomRatio: number;
}

export interface StorageCapacityReport {
  /** Faux quand la couche de stockage n'a pas pu etre interrogee. Les chiffres
   *  sont alors absents plutot que nuls : une jauge dessinee a partir de zeros
   *  se lit comme un cluster vide, soit l'inverse de ce qui est rapporte. */
  available: boolean;
  source?: string;
  detail?: string;
  nodes?: StorageCapacityNode[];
  totalClaimed: number;
  totalSchedulable: number;
  totalAllocatable: number;
  totalFreeDisk: number;
  /** Le seuil sur lequel la plateforme alerte, pour que l'ecran et l'alerte ne
   *  puissent pas diverger sur le moment de s'inquieter. */
  warnBelow: number;
}

/** Ce que la plateforme a servi, sur une periode : des personnes et des
 *  actions, par opposition a UsageTotal qui compte des vCPU-heures. */
export interface ActivityReport {
  since: string;
  until: string;
  /** Ce que l'audit detient reellement, souvent different de la periode
   *  demandee. Un rapport sur 90 jours dont les enregistrements commencent il
   *  y a 12 jours n'est pas un trimestre calme, et l'ecran doit pouvoir le
   *  dire. Absent quand l'audit est vide. */
  coversSince?: string;
  coversUntil?: string;
  totalEvents: number;
  people: { actor: string; organization?: string; events: number; lastSeen?: string }[];
  /** La coupe dans laquelle un pilote se raconte : une installation partagée
   *  entre un industriel, une école et un institut se fait demander combien
   *  chacun s'en est servi, pas combien chaque compte a fait. */
  organizations: { organization: string; people: number; events: number }[];
  actions: { action: string; count: number }[];
  /** Compte des *personnes* par jour, pas des evenements : un import massif
   *  ecrit des centaines de milliers de lignes et enterrerait une semaine de
   *  travail reel sous une seule operation machine. */
  daily: { day: string; people: number; events: number }[];
  actionsShown: number;
  actionsTotal: number;
}
