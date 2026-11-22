import { lazy, Suspense, useEffect, useMemo, useState } from 'react';
import { ChevronRight } from 'lucide-react';
import type { HarnessAgentRun, HarnessStage } from '../../lib/types';
import { TranscriptTurns } from '../ai-session/TranscriptTurns';
import type { HarnessClient } from './harnessClient';
import { HxStatusBadge } from './HxStatusBadge';
import { agentRoleLabels, formatCost, hxStatuses, modelLabel, plural } from './vocabulary';
import { type LiveStep, liveSteps, agentStatus, useAgentSession, useAgentTranscript } from './runAgents';

// The live terminal pulls in xterm; load it only when someone opens the Live tab.
const SessionTerminal = lazy(() => import('../ai-session/SessionTerminal').then((module) => ({ default: module.SessionTerminal })));

/**
 * What each agent said, in the order it happened: every conversation of the
 * run as one numbered list, grouped by the stage that ran it, oldest first,
 * with the working agent chosen by default. A running agent's transcript is
 * read again every few seconds; its live terminal is shown instead when the
 * backend exposes the run as a session.
 */
export function LiveTab({ client, focus, stages, agentRuns, onReload }: { client: HarnessClient; focus?: string; stages: HarnessStage[]; agentRuns: HarnessAgentRun[]; onReload: () => Promise<unknown> }) {
	const steps = useMemo(() => liveSteps(stages, agentRuns), [stages, agentRuns]);
	const rounds = useMemo(() => steps.flatMap((step) => step.agentRuns), [steps]);
	const working = rounds.find((agentRun) => agentRun.state === 'running');
	const [chosen, setChosen] = useState<string | undefined>(focus);
	useEffect(() => { if (focus) setChosen(focus); }, [focus]);
	const selected = rounds.find((agentRun) => agentRun.id === chosen) ?? working ?? rounds.at(-1);
	// Which stages the person folded or unfolded by hand; the rest fold unless they hold the chosen or the working agent.
	const [folded, setFolded] = useState<Record<string, boolean>>({});
	const stageOpen = (step: LiveStep) => folded[step.id] ?? step.agentRuns.some((agentRun) => agentRun.id === selected?.id || agentRun.state === 'running');
	const session = useAgentSession(client, selected?.id);
	const transcript = useAgentTranscript(client, selected);
	if (!selected) return <p className="hx-muted">No agent has worked on this task yet.</p>;
	let number = 0;
	return (
		<div className="hx-live">
			{rounds.length > 1 && (
				<ol className="hx-live-steps" role="tablist" aria-orientation="vertical" aria-label="Agent rounds">
					{steps.map((step) => {
						const open = stageOpen(step);
						const first = number + 1;
						if (!open) number += step.agentRuns.length;
						return (
						<li key={step.id} className="hx-live-stage">
							<button type="button" className="hx-live-stage-title" aria-expanded={open} onClick={() => setFolded((prev) => ({ ...prev, [step.id]: !open }))}>
								<ChevronRight size={14} aria-hidden="true" />{step.title}
								<span className="hx-live-stage-count hx-num">{open ? plural(step.agentRuns.length, 'step') : first === number ? `step ${first}` : `steps ${first}–${number}`}</span>
							</button>
							{open && <ol>
								{step.agentRuns.map((agentRun) => {
									number += 1;
									const on = agentRun.id === selected.id;
									return (
										<li key={agentRun.id}>
											<button type="button" role="tab" aria-selected={on} className={on ? 'on' : undefined} onClick={() => setChosen(agentRun.id)}>
												<span className="hx-live-step-no hx-num">{number}</span>
												<HxStatusBadge status={agentStatus(agentRun)} size="sm" label={`${agentRoleLabels[agentRun.role]} · round ${agentRun.round}${(agentRun.attempt ?? 1) > 1 ? ` · try ${agentRun.attempt}` : ''}`} />
												<span className="hx-live-step-meta hx-num">{agentRun.state === 'running' ? 'working' : `${plural(agentRun.turns, 'turn')} · ${formatCost(agentRun.costUsd)}`}</span>
											</button>
										</li>
									);
								})}
							</ol>}
						</li>
						);
					})}
				</ol>
			)}
			<p><b>{agentRoleLabels[selected.role]}</b> · {modelLabel(selected.model)} · <span className="hx-num">{plural(selected.turns, 'turn')} · {formatCost(selected.costUsd)}</span>{selected.state !== 'running' && <span className="hx-muted"> · {hxStatuses[agentStatus(selected)].label.toLowerCase()}</span>}</p>
			{selected.state === 'running' && <p className="hx-live-line">{selected.progress ?? 'Starting…'}</p>}
			{session.record?.live
				? (
					<Suspense fallback={<p className="hx-muted">Opening the live view…</p>}>
						<SessionTerminal record={session.record} visible onReload={onReload} classPrefix="hx-terminal" ariaLabel={`${agentRoleLabels[selected.role]} live terminal`} />
					</Suspense>
				)
				: transcript.error
					? <p className="hx-banner" role="alert">{transcript.error}</p>
					: transcript.turns && transcript.turns.length === 0
						? <p className="hx-muted">{selected.state === 'running' ? 'The conversation has not started yet.' : 'This agent left no conversation the harness can read.'}</p>
						: <TranscriptTurns className="hx-transcript" turns={transcript.turns} label={`${agentRoleLabels[selected.role]} conversation`} placeholder="Reading the conversation…" names={{ user: 'Harness', assistant: agentRoleLabels[selected.role] }} />}
			<p className="hx-muted">What the harness asked, what the {agentRoleLabels[selected.role]} thought, each tool it called, and what it answered. Tool output stays out; the coding tool keeps the full conversation.</p>
		</div>
	);
}
