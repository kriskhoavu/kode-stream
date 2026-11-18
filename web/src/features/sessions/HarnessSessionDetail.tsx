import { useEffect, useMemo, useState } from 'react';
import type { HarnessRun } from '../../lib/types';
import { api } from '../../shared/api';
import { TranscriptTurns } from '../ai-session/TranscriptTurns';
import type { HarnessClient } from '../harness/harnessClient';
import { useAgentTranscript } from '../harness/RunDrawer';
import { StopRunButton, canStopRun } from '../harness/StopRunButton';
import { agentRoleLabels, formatCost, formatMinutes, plural } from '../harness/vocabulary';
import '../harness/harness.css';
import type { HarnessSessionRow } from './agentSessions';
import { SessionKindTag, harnessAgentLabel, harnessTitle } from './SessionCard';
import { SessionChangesPanel } from './SessionChangesPanel';
import type { ChangesSource } from './SessionChangesPanel';
import { formatSessionDate } from './sessionDisplay';

/**
 * A harness session in the Session Browser: one agent the harness ran for a
 * task. It has no terminal, because the agent runs headless; the browser shows
 * its conversation instead, and its run's changes. Steering happens on the
 * run page, through the run's questions and approvals.
 */
export function HarnessSessionDetail({ row, client, onOpenRun, onStopRun }: {
	row: HarnessSessionRow;
	client: HarnessClient;
	onOpenRun?: (runId: string) => void;
	onStopRun?: (run: HarnessRun) => Promise<void>;
}) {
	const { agentRun, run, task } = row;
	const [tab, setTab] = useState<'conversation' | 'changes'>('conversation');
	useEffect(() => { setTab('conversation'); }, [agentRun.id]);
	const transcript = useAgentTranscript(client, agentRun);
	const title = harnessTitle(row);
	const branch = run.prUrl && run.remoteBranch ? run.remoteBranch : run.branch;
	const changes = useMemo<ChangesSource>(() => ({
		id: run.id,
		live: run.state === 'running',
		loadChanges: (id, signal) => api.harnessRunChanges(id, signal),
		loadDiff: (id, path, signal) => api.harnessRunFileDiff(id, path, signal),
		note: 'The whole run\'s changes, read from its own worktree while it runs and from its branch after.'
	}), [run.id, run.state]);

	return <div className="session-browser-detail-content">
		<header className="session-browser-detail-header">
			<div>
				<h3><SessionKindTag kind="harness" />{title}</h3>
				<dl>
					<dt>Task</dt><dd>{task ? `${task.source.key} · ${task.title}` : '—'}</dd>
					<dt>Step</dt><dd>{row.step}</dd>
					<dt>Branch</dt><dd>{branch}</dd>
					<dt>Agent</dt><dd>{harnessAgentLabel(agentRun)}</dd>
					<dt>Started</dt><dd>{formatSessionDate(agentRun.startedAt)}</dd>
					<dt>Work</dt><dd>{[plural(agentRun.turns, 'turn'), formatMinutes(agentRun.durationMs), formatCost(agentRun.costUsd)].join(' · ')}</dd>
				</dl>
			</div>
			<div className="session-browser-detail-actions">
				{onOpenRun && <button type="button" className="secondary" onClick={() => onOpenRun(run.id)}>Open run page</button>}
				{onStopRun && task && canStopRun(run) && <StopRunButton task={task} run={run} onCancel={onStopRun} />}
			</div>
		</header>
		<p className="session-harness-note">The harness drives this agent, so it has no terminal to type into. Answer its questions and approve its work on the run page.</p>
		<div className="side-panel-tabs session-browser-detail-tabs" role="tablist" aria-label="Session detail tabs">
			<button type="button" role="tab" className={tab === 'conversation' ? 'active' : ''} aria-selected={tab === 'conversation'} onClick={() => setTab('conversation')}>Conversation</button>
			<button type="button" role="tab" className={tab === 'changes' ? 'active' : ''} aria-selected={tab === 'changes'} onClick={() => setTab('changes')}>Changes</button>
		</div>
		{tab === 'conversation' && <div className="session-harness-conversation">
			{transcript.error
				? <p className="error" role="alert">{transcript.error}</p>
				: <TranscriptTurns className="hx-transcript" turns={transcript.turns} label={`${agentRoleLabels[agentRun.role]} conversation`} placeholder="Reading the conversation…" names={{ user: 'Harness', assistant: agentRoleLabels[agentRun.role] }} />}
		</div>}
		{tab === 'changes' && <SessionChangesPanel source={changes} />}
	</div>;
}
