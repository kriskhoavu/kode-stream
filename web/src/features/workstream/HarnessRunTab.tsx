import { useEffect, useState } from 'react';
import { FileText, Hand, Maximize2, MessageSquarePlus, PauseCircle } from 'lucide-react';
import type { HarnessApproval, HarnessApprovalGate } from '../../lib/types';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { FindingsPanel } from '../harness/FindingsPanel';
import { GateCard, gateWords } from '../harness/GateCard';
import { HxStatusBadge } from '../harness/HxStatusBadge';
import { PipelineStepper } from '../harness/PipelineStepper';
import { RunLedger } from '../harness/RunLedger';
import { WorkerChip } from '../harness/WorkerChip';
import { pipelineModel, runStatus } from '../harness/pipeline';
import { useNow } from '../harness/useNow';
import type { HarnessController } from '../harness/useHarness';
import { agentRoleLabels, agentRoleToRole, hxStatuses, roleLabels, stageKindLabels } from '../harness/vocabulary';
import type { CardWork } from './harnessJoin';

/** The gates a note can reach an agent through, and the decision that carries it. */
const noteGates: Partial<Record<HarnessApprovalGate, 'approve' | 'reject'>> = { resume: 'approve', plan: 'reject', question: 'approve' };

/**
 * The card's Agents tab in its "Harness run" mode: the compact stepper, who
 * is working, the Checker's findings, the ledger and the steering actions.
 * The hand-driven sessions are the tab's other mode.
 */
