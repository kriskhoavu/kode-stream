import type { E2ERunbookList, CreateVerificationJobInput, JiraIssueState, ItemVerificationTests, VerifyProfile, VerificationJob, RunArtifact, VerificationTestSelection } from '../../lib/types';
import { request } from './http';
import { normalizeItemVerificationTests } from './normalize';

export const verificationApi = {
  createVerificationJob: (workspaceId: string, input: CreateVerificationJobInput = {}, signal?: AbortSignal) =>
    request<VerificationJob>(`/api/workspaces/${encodeURIComponent(workspaceId)}/verification-jobs`, { method: 'POST', body: JSON.stringify(input), signal }, false),
  ingestVerificationCheckpoint: (workspaceId: string, input: { eventType: string; profile?: VerifyProfile; provider?: string; sessionId?: string; terminalMode?: string }) =>
    request<VerificationJob>(`/api/workspaces/${encodeURIComponent(workspaceId)}/verification-checkpoints`, { method: 'POST', body: JSON.stringify(input) }, false),
  verificationJob: (workspaceId: string, jobId: string, signal?: AbortSignal) =>
    request<VerificationJob>(`/api/workspaces/${encodeURIComponent(workspaceId)}/verification-jobs/${encodeURIComponent(jobId)}`, { signal }, false),
  verificationArtifacts: (workspaceId: string, jobId: string) =>
    request<RunArtifact[]>(`/api/workspaces/${encodeURIComponent(workspaceId)}/verification-jobs/${encodeURIComponent(jobId)}/artifacts`, undefined, false),
  rerunVerificationJob: (workspaceId: string, jobId: string, profile?: VerifyProfile, signal?: AbortSignal) =>
    request<VerificationJob>(`/api/workspaces/${encodeURIComponent(workspaceId)}/verification-jobs/${encodeURIComponent(jobId)}/rerun`, { method: 'POST', body: JSON.stringify({ profile }), signal }, false),
  itemVerificationTests: (itemId: string, signal?: AbortSignal) =>
    request<ItemVerificationTests>(`/api/items/${encodeURIComponent(itemId)}/verification-tests`, { signal }, !signal).then(normalizeItemVerificationTests),
  saveItemVerificationTests: (itemId: string, input: VerificationTestSelection) =>
    request<ItemVerificationTests>(`/api/items/${encodeURIComponent(itemId)}/verification-tests`, { method: 'PUT', body: JSON.stringify(input) }).then(normalizeItemVerificationTests),
	itemE2ERunbooks: (itemId: string, signal?: AbortSignal) => request<E2ERunbookList>(`/api/items/${encodeURIComponent(itemId)}/e2e-runbooks`, { signal }, !signal),
  jiraIssue: (itemId: string, signal?: AbortSignal) => request<JiraIssueState>(`/api/items/${encodeURIComponent(itemId)}/jira`, { signal }, !signal),
  refreshJiraIssue: (itemId: string, signal?: AbortSignal) => request<JiraIssueState>(`/api/items/${encodeURIComponent(itemId)}/jira/refresh`, { method: 'POST', signal }, false),
  jiraAttachmentURL: (itemId: string, attachmentId: string) => `/api/items/${encodeURIComponent(itemId)}/jira/attachments/${encodeURIComponent(attachmentId)}`
};
