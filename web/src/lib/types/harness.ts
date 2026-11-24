import { type ClaudeTurn } from './canvas';

// Timestamps are RFC 3339 strings. Costs are US dollars.

export type HarnessSourceKind = 'jira' | 'github' | 'bitbucket' | 'local';
export type HarnessTaskState = 'discovered' | 'ranked' | 'running' | 'blocked' | 'done' | 'dismissed' | 'stale' | 'cancelled';
export type HarnessRunState = 'running' | 'blocked' | 'passed' | 'failed' | 'cancelled';
export type HarnessStageKind = 'plan' | 'plan_approval' | 'implement' | 'qa' | 'review' | 'draft_pr';
export type HarnessStageState = 'pending' | 'running' | 'awaiting_approval' | 'passed' | 'failed' | 'retry_wait' | 'cancelled' | 'skipped';
export type HarnessAgentRole = 'planner' | 'plan_reviewer' | 'coder' | 'code_reviewer' | 'qa_fixer';
export type HarnessAgentRunState = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'interrupted';
/** `clarify` waits for the ticket's reporter, not the assignee. */
export type HarnessApprovalGate = 'start' | 'plan' | 'resume' | 'review' | 'question' | 'clarify';
export type HarnessApprovalState = 'requested' | 'approved' | 'rejected' | 'expired' | 'superseded';
export type HarnessChannel = 'dashboard' | 'webex' | 'api' | 'source';
export type HarnessRoleId = 'scout' | 'sorter' | 'planner' | 'coder' | 'reviewer' | 'qa';
export type HarnessEngineId = 'claudecode' | 'codex' | 'opencode';
/** `replan` sends a Review back to Plan. `reject` sends it back to Build. */
export type HarnessDecision = 'approve' | 'reject' | 'replan';

export interface HarnessActor { actorId: string; displayName: string; email?: string }

export interface HarnessTaskSource {
  kind: HarnessSourceKind;
  key: string;
  url?: string;
  status?: string;
  labels: string[];
  priority?: string;
  assignee?: string;
  updatedAt?: string;
  reporter?: { name: string; mention?: string };
}

export interface HarnessTask {
  id: string;
  workspaceId: string;
  source: HarnessTaskSource;
  title: string;
  state: HarnessTaskState;
  score: number;
  scoreReasons: string[];
  priorityOverride: number;
  pinned: boolean;
  needsYou: boolean;
  waitingFor?: string;
  startRequested?: boolean;
  /** Null or absent follows the workspace, then the settings. */
  askQuestions?: boolean | null;
  note?: string;
  runId?: string;
  attempts: number;
  revision: number;
  dependsOn: string[];
  milestone?: string;
  epic?: string;
  risk?: 'low' | 'medium' | 'high';
  estimatedMinutes?: number;
  finishedOutside?: { by?: string; at?: string };
  createdAt?: string;
  updatedAt?: string;
}

export interface HarnessCreateTaskInput { workspaceId: string; key: string; title: string; body?: string; url?: string; startNow?: boolean; idempotencyKey: string }

export interface HarnessRunFile { path: string; status: 'added' | 'modified' | 'deleted'; content: string; truncated?: boolean }
export interface HarnessRunFileDiff { path: string; diff: string; truncated: boolean; from: 'worktree' | 'branch' | '' }
export interface HarnessRunPlan { branch: string; from: 'worktree' | 'branch' | ''; files: HarnessRunFile[] }

export interface HarnessTotals { turns: number; durationMs: number; costUsd: number; blocking: number; advisory: number }

export interface HarnessStage {
  id: string;
  runId: string;
  kind: HarnessStageKind;
  ordinal: number;
  state: HarnessStageState;
  round: number;
  maxRounds: number;
  attempt: number;
  maxAttempts: number;
  engine?: HarnessEngineId;
  model?: string;
  turns: number;
  durationMs: number;
  costUsd: number;
  blocking: number;
  advisory: number;
  startedAt?: string;
  finishedAt?: string;
  failureReason?: string;
}

export interface HarnessFinding {
  severity: 'blocking' | 'advisory';
  title: string;
  path?: string;
  line?: number;
  detail?: string;
  resolved?: boolean;
}

export interface HarnessAgentRun {
  id: string;
  runId: string;
  stageId: string;
  role: HarnessAgentRole;
  round: number;
  attempt?: number;
  engine: HarnessEngineId;
  model: string;
  state: HarnessAgentRunState;
  startedAt?: string;
  finishedAt?: string;
  turns: number;
  durationMs: number;
  costUsd: number;
  verdict?: 'approve' | 'changes_requested';
  blocking: number;
  advisory: number;
  summary?: string;
  progress?: string;
  findings?: HarnessFinding[];
  stopReason?: string;
}

export interface HarnessRun {
  id: string;
  taskId: string;
  workspaceId: string;
  branch: string;
  state: HarnessRunState;
  currentStageId?: string;
  totals: HarnessTotals;
  prUrl?: string;
  remoteBranch?: string;
  startedAt: string;
  finishedAt?: string;
  revision: number;
  currentStage?: HarnessStage;
  activeAgentRun?: HarnessAgentRun;
}

export interface HarnessApproval {
  id: string;
  runId?: string;
  stageId?: string;
  taskId: string;
  gate: HarnessApprovalGate;
  state: HarnessApprovalState;
  requestedAt: string;
  expiresAt?: string;
  decidedAt?: string;
  decidedBy?: HarnessActor;
  channel?: HarnessChannel;
  reason?: string;
  revision: number;
  summary?: string;
  estimate?: { minMinutes?: number; maxMinutes?: number; costUsd?: number };
  context?: { difficulty?: 'easy' | 'hard'; subtasks?: number; rounds?: number; blocking?: number; advisory?: number; costUsd?: number; durationMs?: number; verdict?: 'approve' | 'changes_requested'; questions?: string[]; role?: HarnessAgentRole; reporter?: string; draft?: string };
}

