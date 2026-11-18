import { useEffect, useMemo, useRef, useState } from 'react';
import type { HarnessAgentRole, HarnessAgentRun, HarnessRun } from '../../lib/types';
import { api } from '../../shared/api';
import { TranscriptTurns } from '../ai-session/TranscriptTurns';
import type { HarnessClient } from '../harness/harnessClient';
import { HxStatusBadge } from '../harness/HxStatusBadge';
import { PipelineStepper } from '../harness/PipelineStepper';
import { pipelineModel } from '../harness/pipeline';
import { useAgentTranscript } from '../harness/RunDrawer';
import { StopRunButton, canStopRun } from '../harness/StopRunButton';
import { agentRoleLabels, formatCost, formatMinutes, hxStatuses, hxSteps, plural } from '../harness/vocabulary';
import type { HxStep } from '../harness/vocabulary';
import '../harness/harness.css';
import { readableTurns } from './agentSessions';
import type { HarnessSessionRow } from './agentSessions';
import { SessionKindTag, focusAgent, harnessAgentLabel, harnessGlyph, harnessRunGlyph, startedAgents } from './SessionCard';
import { SessionChangesPanel } from './SessionChangesPanel';
import type { ChangesSource } from './SessionChangesPanel';
import { formatSessionDate } from './sessionDisplay';

/** The step of the run page's stepper each agent works in. */
const roleSteps: Record<HarnessAgentRole, HxStep> = { planner: 'plan', plan_reviewer: 'plan_check', coder: 'build', code_reviewer: 'code_check', qa_fixer: 'fix' };

/** What the person picked: a step, and the round in it. Unset, the detail follows the agent in play. */
type Pick = { step: HxStep; agentRunId?: string };

/**
 * A ticket's harness sessions in the Session Browser. The run's stepper picks
 * a step, its round chips pick one agent of that step, and the agent's
 * conversation shows below; so a ticket with many rounds stays one card in
 * the list. Agents run headless, so there is no terminal: steering happens on
 * the run page, through the run's questions and approvals.
 */
