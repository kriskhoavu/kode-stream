import { Fragment } from 'react';
import { Code2, Eye, GitCompare, X } from 'lucide-react';
import type { HarnessAgentRun, HarnessRunDetail, HarnessRunFile, HarnessRunFileDiff, HarnessRunPlan, SessionChanges, SessionFile } from '../../../lib/types';
import { parseGitDiff } from '../../../shared/domain/diff';
import { MarkdownPreview } from '../../content-viewer/renderers/MarkdownPreview';
import { TranscriptTurns } from '../../ai-session/TranscriptTurns';
import type { HarnessClient } from '../harnessClient';
import { useAgentTranscript } from '../runAgents';
import type { LiveStep } from '../runAgents';
import { planSourceLine } from '../RunPlanPanel';
import { agentRoleLabels, formatCost, formatMinutes, hxStatuses, plural } from '../vocabulary';
import { type View, type WorkbenchKey, type Loaded } from './workbenchTypes';
import { useFileDiff } from './useRunWorkbenchData';
import { Delta, agentDot, agentTitle } from './WorkbenchTrees';

export function keyTitle(key: WorkbenchKey, detail: HarnessRunDetail, steps: LiveStep[]): string {
	if (key.startsWith('plan:') || key.startsWith('file:')) return key.slice(5).split('/').pop() ?? key;
	if (key === 'stage:*') return 'All steps';
	if (key.startsWith('stage:')) return `${steps.find((step) => step.id === key.slice(6))?.title ?? 'Step'} · all`;
	const agentRun = detail.agentRuns.find((candidate) => candidate.id === key.slice(6));
	return agentRun ? `${agentRoleLabels[agentRun.role]} · round ${agentRun.round}` : 'Conversation';
}

export function Reader({ client, detail, keyed, open, steps, planFiles, codeFiles, plan, changes, view, onView, onSelect, onClose }: {
	client: HarnessClient; detail: HarnessRunDetail; keyed?: WorkbenchKey; open: WorkbenchKey[]; steps: LiveStep[];
	planFiles: HarnessRunFile[]; codeFiles: SessionFile[]; plan?: HarnessRunPlan; changes?: SessionChanges;
	view?: View; onView: (view: View) => void; onSelect: (key: WorkbenchKey) => void; onClose: (key: WorkbenchKey) => void;
}) {
	const { run } = detail;
	const path = keyed && (keyed.startsWith('plan:') || keyed.startsWith('file:')) ? keyed.slice(5) : undefined;
	const isPlan = keyed?.startsWith('plan:');
	const modes: View[] = isPlan ? ['preview', 'source', 'diff'] : ['diff'];
	const current = path ? (view && modes.includes(view) ? view : modes[0]) : undefined;
	const diff = useFileDiff(client, run.id, current === 'diff' ? path : undefined, `${run.revision}:${run.state}`);
	const planFile = isPlan ? planFiles.find((file) => file.path === path) : undefined;
	const codeFile = keyed?.startsWith('file:') ? (codeFiles as (SessionFile & { additions?: number; deletions?: number })[]).find((file) => file.path === path) : undefined;
	const from = isPlan ? plan?.from : changes?.from;
	return (
		<>
			<div className="explorer-breadcrumbs">
				{path ? crumbs(path).map((part, index) => <Fragment key={index}>{index > 0 && <span className="hx-workbench-crumb-sep" aria-hidden="true">/</span>}<span title={part === '…' ? path : undefined}>{part}</span></Fragment>) : <span>{keyed ? keyTitle(keyed, detail, steps) : task(detail)}</span>}
			</div>
			<div className="editor-file-strip">
				<div className="tab-list file-tab-list" aria-label="Open items">
					{open.map((key) => (
						<div key={key} className={key === keyed ? 'file-tab active' : 'file-tab'}>
							<button type="button" className="file-tab-button" title={key.slice(key.indexOf(':') + 1)} onClick={() => onSelect(key)}>{keyTitle(key, detail, steps)}</button>
							<button type="button" className="file-tab-close" aria-label={`Close ${keyTitle(key, detail, steps)}`} onClick={() => onClose(key)}><X size={13} /></button>
						</div>
					))}
				</div>
			</div>
			{path && (
				<div className="tabs explorer-editor-tabs">
					<div className="editor-toolbar-actions">
						{modes.length > 1 ? <div className="editor-view-switch" role="tablist" aria-label="File view mode">
							{modes.map((mode) => {
								const Icon = mode === 'preview' ? Eye : mode === 'source' ? Code2 : GitCompare;
								return <button key={mode} type="button" role="tab" aria-selected={current === mode} className={current === mode ? 'active' : ''} onClick={() => onView(mode)}><Icon size={14} aria-hidden="true" /> {mode[0].toUpperCase() + mode.slice(1)}</button>;
							})}
						</div> : <span className="hx-workbench-viewname"><GitCompare size={14} aria-hidden="true" /> Diff{codeFile && <Delta additions={codeFile.additions ?? 0} deletions={codeFile.deletions ?? 0} />}</span>}
						<span className="autosave-state idle">{from === 'branch' ? `from ${run.remoteBranch || run.branch}` : from === 'worktree' ? 'live, not committed yet' : ''}</span>
					</div>
				</div>
			)}
			<section className="content-viewer hx-workbench-reader" data-file-kind={current === 'preview' ? 'markdown' : 'text'}>
				{!keyed && <p className="hx-workbench-empty">Pick a plan file, a changed file or a conversation on the left.</p>}
				{planFile && current === 'preview' && (planFile.truncated ? <p className="hx-workbench-empty">This file is too large to show here.</p> : /\.md$/i.test(planFile.path) ?<MarkdownPreview content={planFile.content} /> : <pre className="hx-workbench-source">{planFile.content}</pre>)}
				{planFile && current === 'source' && <pre className="hx-workbench-source">{planFile.content}</pre>}
				{path && current === 'diff' && <DiffView diff={diff} />}
				{keyed?.startsWith('agent:') && <Conversation client={client} agentRun={detail.agentRuns.find((agentRun) => agentRun.id === keyed.slice(6))} />}
				{keyed?.startsWith('stage:') && <Section client={client} steps={keyed === 'stage:*' ? steps : steps.filter((step) => step.id === keyed.slice(6))} titled={keyed === 'stage:*'} onOpen={onSelect} />}
				{isPlan && !planFile && plan && <p className="hx-workbench-empty">{planSourceLine(plan) || 'This plan file is no longer there.'}</p>}
			</section>
		</>
	);
}

