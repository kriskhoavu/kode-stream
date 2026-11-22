export type RuntimeType = 'docker-compose' | 'procfile' | 'makefile' | 'custom';
export type RebuildPolicy = 'never' | 'changed-only' | 'always';
export type RuntimeHealthCheckType = 'http' | 'command';

export interface RuntimeHealthCheck {
  name?: string;
  type: RuntimeHealthCheckType;
  target: string;
  timeoutSeconds?: number;
}

export interface RuntimeVerifyCommands {
  smoke: string;
  critical?: string;
  full?: string;
}

export interface RuntimeCommandSet {
  up: string;
  down: string;
  rebuildChanged?: string;
  verify: RuntimeVerifyCommands;
}

export interface RuntimeArtifacts {
  paths?: string[];
}

export type AutomationRunner = 'cypress' | 'playwright';

export interface RuntimeAutomationConfig {
  enabled: boolean;
  repositoryPath?: string;
  runner?: AutomationRunner;
  defaultEnvironment?: string;
  commandTemplate?: string;
  artifactPaths?: string[];
}

export interface WorkspaceRuntimeConfig {
  type: RuntimeType;
  configPath?: string;
  rebuildPolicy?: RebuildPolicy;
  commands: RuntimeCommandSet;
  healthChecks?: RuntimeHealthCheck[];
  artifacts?: RuntimeArtifacts;
  automation?: RuntimeAutomationConfig;
}

export type VerifyProfile = 'smoke' | 'critical' | 'full';
export type VerificationRunMode = 'runtime' | 'automation';
export type AutomationDisplayMode = 'silent' | 'visible';
export type VerificationStatus = 'queued' | 'running' | 'passed' | 'failed';
export type VerificationFailureType = 'boot_failure' | 'test_failure' | 'infra_failure';

export interface VerificationStepResult {
  step: string;
  status: 'ok' | 'failed' | string;
  message?: string;
  durationMs: number;
  at: string;
}

export interface RunArtifact {
  kind: string;
  root?: 'workspace' | 'automation' | string;
  path: string;
  sizeBytes: number;
  createdAt: string;
}

export interface VerificationJob {
  id: string;
  workspaceId: string;
  mode?: VerificationRunMode;
  profile: VerifyProfile;
  environment?: string;
  displayMode?: AutomationDisplayMode | string;
  selectedSpecs?: string[];
  automationRepoPath?: string;
  renderedCommand?: string;
  status: VerificationStatus;
  failureType?: VerificationFailureType;
  exitCode: number;
  trigger?: string;
  provider?: string;
  sessionId?: string;
  terminalMode?: 'embedded' | 'external' | string;
  startedAt?: string;
  finishedAt?: string;
  steps: VerificationStepResult[];
  artifacts: RunArtifact[];
  runtime?: WorkspaceRuntimeConfig;
	startFingerprint?: RepositoryFingerprint;
	finishFingerprint?: RepositoryFingerprint;
	currentFingerprint?: RepositoryFingerprint;
	freshness?: VerificationFreshness;
}

export type VerificationFreshness = 'fresh' | 'stale' | 'inconclusive';
export interface RepositoryFingerprint { value: string; branch: string; commit: string; }

export interface VerificationTestSelection {
	 expectedRevision?: string;
	 revision?: string;
  selectedSpecs: string[];
  environment?: string;
  displayMode?: AutomationDisplayMode;
  updatedAt?: string;
}

export interface DiscoveredVerificationSpec {
  path: string;
  runner: AutomationRunner | string;
  sourcePath?: string;
}

export interface ItemVerificationTests {
  selection: VerificationTestSelection;
  discoveredSpecs: DiscoveredVerificationSpec[];
}

export interface E2ELatestResult {
  status: string;
  freshness?: 'fresh' | 'stale' | 'unknown';
  recordedAt?: string;
  provider?: string;
  environment?: string;
  failedStep?: string;
  evidence: string[];
}

export interface E2ERunbook {
  title: string;
  path: string;
  source: 'plan' | 'wiki' | string;
  resultPath: string;
  latestResult?: E2ELatestResult;
  diagnostic?: string;
}

export interface E2ERunbookList {
  runbooks: E2ERunbook[];
  diagnostic?: string;
}

export interface CreateVerificationJobInput {
  profile?: VerifyProfile;
  mode?: VerificationRunMode;
  environment?: string;
  displayMode?: AutomationDisplayMode;
  selectedSpecs?: string[];
  trigger?: string;
  provider?: string;
  sessionId?: string;
  terminalMode?: string;
}
