import { type WorkspaceSourceLike } from './common';
import { type ScanResult } from './item';
import { type JiraConnection } from './jira';
import { type KnowledgeSettings } from './knowledge';
import { type WorkspaceRuntimeConfig } from './runtime';

export interface WorkspaceConfig {
  id: string;
  name: string;
  path: string;
  location?: WorkspaceLocation;
  accessMode?: WorkspaceAccessMode;
  ownerUserId?: string;
  agentId?: string;
  provider?: string;
  providerInstanceId?: string;
  providerRepository?: string;
  selectedRef?: string;
  resolvedCommitSha?: string;
  localRootLabel?: string;
  publishedSummary?: boolean;
  scanStatus?: string;
  baselineBranch: string;
  registrationMode?: WorkspaceRegistrationMode;
  remoteUrl?: string;
  clonePathManaged?: boolean;
  managedCloneRoot?: string;
  managedCloneId?: string;
  managedCloneVerified?: boolean;
  lastSelectedBranch?: string;
  sources: WorkspaceSourceLike[];
  createdAt: string;
  lastScannedAt?: string;
  jira?: JiraConnection;
  knowledge?: KnowledgeSettings;
  runtime?: WorkspaceRuntimeConfig;
}

export interface WorkspaceInput {
  name: string;
  path?: string;
  baselineBranch: string;
  sources: WorkspaceSourceLike[];
  registrationMode?: WorkspaceRegistrationMode;
  remoteUrl?: string;
  cloneRoot?: string;
  jira?: JiraConnection;
  knowledge?: KnowledgeSettings;
  runtime?: WorkspaceRuntimeConfig;
}

export type WorkspaceRegistrationMode = 'local_path' | 'remote_clone' | 'existing_workspace';
export type WorkspaceLocation = 'local_path' | 'cloud_agent' | 'cloud_remote_snapshot';
export type WorkspaceAccessMode = 'agent_backed' | 'remote_snapshot';
export type RuntimeMode = 'local' | 'cloud';
export type CloudRole = 'admin' | 'editor' | 'viewer';
export type Capability = 'read' | 'write' | 'workspace_registration' | 'git' | 'system' | 'terminal' | 'ai' | 'runtime' | 'verification';

export interface CloudUser {
  id: string;
  email?: string;
  name?: string;
  role: CloudRole;
  subject?: string;
}

export interface AgentConnection {
  available: boolean;
  status: string;
}

export interface CloudAgent {
  id: string;
  userId: string;
  name: string;
  platform?: string;
  status: string;
  lastSeenAt?: string;
}

export interface AgentConnectToken {
  token: string;
  expiresAt: string;
  deepLink: string;
}

export interface RuntimeContext {
  mode: RuntimeMode;
  user?: CloudUser;
  role?: CloudRole;
  capabilities: Record<Capability, boolean>;
  agent: AgentConnection;
}

export interface WorkspaceImportIssue {
  field: string;
  code: string;
  message: string;
}

export type WorkspaceImportCandidateStatus = 'valid' | 'invalid' | 'duplicate' | 'already_registered';
export type WorkspaceImportResultStatus = 'indexed' | 'scan_failed' | 'skipped' | 'failed';

export interface WorkspaceImportCandidate {
  candidateKey: string;
  position: number;
  workspace: WorkspaceInput;
  status: WorkspaceImportCandidateStatus;
  issues: WorkspaceImportIssue[];
  selected: boolean;
}

export interface WorkspaceImportSummary {
  valid: number;
  invalid: number;
  duplicate: number;
  alreadyRegistered: number;
}

export interface WorkspaceImportPreview {
  sourcePath: string;
  destinationPath: string;
  sourceFingerprint: string;
  candidates: WorkspaceImportCandidate[];
  summary: WorkspaceImportSummary;
}

export interface WorkspaceImportRequest {
  sourcePath: string;
  sourceFingerprint: string;
  candidateKeys: string[];
}

export interface WorkspaceImportResult {
  candidateKey: string;
  workspace?: WorkspaceConfig;
  status: WorkspaceImportResultStatus;
  scan?: ScanResult;
  message?: string;
}

export interface WorkspaceCreateResult {
  workspace: WorkspaceConfig;
  operationLog?: string;
}
