import type { ItemStatus, JiraIssue } from '../../lib/types';

export type NewItemOrigin = 'blank' | 'jira';

export type NewWorkItemDraft = {
  origin: NewItemOrigin;
  source: string;
  scope: string;
  identifier: string;
  title: string;
  status: ItemStatus;
  owner: string;
  tags: string;
  jiraKey: string;
};

export const emptyNewWorkItemDraft = (): NewWorkItemDraft => ({
  origin: 'blank',
  source: '',
  scope: '',
  identifier: '',
  title: '',
  status: 'draft',
  owner: '',
  tags: '',
  jiraKey: ''
});


export function parseTags(value: string): string[] {
  return Array.from(new Set(value.split(',').map((tag) => tag.trim()).filter(Boolean)));
}

export function slugTag(value: string): string {
  return value.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
}

export function jiraTags(issue: JiraIssue): string[] {
  const tags = [...issue.labels];
  const issueType = slugTag(issue.issueType);
  const priority = slugTag(issue.priority ?? '');
  if (issueType) tags.unshift(issueType);
  if (priority) tags.unshift(`priority-${priority}`);
  return Array.from(new Set(tags.filter(Boolean)));
}

export function jiraReadme(issue: JiraIssue): string {
  const lines = [
    `# ${issue.key}: ${issue.summary}`,
    '',
    '## Jira Context',
    '',
    `- Key: ${issue.key}`,
    `- Status: ${issue.status || 'Unknown'}`,
    `- Type: ${issue.issueType || 'Unknown'}`,
    `- Priority: ${issue.priority || 'Unspecified'}`,
    `- Assignee: ${issue.assignee?.displayName || 'Unassigned'}`,
    `- Reporter: ${issue.reporter?.displayName || 'Unknown'}`,
    issue.browserUrl ? `- Jira: ${issue.browserUrl}` : '',
    '',
    '## Description',
    '',
    issue.description?.trim() || '_No Jira description available._'
  ].filter((line) => line !== '');
  if (issue.attachments.length > 0) {
    lines.push('', '## Attachments', '');
    for (const attachment of issue.attachments) {
      lines.push(`- ${attachment.filename} (${attachment.mediaType || 'unknown'}, ${attachment.sizeBytes.toLocaleString()} bytes)`);
    }
  }
  return `${lines.join('\n')}\n`;
}
