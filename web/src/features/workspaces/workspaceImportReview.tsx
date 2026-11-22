import type { JiraConnection, WorkspaceImportCandidate, WorkspaceImportPreview, WorkspaceImportResult } from '../../lib/types';
import { sourcePaths } from '../../lib/sources';

export type WorkspaceImportState = 'selecting' | 'previewing' | 'reviewing' | 'importing' | 'complete' | 'error';

export function defaultWorkspaceImportSelection(preview: WorkspaceImportPreview): string[] {
	return preview.candidates.filter((candidate) => candidate.status === 'valid' && candidate.selected).map((candidate) => candidate.candidateKey);
}

export function WorkspaceImportReview({ preview, selectedKeys, onToggle, onToggleAll }: {
	preview: WorkspaceImportPreview;
	selectedKeys: string[];
	onToggle: (candidateKey: string) => void;
	onToggleAll: () => void;
}) {
	const selectable = preview.candidates.filter((candidate) => candidate.status === 'valid');
	const allSelected = selectable.length > 0 && selectable.every((candidate) => selectedKeys.includes(candidate.candidateKey));
	return <section className="workspace-import-review" aria-label="Workspace import review">
		<div className="workspace-import-summary" aria-live="polite">
			<div><strong>{preview.summary.valid} workspace{preview.summary.valid === 1 ? '' : 's'} ready to import</strong><span>{selectedKeys.length} selected from {preview.candidates.length} candidates.</span></div>
			<button className="secondary" type="button" onClick={onToggleAll} disabled={selectable.length === 0}>{allSelected ? 'Clear selectable' : 'Select all valid'}</button>
		</div>
		<div className="workspace-import-candidates">
			{preview.candidates.map((candidate) => <WorkspaceImportCandidateCard key={candidate.candidateKey} candidate={candidate} selected={selectedKeys.includes(candidate.candidateKey)} onToggle={onToggle} />)}
		</div>
	</section>;
}

export function WorkspaceImportCandidateCard({ candidate, selected, onToggle }: { candidate: WorkspaceImportCandidate; selected: boolean; onToggle: (candidateKey: string) => void }) {
	const workspace = candidate.workspace;
	const selectable = candidate.status === 'valid';
	const issues = workspaceImportVisibleIssues(candidate);
	return <article className={`workspace-import-candidate status-${candidate.status}`}>
		<header>
			<label><input type="checkbox" checked={selected} disabled={!selectable} onChange={() => onToggle(candidate.candidateKey)} /><span><strong>{workspace.name || `Candidate ${candidate.position}`}</strong><small>{workspace.path || 'No workspace path'}</small></span></label>
			<span className="workspace-import-status">{workspaceImportStatusLabel(candidate.status)}</span>
		</header>
		<dl>
			<div className="workspace-import-detail-card"><dt>Base branch</dt><dd>{workspace.baselineBranch || 'Not set'}</dd></div>
			<div className="workspace-import-detail-card"><dt>Sources</dt><dd>{sourcePaths(workspace).join(', ') || 'None'}</dd></div>
			{workspace.remoteUrl && <div className="workspace-import-detail-card workspace-import-detail-card-wide"><dt>Original remote</dt><dd>{workspace.remoteUrl}</dd></div>}
			{workspace.jira && <WorkspaceImportJiraSummary jira={workspace.jira} />}
			{workspace.knowledge && <div className="workspace-import-detail-card workspace-import-detail-card-wide"><dt>Knowledge</dt><dd>{workspace.knowledge.enabled === false ? 'Disabled' : `Enabled${workspace.knowledge.enrichExecutable ? ` · ${workspace.knowledge.enrichExecutable}` : ''}${workspace.knowledge.enrichArgs?.length ? ` · arguments: ${workspace.knowledge.enrichArgs.join(', ')}` : ''}`}</dd></div>}
		</dl>
		{issues.length > 0 && <ul className="workspace-import-issues" aria-label={`${workspace.name || `Candidate ${candidate.position}`} issues`}>{issues.map((issue, index) => <li key={`${issue.code}-${index}`}><strong>{issue.field}</strong><span>{issue.message}</span></li>)}</ul>}
	</article>;
}

export function WorkspaceImportJiraSummary({ jira }: { jira: JiraConnection }) {
	return <div className="workspace-import-detail-card workspace-import-jira">
		<dt>Jira</dt>
		<dd>
			<span><strong>Deployment</strong><em>{jira.deploymentType}</em></span>
			<span><strong>Project</strong><em>{jira.projectKey || 'Not set'}</em></span>
			<span className="wide"><strong>URL</strong><em>{jira.baseUrl}</em></span>
			{jira.accountEmail && <span className="wide"><strong>Account</strong><em>{jira.accountEmail}</em></span>}
			<span className="wide"><strong>Token env</strong><em>{jira.tokenEnvVar}</em></span>
		</dd>
	</div>;
}

export function workspaceImportVisibleIssues(candidate: WorkspaceImportCandidate): WorkspaceImportCandidate['issues'] {
	if (candidate.status !== 'already_registered') return candidate.issues;
	return candidate.issues.filter((issue) => issue.field !== 'path');
}

export function WorkspaceImportResults({ results }: { results: WorkspaceImportResult[] }) {
	return <section className="workspace-import-results" aria-label="Workspace import results" aria-live="polite">
		<header><h3>Import complete</h3><span>{results.filter((result) => result.workspace).length} registered</span></header>
		{results.length === 0 && <div className="empty-inline">No candidates were imported.</div>}
		<ul>{results.map((result) => <li key={result.candidateKey} className={`status-${result.status}`}>
			<span><strong>{result.workspace?.name ?? 'Selected candidate'}</strong><small>{result.message || workspaceImportResultLabel(result.status)}</small></span>
			<span className="workspace-import-status">{workspaceImportResultLabel(result.status)}</span>
		</li>)}</ul>
	</section>;
}

export function workspaceImportStatusLabel(status: WorkspaceImportCandidate['status']): string {
	switch (status) {
	case 'already_registered': return 'Already registered';
	case 'duplicate': return 'Duplicate';
	case 'invalid': return 'Invalid';
	default: return 'Valid';
	}
}

export function workspaceImportResultLabel(status: WorkspaceImportResult['status']): string {
	switch (status) {
	case 'indexed': return 'Indexed';
	case 'scan_failed': return 'Scan failed';
	case 'skipped': return 'Skipped';
	default: return 'Failed';
	}
}