export function HarnessRunTab({ work, harness, onOpenConversation, onOpenPlan, onOpenFull, onTakeOver }: {
	work: CardWork;
	harness: HarnessController;
	/** Opens one agent's conversation in the card's main pane. */
	onOpenConversation: (agentRunId: string) => void;
	/** Opens the plan the run wrote in the card's main pane, before it is merged. */
	onOpenPlan?: (runId: string) => void;
	/** Shows the run whole, full width. */
	onOpenFull?: () => void;
	/** Hands the card to a person: the working role pauses and the Sessions mode opens. */
	onTakeOver: () => void;
}) {
	const { state } = harness;
	const now = useNow(30_000);
	const run = work.run;
	const runId = run?.id;
	const detail = runId ? state.details[runId] : undefined;
	const version = run ? `${run.revision}:${run.state}:${run.currentStageId}:${run.totals.costUsd}` : '';
	useEffect(() => {
		if (runId) void harness.loadRun(runId);
	}, [harness.loadRun, runId, version]);

	const [confirmPause, setConfirmPause] = useState(false);
	const [noteOpen, setNoteOpen] = useState(false);
	const [note, setNote] = useState('');
	if (!run) return null;

	const working = detail?.agentRuns.find((agentRun) => agentRun.state === 'running') ?? run.activeAgentRun;
	const workingRole = working ? agentRoleToRole[working.role] : undefined;
	const paused = workingRole ? state.status?.roles.find((role) => role.role === workingRole)?.paused : false;
	const waiting = work.waiting.map((approval) => state.approvals[approval.id] ?? approval);
	const noteGate = waiting.find((approval) => noteGates[approval.gate]);
	const status = runStatus(run);
	const stage = run.currentStage;
	const where = stage ? `${stageKindLabels[stage.kind]}${stage.maxRounds > 1 && status === 'working' ? ` · round ${stage.round} of ${stage.maxRounds}` : ''}` : '';
	const sendNote = (approval: HarnessApproval) => {
		const text = note.trim();
		if (!text) return;
		if (noteGates[approval.gate] === 'reject') harness.reject(approval, text);
		else harness.approve(approval, text);
		setNote('');
		setNoteOpen(false);
	};

	return (
		<section className="harness-run-tab hx" aria-label="Harness run">
			<div className="harness-run-head">
				<HxStatusBadge status={status} size="sm" label={where && (status === 'working' || status === 'waiting') ? `${hxStatuses[status].label} · ${where}` : undefined} />
				{working ? <WorkerChip engine={working.engine} model={working.model} role={agentRoleLabels[working.role]} /> : <WorkerChip role={stage?.kind === 'qa' ? 'Tester' : 'Harness'} />}
				{onOpenPlan && <button type="button" className="hx-link harness-run-plan" onClick={() => onOpenPlan(run.id)}><FileText size={13} aria-hidden="true" /> Read the plan</button>}
				{onOpenFull && <button type="button" className="hx-link harness-run-full" onClick={onOpenFull}><Maximize2 size={13} aria-hidden="true" /> Open full width</button>}
			</div>

			<div className="harness-run-actions">
				<button type="button" className="secondary hx-small-button" disabled={!workingRole || paused} title={workingRole ? (paused ? `The ${roleLabels[workingRole]} is already paused.` : `The ${roleLabels[workingRole]} finishes this round and takes nothing more until you resume it in Settings.`) : 'Nothing is working right now.'} onClick={() => setConfirmPause(true)}><PauseCircle size={14} aria-hidden="true" />{paused && workingRole ? `${roleLabels[workingRole]} paused` : 'Pause after this round'}</button>
				<button type="button" className="secondary hx-small-button" onClick={onTakeOver} title="The working role pauses and you continue from a session of your own."><Hand size={14} aria-hidden="true" />Take over by hand</button>
				<button type="button" className="secondary hx-small-button" disabled={!noteGate} aria-expanded={noteOpen} title={noteGate ? undefined : 'A note reaches the agent with your next decision, when the run waits on you.'} onClick={() => setNoteOpen((open) => !open)}><MessageSquarePlus size={14} aria-hidden="true" />Add a note</button>
			</div>
			{noteOpen && noteGate && (
				<form className="harness-run-note" onSubmit={(event) => { event.preventDefault(); sendNote(noteGate); }}>
					<label>
						<span>{noteGate.gate === 'plan' ? gateWords.plan.askLabel : (gateWords[noteGate.gate].answerLabel ?? 'Your note')}</span>
						<textarea rows={3} value={note} autoFocus onChange={(event) => setNote(event.target.value)} placeholder={noteGate.gate === 'plan' ? gateWords.plan.askPlaceholder : gateWords[noteGate.gate].answerPlaceholder} />
					</label>
					<div className="harness-run-note-actions">
						<button type="submit" className="primary hx-small-button" disabled={!note.trim()}>{noteGate.gate === 'plan' ? 'Send to Planner' : (gateWords[noteGate.gate].answerSubmit ?? 'Send')}</button>
						<button type="button" className="ghost hx-small-button" onClick={() => { setNoteOpen(false); setNote(''); }}>Back</button>
					</div>
				</form>
			)}

			{confirmPause && workingRole && (
				<ConfirmDialog
					title={`Pause the ${roleLabels[workingRole]} after this round?`}
					message={`The ${roleLabels[workingRole]} finishes the round it is on and starts nothing more, on this card or any other, until you resume it under Settings. The run keeps its place.`}
					confirmLabel="Pause after this round"
					onCancel={() => setConfirmPause(false)}
					onConfirm={() => { setConfirmPause(false); void harness.pauseRole(workingRole); }}
				/>
			)}

			{waiting.map((approval) => (
				<GateCard key={approval.id} approval={approval} task={work.task} sent={state.decisions[approval.id]} onApprove={(reason) => harness.approve(approval, reason)} onReject={(reason) => harness.reject(approval, reason)} />
			))}

			{detail ? (
				<>
					<div className="hx-card hx-stepper-card"><PipelineStepper model={pipelineModel(detail, now)} label={`Progress for ${work.task.source.key}`} orientation="vertical" /></div>
					<FindingsPanel agentRuns={detail.agentRuns} />
					<RunLedger detail={detail} onOpenAgentRun={onOpenConversation} />
				</>
			) : <p className="hx-muted" aria-busy="true">Loading progress for this run…</p>}
		</section>
	);
}
