import { useState } from 'react';
import type { CSSProperties, ReactNode } from 'react';
import { ChevronDown, ChevronRight, FileText, Folder, MessagesSquare } from 'lucide-react';
import type { HarnessAgentRun, HarnessRunFile, HarnessRunPlan, SessionChanges, SessionFile } from '../../../lib/types';
import { agentStatus } from '../runAgents';
import type { LiveStep } from '../runAgents';
import { commonFolder } from '../RunPlanPanel';
import { agentRoleLabels, hxStatuses, plural } from '../vocabulary';
import { type WorkbenchKey, statusLetter, statusClass, type Loaded, type TreeDir } from './workbenchTypes';

export function buildTree(paths: string[], root = ''): TreeDir {
	const top: TreeDir = { name: '', dirs: [], files: [] };
	for (const path of paths) {
		const parts = path.slice(root.length).split('/');
		let node = top;
		parts.slice(0, -1).forEach((part) => {
			let next = node.dirs.find((dir) => dir.name === part);
			if (!next) { next = { name: part, dirs: [], files: [] }; node.dirs.push(next); }
			node = next;
		});
		node.files.push(path);
	}
	const squash = (dir: TreeDir): TreeDir => {
		let node = { ...dir, dirs: dir.dirs.map(squash) };
		while (node.files.length === 0 && node.dirs.length === 1 && node.name) {
			const only = node.dirs[0];
			node = { name: `${node.name}/${only.name}`, dirs: only.dirs, files: only.files };
		}
		return node;
	};
	return { ...top, dirs: top.dirs.map(squash) };
}

export function Tree({ dir, depth, prefix, active, row }: { dir: TreeDir; depth: number; prefix: string; active?: WorkbenchKey; row: (path: string, depth: number) => ReactNode }) {
	const [closed, setClosed] = useState<Record<string, boolean>>({});
	return (
		<>
			{dir.dirs.map((child) => {
				const id = `${prefix}${child.name}`;
				const isOpen = !closed[id];
				return (
					<div key={id} role="none">
						<div className="explorer-tree-row" role="treeitem" aria-expanded={isOpen} aria-level={depth + 1} aria-selected={false} style={{ '--explorer-depth': depth } as CSSProperties}>
							<button className="explorer-row-toggle" type="button" tabIndex={-1} aria-label={`${isOpen ? 'Collapse' : 'Expand'} ${child.name}`} onClick={() => setClosed((current) => ({ ...current, [id]: isOpen }))}>{isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}</button>
							<button className="explorer-row-main" type="button" onClick={() => setClosed((current) => ({ ...current, [id]: isOpen }))}><Folder size={15} aria-hidden="true" /><span className="explorer-row-label directory" title={child.name}>{child.name}</span></button>
						</div>
						{isOpen && <Tree dir={child} depth={depth + 1} prefix={`${id}/`} active={active} row={row} />}
					</div>
				);
			})}
			{dir.files.map((path) => <div key={path} role="none">{row(path, depth)}</div>)}
		</>
	);
}

export function FileRow({ path, depth, selected, additions, deletions, status, onOpen }: { path: string; depth: number; selected: boolean; additions?: number; deletions?: number; status?: string; onOpen: () => void }) {
	return (
		<div className={selected ? 'explorer-tree-row selected' : 'explorer-tree-row'} role="treeitem" aria-level={depth + 1} aria-selected={selected} style={{ '--explorer-depth': depth } as CSSProperties}>
			<button className="explorer-row-toggle" type="button" tabIndex={-1} disabled />
			<button className="explorer-row-main" type="button" title={path} onClick={onOpen}>
				<FileText size={15} aria-hidden="true" />
				<span className="explorer-row-label file">{path.split('/').pop()}</span>
				{additions !== undefined ? <Delta additions={additions} deletions={deletions ?? 0} /> : <span className="hx-workbench-delta">{status && statusLetter[status] ? <span className={`hx-workbench-letter ${statusClass[statusLetter[status]]}`} title={statusClass[statusLetter[status]]}>{statusLetter[status]}</span> : null}</span>}
			</button>
		</div>
	);
}

export function Delta({ additions, deletions }: { additions: number; deletions: number }) {
	return (
		<span className="hx-workbench-delta">
			{additions > 0 && <span className="hx-add">+{additions}</span>}
			{additions > 0 && deletions > 0 && ' '}
			{deletions > 0 && <span className="hx-del">−{deletions}</span>}
		</span>
	);
}