export interface HarnessTranscript { agentRunId: string; sessionId?: string; available: boolean; turns: ClaudeTurn[] }

export interface HarnessRunDetail {
  run: HarnessRun;
  task: HarnessTask;
  stages: HarnessStage[];
  agentRuns: HarnessAgentRun[];
  approvals: HarnessApproval[];
}

export interface HarnessRoleHealth {
  role: HarnessRoleId;
  paused: boolean;
  pausedBy?: string;
  pausedAt?: string;
  working: number;
  slots: number;
  queued: number;
  lastRunAt?: string;
  found?: number;
  model?: string;
  engine?: HarnessEngineId;
}

export interface HarnessEngineStatus {
  id: HarnessEngineId;
  available: boolean;
  version?: string;
  limits?: string[];
}

export interface HarnessStatus {
  autoStart?: boolean;
  askQuestions?: boolean;
  enabled: boolean;
  storageSupported: boolean;
  mode: 'local' | 'cloud';
  running: boolean;
  waitingCount: number;
  lastEventSeq: number;
  roles: HarnessRoleHealth[];
  engines: HarnessEngineStatus[];
}

export interface HarnessSourcePoll {
  workspaceId: string;
  kind: string;
  lastPolledAt?: string;
  found: number;
  added: number;
  lastError?: string;
}

export interface HarnessCostGroup { key: string; costUsd: number; agentRuns: number; turns: number; durationMs: number }
export interface HarnessCostSummary {
  groupBy: 'day' | 'run' | 'role' | 'model';
  from: string;
  to: string;
  totalUsd: number;
  agentRuns: number;
  groups: HarnessCostGroup[];
}

export interface HarnessRolePolicy {
  engine: HarnessEngineId;
  model: string;
  hardModel?: string;
  maxTurns: number;
}

export interface HarnessSourceSettings {
  workspaceId: string;
  kind: HarnessSourceKind;
  enabled: boolean;
  query: string;
  labels?: string[];
  tokenEnvVar?: string;
  /** Jira only. Absent means off. */
  checkReady?: boolean;
  /** Jira only. Empty means READY. */
  readyStatuses?: string[];
  lastCheckedAt?: string;
  lastFound?: number;
  lastError?: string;
}

export interface HarnessSettings {
  revision: number;
  enabled: boolean;
  autoStart: boolean;
  requireReview: boolean;
  askQuestions: boolean;
  slots: Record<HarnessRoleId, number>;
  paused: Partial<Record<HarnessRoleId, boolean>>;
  sources: HarnessSourceSettings[];
  modelPolicy: Record<HarnessAgentRole, HarnessRolePolicy>;
  /** When true, modelPolicy only seeds a new workspace. */
  workspaceModels?: boolean;
  workspaces: HarnessWorkspaceSettings[];
}

export interface HarnessRoleModel {
  engine?: HarnessEngineId;
  model?: string;
  highDifficultyModel?: string;
  maxTurns?: number;
}

export interface HarnessProcedureStep {
  skill: string;
  track?: string;
}

export interface HarnessWorkspaceSettings {
  workspaceId: string;
  enabled?: boolean | null;
  autoStart?: boolean | null;
  requireReview?: boolean | null;
  askQuestions?: boolean | null;
  verificationCommands: string[];
  scope: string[];
  models: Partial<Record<HarnessAgentRole, HarnessRoleModel>>;
  procedure: Partial<Record<HarnessAgentRole, HarnessProcedureStep>>;
}

export type SettingsFilePart = 'provider' | 'pipeline' | 'tests' | 'scope' | 'models' | 'procedure' | 'sources';

export interface SettingsFileContent {
  version: number;
  ai?: { provider: string };
  harness?: {
    enabled?: boolean;
    autoStart?: boolean;
    review?: boolean;
    askQuestions?: boolean;
    tests?: string[];
    scope?: string[];
    models?: Partial<Record<HarnessAgentRole, { engine?: string; model?: string; hardModel?: string; maxTurns?: number }>>;
    procedure?: Partial<Record<HarnessAgentRole, HarnessProcedureStep>>;
    sources?: { kind: HarnessSourceKind; enabled: boolean; query: string; labels?: string[]; tokenEnvVar?: string }[];
  };
}

export interface SettingsFileView {
  path: string;
  exists: boolean;
  error?: string;
  file?: SettingsFileContent;
  has: SettingsFilePart[];
  current: SettingsFileContent;
  parts: SettingsFilePart[];
}

export type HarnessEvent =
  | { seq: number; at: string; type: 'task.discovered' | 'task.updated' | 'task.ranked' | 'task.pinned'; task: HarnessTask }
  | { seq: number; at: string; type: 'task.removed'; taskId: string }
  | { seq: number; at: string; type: 'run.started' | 'run.blocked' | 'run.finished'; run: HarnessRun; task?: HarnessTask }
  | { seq: number; at: string; type: 'stage.started' | 'stage.round' | 'stage.finished'; stage: HarnessStage }
  | { seq: number; at: string; type: 'agent.started' | 'agent.progress' | 'agent.finished'; agentRun: HarnessAgentRun }
  | { seq: number; at: string; type: 'approval.requested' | 'approval.decided'; approval: HarnessApproval; task?: HarnessTask }
  | { seq: number; at: string; type: 'role.paused' | 'role.resumed'; role: HarnessRoleHealth }
  | { seq: number; at: string; type: 'notification.failed'; message: string };
