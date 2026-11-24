import { type EmbeddedAISessionState } from './ai';
import { type ItemStatus } from './common';
import { type GitStatus } from './git';
import { type VerificationJob } from './runtime';

export type WorkspaceAction = 'layout.move' | 'repo.read' | 'git.status' | 'terminal.launch' | 'verification.run';
export type ActionCapabilityState = 'available' | 'unavailable' | 'unsupported' | 'forbidden' | 'conflicted';
export interface CapabilityRecoveryAction { action: string; label: string; }
export interface ActionCapability { action: WorkspaceAction; state: ActionCapabilityState; reasonCode?: string; message?: string; recoveryActions: CapabilityRecoveryAction[]; }
export interface WorkspaceProviderAxes { topology: string; contentProvider: string; executionProvider: string; datastore?: string; }

export type CanvasEntityKind = 'workspace' | 'plan' | 'session';
export interface CanvasEntityRef { kind: CanvasEntityKind; workspaceId: string; itemId?: string; itemPath?: string; identifier?: string; branchKey?: string; observedCommit?: string; sessionId?: string; }
export interface CanvasPosition { x: number; y: number; }
export interface CanvasSize { width: number; height: number; }
export interface CanvasViewport extends CanvasPosition { zoom: number; }
export interface CanvasLayout { id: string; ownerUserId?: string; workspaceId: string; branchKey: string; viewport: CanvasViewport; version: number; createdAt: string; updatedAt: string; }
export interface CanvasPlacementPatch { nodeId: string; entityRef: CanvasEntityRef; position: CanvasPosition; size?: CanvasSize; collapsed: boolean; expectedRevision: number; }
export interface SafeSessionPlanRef { itemId: string; itemPath: string; identifier?: string; branchKey: string; observedCommit?: string; }
export interface SafeSessionRecord { id: string; workspaceId: string; planRef?: SafeSessionPlanRef; provider: string; intent: string; requestedBranch: string; observedCommit?: string; state: EmbeddedAISessionState | 'interrupted'; startedAt: string; endedAt?: string; exitCode?: number; lastKnownAt: string; live?: boolean; }

export type SessionFileStatus = 'A' | 'M' | 'D' | 'R' | 'C' | '?';
export interface SessionFile { path: string; status: SessionFileStatus; oldPath?: string; }
export type SessionChangesAvailability = 'available' | 'unavailable' | 'unsupported';
export type SessionChangesReason = 'launch_commit_unknown' | 'launch_commit_missing' | 'workspace_missing' | 'content_provider_unsupported';
export interface SessionChanges { sessionId: string; launchCommit: string; headCommit: string; headMoved: number; files: SessionFile[]; additions: number; deletions: number; truncated: boolean; availability: SessionChangesAvailability; reason?: SessionChangesReason; /** A harness run's changes: where they were read, its own copy or its branch. */ from?: 'worktree' | 'branch'; branch?: string; }
export interface ClaudeTranscript { sessionId: string; branch?: string; startedAt: string; lastActiveAt: string; messages: number; itemIds: string[]; }
/** `tool` and `thinking` turns appear only when the reader asks for the model's work. */
export interface ClaudeTurn { role: 'user' | 'assistant' | 'tool' | 'thinking'; text: string; at?: string; }
export type SessionScope = 'active' | 'all';
export interface GlobalSessionRow { record: SafeSessionRecord; workspaceName: string; checkoutBranch: string; planTitle?: string; /** The plan's key (its identifier), which joins it to a harness task on the same ticket. */ planKey?: string; changesCount?: number; }
export interface GlobalSessionsResponse { rows: GlobalSessionRow[]; truncated: boolean; }
export interface CanvasWorkspaceNode { id: string; name?: string; branch?: string; commit?: string; git?: GitStatus; providerAxes: WorkspaceProviderAxes; actions: Partial<Record<WorkspaceAction, ActionCapability>>; verification?: VerificationJob; }
export interface CanvasPlanNode { itemId: string; identifier?: string; title?: string; service?: string; status: ItemStatus; branch?: string; commit?: string; editable: boolean; actions: Partial<Record<WorkspaceAction, ActionCapability>>; }
export interface CanvasSessionNode { record: SafeSessionRecord; }
/** An `ephemeral` node leaves the Canvas when its session stops, unless pinned. */
export interface CanvasNode { id: string; kind: CanvasEntityKind; state: 'resolved' | 'stale' | 'forbidden'; entityRef: CanvasEntityRef; position: CanvasPosition; size?: CanvasSize; collapsed: boolean; revision: number; workspace?: CanvasWorkspaceNode; plan?: CanvasPlanNode; session?: CanvasSessionNode; ephemeral?: boolean; }
export interface CanvasSection { id: string; title: string; position: CanvasPosition; width: number; height: number; nodeIds: string[]; }
export interface CanvasConnection { id: string; source: string; target: string; kind: 'repository_contains' | 'session_launched_from' | string; sourceOfTruth: 'derived'; }
export interface CanvasProjection { layout: CanvasLayout; nodes: CanvasNode[]; connections: CanvasConnection[]; unplaced: CanvasEntityRef[]; }