export function PlanTree({ plan, files, active, onOpen }: { plan: Loaded<HarnessRunPlan>; files: HarnessRunFile[]; active?: WorkbenchKey; onOpen: (key: WorkbenchKey) => void }) {
	if (plan.error) return <p className="hx-banner" role="alert">{plan.error}</p>;
	if (!plan.value) return <p className="hx-workbench-empty" aria-busy="true">Reading the plan…</p>;
	if (files.length === 0) return <p className="hx-workbench-empty">{plan.value.from === '' ? 'This run\'s copy and branch are gone.' : 'The Planner has not written a plan yet.'}</p>;
	const folder = commonFolder(files.map((file) => file.path));
	const ordered = new Map(files.map((file, index) => [file.path, index]));
	const tree = buildTree(files.map((file) => file.path), folder);
	const sortTree = (dir: TreeDir): TreeDir => ({ ...dir, dirs: dir.dirs.map(sortTree), files: [...dir.files].sort((a, b) => (ordered.get(a) ?? 0) - (ordered.get(b) ?? 0)) });
	return (
		<Tree dir={sortTree(tree)} depth={0} prefix="plan:" active={active} row={(path, depth) => {
			const file = files.find((candidate) => candidate.path === path);
			return <FileRow path={path} depth={depth} selected={active === `plan:${path}`} status={file?.status} onOpen={() => onOpen(`plan:${path}`)} />;
		}} />
	);
}

export function ChangesTree({ changes, files, active, onOpen }: { changes: Loaded<SessionChanges>; files: SessionFile[]; active?: WorkbenchKey; onOpen: (key: WorkbenchKey) => void }) {
	if (changes.error) return <p className="hx-banner" role="alert">{changes.error}</p>;
	if (!changes.value) return <p className="hx-workbench-empty" aria-busy="true">Reading the changes…</p>;
	if (changes.value.availability !== 'available') return <p className="hx-workbench-empty">This run's copy and branch are gone, so its changes cannot be read here.</p>;
	if (files.length === 0) return <p className="hx-workbench-empty">No code has changed yet.</p>;
	const counted = files as (SessionFile & { additions?: number; deletions?: number })[];
	return (
		<>
			{[...counted].sort((a, b) => a.path.localeCompare(b.path)).map((file) => {
				const key: WorkbenchKey = `file:${file.path}`;
				const slash = file.path.lastIndexOf('/');
				return (
					<div key={file.path} className={active === key ? 'explorer-tree-row hx-workbench-change selected' : 'explorer-tree-row hx-workbench-change'} role="treeitem" aria-selected={active === key}>
						<button className="explorer-row-main" type="button" title={file.path} onClick={() => onOpen(key)}>
							<FileText size={15} aria-hidden="true" />
							<span className="hx-workbench-change-name">
								<span className="explorer-row-label file">{file.path.slice(slash + 1)}</span>
								{slash > 0 && <small dir="rtl"><bdi>{file.path.slice(0, slash)}</bdi></small>}
							</span>
							<Delta additions={file.additions ?? 0} deletions={file.deletions ?? 0} />
						</button>
					</div>
				);
			})}
		</>
	);
}

export function AgentsList({ steps, active, onOpen }: { steps: LiveStep[]; active?: WorkbenchKey; onOpen: (key: WorkbenchKey) => void }) {
	if (steps.length === 0) return <p className="hx-workbench-empty">No agent has worked on this task yet.</p>;
	return (
		<>
			<button type="button" className={active === 'stage:*' ? 'hx-workbench-all on' : 'hx-workbench-all'} onClick={() => onOpen('stage:*')} title="Read every conversation in order">
				<MessagesSquare size={15} aria-hidden="true" /><span>All steps</span><small>Read in order</small>
			</button>
			{steps.map((step) => (
				<div key={step.id} role="group" aria-label={step.title}>
					<button type="button" className={active === `stage:${step.id}` ? 'hx-workbench-group on' : 'hx-workbench-group'} onClick={() => onOpen(`stage:${step.id}`)} title={`Read all of ${step.title}`}>
						<span>{step.title}</span><span className="hx-workbench-group-all">Read all {step.agentRuns.length}</span>
					</button>
					{step.agentRuns.map((agentRun) => {
						const key: WorkbenchKey = `agent:${agentRun.id}`;
						return (
							<div key={agentRun.id} className={active === key ? 'explorer-tree-row selected' : 'explorer-tree-row'} role="treeitem" aria-selected={active === key} style={{ '--explorer-depth': 0 } as CSSProperties}>
								<button className="explorer-row-toggle" type="button" tabIndex={-1} disabled />
								<button className="explorer-row-main" type="button" onClick={() => onOpen(key)}>
									<span className={`hx-workbench-dot hx-tone-${hxStatuses[agentDot(agentRun)].tone}`} title={hxStatuses[agentDot(agentRun)].label} />
									<span className="explorer-row-label file hx-workbench-agent"><b>{agentRoleLabels[agentRun.role]}</b> <span>· {agentTitle(agentRun)}</span></span>
								</button>
							</div>
						);
					})}
				</div>
			))}
		</>
	);
}

export function agentDot(agentRun: HarnessAgentRun) {
	if (agentRun.state === 'succeeded' && agentRun.blocking > 0) return 'failed' as const;
	return agentStatus(agentRun);
}

export function agentTitle(agentRun: HarnessAgentRun): string {
	const review = agentRun.role === 'plan_reviewer' || agentRun.role === 'code_reviewer';
	const verdict = review && agentRun.verdict ? (agentRun.verdict === 'approve' ? ' · approved' : ` · ${plural(agentRun.blocking, 'must fix', 'must fix')}`) : '';
	return `Round ${agentRun.round}${verdict}`;
}

