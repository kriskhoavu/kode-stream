import { type ClaudeTurn } from './canvas';

// The engineering harness.
// Mirrors the backend contract in plans/platform/PM-044 (API section). Every
// timestamp is an RFC 3339 string; every cost is US dollars as a number.

/** `local` reads tasks from Markdown files in a folder; its hand-off pushes and opens no pull request. */
export type HarnessSourceKind = 'jira' | 'github' | 'bitbucket' | 'local';
export type HarnessTaskState = 'discovered' | 'ranked' | 'running' | 'blocked' | 'done' | 'dismissed' | 'stale' | 'cancelled';
export type HarnessRunState = 'running' | 'blocked' | 'passed' | 'failed' | 'cancelled';
export type HarnessStageKind = 'plan' | 'plan_approval' | 'implement' | 'qa' | 'review' | 'draft_pr';
export type HarnessStageState = 'pending' | 'running' | 'awaiting_approval' | 'passed' | 'failed' | 'retry_wait' | 'cancelled' | 'skipped';
export type HarnessAgentRole = 'planner' | 'plan_reviewer' | 'coder' | 'code_reviewer' | 'qa_fixer';
export type HarnessAgentRunState = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'interrupted';
/** `clarify` waits for a ticket's reporter, not for the person working on it. */
export type HarnessApprovalGate = 'start' | 'plan' | 'resume' | 'review' | 'question' | 'clarify';
export type HarnessApprovalState = 'requested' | 'approved' | 'rejected' | 'expired' | 'superseded';
/** `source` is a decision the harness took on what the ticket said, such as its reporter replying. */
export type HarnessChannel = 'dashboard' | 'webex' | 'api' | 'source';
/** The orchestrator's loops a person can pause; `reviewer` covers plan and code review. */
export type HarnessRoleId = 'scout' | 'sorter' | 'planner' | 'coder' | 'reviewer' | 'qa';
export type HarnessEngineId = 'claudecode' | 'codex' | 'opencode';
/** `replan` answers Review only: it sends the change back to Plan; `reject` sends it back to Build. */
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
  /** Who opened the issue, where the source names them. */
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
  /** Who the run waits for at a clarify gate: the ticket's reporter, not you. */
  waitingFor?: string;
  /** A person pressed Start; the next free Planner takes it. */
  startRequested?: boolean;
  /** This task's own answer to "may an agent stop and ask"; absent follows the workspace, then the settings. */
  askQuestions?: boolean | null;
  note?: string;
  runId?: string;
  attempts: number;
  revision: number;
  /** Ids of the tasks this one waits for: the Task Queue says "Waits for" and starts it only after them. */
  dependsOn: string[];
  milestone?: string;
  epic?: string;
  risk?: 'low' | 'medium' | 'high';
  estimatedMinutes?: number;
  /** Set when the source closed the task outside the harness ("Done by hand"). */
  finishedOutside?: { by?: string; at?: string };
  createdAt?: string;
  updatedAt?: string;
}

/** What a card hands the harness to run it: the ticket's key, title and body. */
export interface HarnessCreateTaskInput { workspaceId: string; key: string; title: string; body?: string; url?: string; /** Start at once (the default) or only put it in the queue. */ startNow?: boolean; idempotencyKey: string }

/** One file a run wrote under the plan folder; `content` is empty when the file is too large to show. */
export interface HarnessRunFile { path: string; status: 'added' | 'modified' | 'deleted'; content: string; truncated?: boolean }
/** The plan a run wrote, readable before it is merged: live from the run's own copy, or from its branch. */
/** The diff of one path a run changed, read from its own copy or its branch. */
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
  /** True once a later round no longer reports it. */
  resolved?: boolean;
}

export interface HarnessAgentRun {
  id: string;
  runId: string;
  stageId: string;
  role: HarnessAgentRole;
  round: number;
  /** The try within the round; a run interrupted or without a report is tried again. */
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
  /** The last ≤200 characters the engine reported, for live progress. */
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
  /** The name the branch is pushed under. */
  remoteBranch?: string;
  startedAt: string;
  finishedAt?: string;
  revision: number;
  /** Embedded by the list endpoint so the Inbox needs no second request. */
  currentStage?: HarnessStage;
  activeAgentRun?: HarnessAgentRun;
}