export function HarnessSessionDetail({ row, client, onOpenRun, onStopRun }: {
	row: HarnessSessionRow;
	client: HarnessClient;
	onOpenRun?: (runId: string) => void;
	onStopRun?: (run: HarnessRun) => Promise<void>;
}) {
	const { task } = row;
	const [runId, setRunId] = useState<string>();
	const [pick, setPick] = useState<Pick>();
	const [tab, setTab] = useState<'conversation' | 'changes'>('conversation');
	useEffect(() => { setRunId(undefined); setPick(undefined); setTab('conversation'); }, [row.key]);

	const detail = row.runs.find((candidate) => candidate.run.id === runId) ?? row.runs[0];
	const { run } = detail;
	const agents = useMemo(() => startedAgents(detail), [detail]);
	const model = useMemo(() => pipelineModel(detail), [detail]);
	const picked = pick?.agentRunId ? agents.find((agentRun) => agentRun.id === pick.agentRunId) : undefined;
	const agent: HarnessAgentRun | undefined = pick ? picked : focusAgent(detail);
	const step = pick?.step ?? (agent ? roleSteps[agent.role] : undefined);
	const rounds = step ? agents.filter((agentRun) => roleSteps[agentRun.role] === step) : [];
	const transcript = useAgentTranscript(client, agent);
	const readable = useMemo(() => readableTurns(transcript.turns), [transcript.turns]);
	const steps = useRef<HTMLElement>(null);
	// Keep the chosen step in sight where the stepper scrolls sideways.
	useEffect(() => {
		// Scroll the stepper alone: scrolling the step into view would move the panes around it too.
		const list = steps.current?.querySelector<HTMLElement>('.hx-stepper');
		const chosen = list?.querySelector<HTMLElement>('.hx-stage.selected');
		if (!list || !chosen) return;
		const box = list.getBoundingClientRect();
		const card = chosen.getBoundingClientRect();
		if (card.left < box.left) list.scrollLeft -= box.left - card.left + 8;
		else if (card.right > box.right) list.scrollLeft += card.right - box.right + 8;
	}, [step, run.id]);

	const branch = run.prUrl && run.remoteBranch ? run.remoteBranch : run.branch;
	const changes = useMemo<ChangesSource>(() => ({
		id: run.id,
		live: run.state === 'running',
		loadChanges: (id, signal) => api.harnessRunChanges(id, signal),
		loadDiff: (id, path, signal) => api.harnessRunFileDiff(id, path, signal),
		note: 'The whole run\'s changes, read from its own worktree while it runs and from its branch after. The harness keeps one worktree per run, not per round.'
	}), [run.id, run.state]);
	const sum = (key: 'durationMs' | 'costUsd') => detail.agentRuns.reduce((total, agentRun) => total + agentRun[key], 0);
	const runOrder = [...row.runs].sort((a, b) => Date.parse(a.run.startedAt) - Date.parse(b.run.startedAt));

	return <div className="session-browser-detail-content session-harness-detail">
		<header className="session-browser-detail-header">
			<div className="session-browser-detail-heading">
				<h3><SessionKindTag kind="harness" /><span>{task ? `${task.source.key} · ${task.title}` : 'Harness run'}</span></h3>
				<dl>
					<div><dt>Run</dt><dd>{row.runs.length > 1
						? <select className="session-harness-run-picker" aria-label="Run" value={run.id} onChange={(event) => { setRunId(event.target.value); setPick(undefined); }}>
							{row.runs.map((candidate) => <option key={candidate.run.id} value={candidate.run.id}>{`Run ${runOrder.indexOf(candidate) + 1} · ${formatSessionDate(candidate.run.startedAt)} · ${harnessRunGlyph(candidate.run).label}`}</option>)}
						</select>
						: <HxStatusBadge status={harnessRunGlyph(run).status} label={harnessRunGlyph(run).label} size="sm" />}</dd></div>
					<div><dt>Branch</dt><dd><code>{branch}</code></dd></div>
					<div><dt>Started</dt><dd>{formatSessionDate(run.startedAt)}</dd></div>
					<div><dt>Work</dt><dd>{[plural(agents.length, 'round'), formatMinutes(sum('durationMs')), formatCost(sum('costUsd'))].join(' · ')}</dd></div>
				</dl>
			</div>
			<div className="session-browser-detail-actions">
				{onOpenRun && <button type="button" className="secondary" onClick={() => onOpenRun(run.id)}>Open run page</button>}
				{onStopRun && task && canStopRun(run) && <StopRunButton task={task} run={run} onCancel={onStopRun} />}
			</div>
		</header>
		<section className="session-harness-steps" ref={steps} aria-label="Steps">
			<PipelineStepper
				model={model}
				label={`Steps of ${task?.source.key ?? 'this run'}`}
				selectedId={step}
				legend={false}
				onSelect={(card) => {
					const own = agents.filter((agentRun) => roleSteps[agentRun.role] === card.step);
					setPick({ step: card.step, agentRunId: own.at(-1)?.id });
					setTab('conversation');
				}}
			/>
			<p className="session-harness-hint">Click a step to read its agent&apos;s conversation. Questions and approvals are on the run page.</p>
		</section>
		<div className="session-harness-agent-bar">
			{agent
				? <p className="session-harness-agent">
					<strong>{`${agentRoleLabels[agent.role]} · round ${agent.round}`}</strong>
					<HxStatusBadge status={harnessGlyph(agent).status} label={harnessGlyph(agent).label} size="sm" className="session-harness-agent-state" />
					<span>{harnessAgentLabel(agent)}</span>
					<span>{[plural(agent.turns, 'turn'), formatMinutes(agent.durationMs), formatCost(agent.costUsd)].join(' · ')}</span>
				</p>
				: <p className="session-harness-agent muted">{step ? `${hxSteps[step].label} has no agent conversation in this run. Its record is on the run page.` : 'No agent has started in this run yet.'}</p>}
			{rounds.length > 1 && <div className="session-harness-rounds" role="group" aria-label={`Rounds of ${step ? hxSteps[step].label : 'this step'}`}>
				<span className="session-harness-rounds-label">Rounds</span>
				{rounds.map((agentRun, index) => {
					const glyph = harnessGlyph(agentRun);
					const on = agentRun.id === agent?.id;
					return <button key={agentRun.id} type="button" className={on ? 'session-harness-round active' : 'session-harness-round'} aria-pressed={on} title={`${agentRoleLabels[agentRun.role]} · round ${agentRun.round} · ${glyph.label}`} onClick={() => { setPick({ step: roleSteps[agentRun.role], agentRunId: agentRun.id }); setTab('conversation'); }}>
						<i className={`hx-swatch hx-tone-${hxStatuses[glyph.status].tone}`} aria-hidden="true" />{index + 1}
					</button>;
				})}
			</div>}
		</div>
		{agent && <>
			<div className="hx-tabs session-browser-detail-tabs" role="tablist" aria-label="Session detail tabs">
				<button type="button" role="tab" className={tab === 'conversation' ? 'on' : ''} aria-selected={tab === 'conversation'} onClick={() => setTab('conversation')}>Conversation</button>
				<button type="button" role="tab" className={tab === 'changes' ? 'on' : ''} aria-selected={tab === 'changes'} onClick={() => setTab('changes')}>Changes</button>
			</div>
			{tab === 'conversation' && <div className="session-harness-conversation">
				{transcript.error
					? <p className="error" role="alert">{transcript.error}</p>
					: <TranscriptTurns className="hx-transcript session-transcript" turns={readable} label={`${agentRoleLabels[agent.role]} conversation`} placeholder="Reading the conversation…" names={{ user: 'Harness', assistant: agentRoleLabels[agent.role] }} foldUser="The brief the harness gave it" />}
			</div>}
			{tab === 'changes' && <SessionChangesPanel source={changes} />}
		</>}
	</div>;
}