export function crumbs(path: string): string[] {
	const parts = path.split('/');
	return parts.length <= 5 ? parts : [parts[0], '…', ...parts.slice(-3)];
}

export function task(detail: HarnessRunDetail): string {
	return `${detail.task.source.key} · ${detail.task.title}`;
}

export function DiffView({ diff }: { diff: Loaded<HarnessRunFileDiff> }) {
	if (diff.error) return <p className="hx-banner" role="alert">{diff.error}</p>;
	if (!diff.value) return <p className="hx-workbench-empty" aria-busy="true">Reading the diff…</p>;
	const files = parseGitDiff(diff.value.diff);
	if (files.length === 0) return <p className="hx-workbench-empty">{diff.value.from === '' ? 'This run\'s copy and branch are gone.' : 'No difference to show.'}</p>;
	return (
		<div className="diff-review hx-workbench-diff">
			{files.map((file) => (
				<article className="diff-file" key={file.path}>
					<div className="diff-lines">
						{file.lines.filter((line) => line.type !== 'meta' || line.text.startsWith('@@')).map((line, index) => (
							<div className={`diff-line ${line.type}`} key={index}>
								<span className="line-number">{line.oldLine ?? ''}</span>
								<span className="line-number">{line.newLine ?? ''}</span>
								<code>{line.text || ' '}</code>
							</div>
						))}
					</div>
				</article>
			))}
			{diff.value.truncated && <p className="hx-workbench-empty">The diff is longer than this view shows.</p>}
		</div>
	);
}

export function Conversation({ client, agentRun }: { client: HarnessClient; agentRun?: HarnessAgentRun }) {
	const transcript = useAgentTranscript(client, agentRun);
	if (!agentRun) return <p className="hx-workbench-empty">This conversation is no longer in the run.</p>;
	return (
		<div className="hx-workbench-thread">
			<p className="hx-workbench-thread-meta">{threadMeta(agentRun)}</p>
			{transcript.error ? <p className="hx-banner" role="alert">{transcript.error}</p> : <TranscriptTurns className="hx-transcript" turns={transcript.turns} label={`${agentRoleLabels[agentRun.role]} conversation`} placeholder="Reading the conversation…" names={{ user: 'Harness', assistant: agentRoleLabels[agentRun.role] }} />}
		</div>
	);
}

export function threadMeta(agentRun: HarnessAgentRun): string {
	return [plural(agentRun.turns, 'turn'), formatMinutes(agentRun.durationMs), formatCost(agentRun.costUsd)].join(' · ');
}

export function Section({ client, steps, titled, onOpen }: { client: HarnessClient; steps: LiveStep[]; titled: boolean; onOpen: (key: WorkbenchKey) => void }) {
	return (
		<div className="hx-workbench-section">
			{steps.map((step) => (
				<div key={step.id}>
					{titled && <h2 className="hx-workbench-section-title">{step.title}</h2>}
					{step.agentRuns.map((agentRun) => (
						<section key={agentRun.id} className="hx-workbench-section-thread" aria-label={`${agentRoleLabels[agentRun.role]}, round ${agentRun.round}`}>
							<button type="button" className="hx-workbench-section-head" onClick={() => onOpen(`agent:${agentRun.id}`)}>
								<span className={`hx-workbench-dot hx-tone-${hxStatuses[agentDot(agentRun)].tone}`} aria-hidden="true" />
								<b>{agentRoleLabels[agentRun.role]}</b> · {agentTitle(agentRun)}
								<span className="hx-workbench-section-meta">{threadMeta(agentRun)}</span>
								<span className="hx-workbench-section-open">Open</span>
							</button>
							<Conversation client={client} agentRun={agentRun} />
						</section>
					))}
				</div>
			))}
		</div>
	);
}