export interface HarnessApproval {
  id: string;
  runId?: string;
  /** The stage that asked, when a stage did: two Reviews of one run are told apart by it. */
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
  /** Plain summary of what is being approved, written by the harness (≤600). */
  summary?: string;
  /** What the next step usually takes, shown in the consequence sentence. */
  estimate?: { minMinutes?: number; maxMinutes?: number; costUsd?: number };
  context?: { difficulty?: 'easy' | 'hard'; subtasks?: number; rounds?: number; blocking?: number; advisory?: number; costUsd?: number; durationMs?: number; verdict?: 'approve' | 'changes_requested'; questions?: string[]; role?: HarnessAgentRole; reporter?: string; draft?: string };
}

/** The last turns of an agent run's own transcript, read from the coding tool's file when asked for. */
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
  /** Finder only: tasks found by the last look. */
  found?: number;
  model?: string;
  engine?: HarnessEngineId;
}

export interface HarnessEngineStatus {
  id: HarnessEngineId;
  available: boolean;
  version?: string;
  /** Plain notes on what this engine cannot do (from Detect). */
  limits?: string[];
}

export interface HarnessStatus {
  /** False means a task waits for a person to press Start. */
  autoStart?: boolean;
  /** Whether an agent may stop and ask a person, unless a workspace or task says otherwise. */
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

/** What one source returned when the Finder asked it. */
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
  /** Builder only: the model used when the plan rates the work hard. */
  hardModel?: string;
  maxTurns: number;
}

export interface HarnessSourceSettings {
  workspaceId: string;
  kind: HarnessSourceKind;
  enabled: boolean;
  /** Jira search query, GitHub `owner/repo`, Bitbucket `workspace/repo`, or a local task folder. */
  query: string;
  labels?: string[];
  tokenEnvVar?: string;
  /** Jira only: whether a ticket in a ready status starts on its own and is first checked for enough context. Absent means off. */
  checkReady?: boolean;
  /** Jira only: the statuses that mean ready; empty means READY. */
  readyStatuses?: string[];
  lastCheckedAt?: string;
  lastFound?: number;
  lastError?: string;
}

export interface HarnessSettings {
  revision: number;
  enabled: boolean;
  autoStart: boolean;
  /** A run whose tests passed waits at Review, where a person tries it before hand-off. */
  requireReview: boolean;
  /** Agents may stop and ask a person when they cannot decide alone. */
  askQuestions: boolean;
  slots: Record<HarnessRoleId, number>;
  paused: Partial<Record<HarnessRoleId, boolean>>;
  sources: HarnessSourceSettings[];
  /** The models a new workspace starts from; each workspace then holds its own. */
  modelPolicy: Record<HarnessAgentRole, HarnessRolePolicy>;
  /** Each workspace holds its own models; modelPolicy is only where a new one starts. */
  workspaceModels?: boolean;
  /** Each workspace's own choices over these settings. */
  workspaces: HarnessWorkspaceSettings[];
}

/** A role's CLI and model in one workspace, which holds its own. An empty hard model keeps hard work on the easy model. Tool permissions are not a workspace's to set. */
export interface HarnessRoleModel {
  engine?: HarnessEngineId;
  model?: string;
  highDifficultyModel?: string;
  maxTurns?: number;
}

/** The skill of the repository, and the track of it, a role follows in one workspace. */
export interface HarnessProcedureStep {
  skill: string;
  track?: string;
}

/** One workspace's overrides. An absent or empty field follows the defaults. */
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

/** The parts of `.kode-stream/settings.yaml` a person exports or imports one at a time. */
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

/** A workspace's shared settings file: whether its checkout has one, what it holds, and what an export would write now. */
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
  | { seq: number; at: string; type: 'run.started' | 'run.blocked' | 'run.finished'; run: HarnessRun; /** The run's task as it stands after the change. */ task?: HarnessTask }
  | { seq: number; at: string; type: 'stage.started' | 'stage.round' | 'stage.finished'; stage: HarnessStage }
  | { seq: number; at: string; type: 'agent.started' | 'agent.progress' | 'agent.finished'; agentRun: HarnessAgentRun }
  | { seq: number; at: string; type: 'approval.requested' | 'approval.decided'; approval: HarnessApproval; /** The gate's task as it stands after the change. */ task?: HarnessTask }
  | { seq: number; at: string; type: 'role.paused' | 'role.resumed'; role: HarnessRoleHealth }
  | { seq: number; at: string; type: 'notification.failed'; message: string };
