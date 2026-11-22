export interface JiraConnection {
  deploymentType: 'cloud' | 'server';
  baseUrl: string;
  projectKey: string;
  accountEmail?: string;
  tokenEnvVar: string;
}

export interface JiraConnectionTest {
  ok: boolean;
  deploymentType: 'cloud' | 'server';
  projectKey: string;
  message: string;
  recoveryHint?: string;
}

export type JiraIssueStateName = 'not_configured' | 'invalid_identifier' | 'project_mismatch' | 'not_found' | 'available' | 'authentication_failed' | 'forbidden' | 'unavailable';
export interface JiraPerson { displayName: string; accountId?: string; email?: string; }
export interface JiraAttachment { id: string; filename: string; mediaType: string; sizeBytes: number; createdAt?: string; author: JiraPerson; }
export interface JiraIssue { key: string; summary: string; status: string; statusCategory?: string; description: string; issueType: string; assignee?: JiraPerson; reporter?: JiraPerson; priority?: string; labels: string[]; createdAt?: string; updatedAt?: string; browserUrl: string; attachments: JiraAttachment[]; truncated?: boolean; warnings?: string[]; }
export interface JiraIssueState { state: JiraIssueStateName; issue?: JiraIssue; message?: string; recoveryHint?: string; refreshedAt?: string; }
