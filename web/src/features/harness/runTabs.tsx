import { useEffect, useState } from 'react';
import { ExternalLink } from 'lucide-react';
import type { HarnessAgentRun, HarnessTask, SessionChanges } from '../../lib/types';
import { SessionChangesPanel } from '../sessions/SessionChangesPanel';
import { JiraItemPanel } from '../jira/JiraItemPanel';
import type { HarnessClient } from './harnessClient';
import { FindingsList } from './FindingsPanel';
import { hxSteps, plural, sourceLabels } from './vocabulary';
import { useAgentSession } from './runAgents';

export function TicketTab({ task, fixture }: { task: HarnessTask; fixture: boolean }) {
	if (task.source.kind === 'jira' && !fixture) return <div className="hx-ticket"><JiraItemPanel workspaceId={task.workspaceId} issueKey={task.source.key} /></div>;
	return (
		<div className="hx-ticket hx-ticket-summary">
			<dl>
				{task.source.status && <><dt>Status</dt><dd>{task.source.status}</dd></>}
				{task.source.priority && <><dt>Priority</dt><dd>{task.source.priority}</dd></>}
				{task.source.assignee && <><dt>Assignee</dt><dd>{task.source.assignee}</dd></>}
				{task.source.labels.length > 0 && <><dt>Labels</dt><dd>{task.source.labels.join(', ')}</dd></>}
			</dl>
			{task.source.url && <a href={task.source.url} target="_blank" rel="noreferrer"><ExternalLink size={14} aria-hidden="true" />Open {task.source.key} in {sourceLabels[task.source.kind]}</a>}
			{fixture && <p className="hx-muted">The sample data has no tracker behind it; a real task shows the whole ticket here.</p>}
		</div>
	);
}

export function NoRun({ run }: { run: boolean }) {
	return <p className="hx-muted">{run ? 'Loading…' : 'This task has not started, so there is nothing here yet.'}</p>;
}

export function latestAgentRun(agentRuns: HarnessAgentRun[] = []): HarnessAgentRun | undefined {
	return [...agentRuns].sort((a, b) => Date.parse(a.startedAt ?? '') - Date.parse(b.startedAt ?? '')).at(-1);
}

export function FindingsByRound({ agentRuns }: { agentRuns: HarnessAgentRun[] }) {
	const reviews = agentRuns
		.filter((agentRun) => (agentRun.role === 'plan_reviewer' || agentRun.role === 'code_reviewer') && agentRun.state === 'succeeded')
		.sort((a, b) => Date.parse(b.startedAt ?? '') - Date.parse(a.startedAt ?? ''));
	if (reviews.length === 0) return <p className="hx-muted">The Checker has not reviewed anything in this run yet.</p>;
	return (
		<div className="hx-findings-rounds">
			{reviews.map((review) => (
				<section key={review.id} aria-label={`${hxSteps[review.role === 'plan_reviewer' ? 'plan_check' : 'code_check'].label}, round ${review.round}`}>
					<h3 className="hx-section-title">{hxSteps[review.role === 'plan_reviewer' ? 'plan_check' : 'code_check'].label} · round {review.round} <small>· {review.verdict === 'approve' ? 'approved' : 'changes asked'}</small></h3>
					<FindingsList findings={review.findings ?? []} />
				</section>
			))}
		</div>
	);
}

export function ChangesTab({ client, runId, agentRun }: { client: HarnessClient; runId: string; agentRun?: HarnessAgentRun }) {
	const session = useAgentSession(client, agentRun?.id);
	const [changes, setChanges] = useState<SessionChanges | undefined>();
	const [error, setError] = useState('');
	useEffect(() => {
		if (session.record) return;
		const controller = new AbortController();
		client.runChanges(runId, controller.signal).then(setChanges).catch(() => { if (!controller.signal.aborted) setError('The changed files could not be loaded.'); });
		return () => controller.abort();
	}, [client, runId, session.record]);
	if (session.record) return <SessionChangesPanel session={session.record} />;
	if (error) return <p className="hx-banner" role="alert">{error}</p>;
	if (!changes) return <p className="hx-muted">Loading the changed files…</p>;
	if (changes.files.length === 0) return <p className="hx-muted">No files have changed yet.</p>;
	const words: Record<string, string> = { A: 'Added', M: 'Changed', D: 'Deleted', R: 'Renamed', C: 'Copied', '?': 'New' };
	return (
		<div className="hx-changes">
			<p className="hx-muted hx-num">{plural(changes.files.length, 'file')} · +{changes.additions} −{changes.deletions}{changes.truncated ? ' · list cut short' : ''}</p>
			<ul>{changes.files.map((file) => <li key={file.path}><span className="hx-chip">{words[file.status] ?? file.status}</span><code>{file.path}</code></li>)}</ul>
		</div>
	);
}
