import { useEffect, useMemo, useRef, useState } from 'react';
import { ChevronLeft, ChevronRight, RefreshCw } from 'lucide-react';
import type { HarnessRun, HarnessRunPlan } from '../../lib/types';
import { MarkdownPreview } from '../content-viewer/renderers/MarkdownPreview';
import type { HarnessClient } from './harnessClient';
import { plainError } from './useHarness';

export function commonFolder(paths: string[]): string {
	if (paths.length === 0) return '';
	const folders = paths.map((path) => path.split('/').slice(0, -1));
	const shared: string[] = [];
	for (let index = 0; index < folders[0].length; index += 1) {
		const part = folders[0][index];
		if (!folders.every((parts) => parts[index] === part)) break;
		shared.push(part);
	}
	return shared.length ? `${shared.join('/')}/` : '';
}

const readmePattern = /(^|\/)readme\.md$/i;

function readingRank(path: string): number {
	const lower = path.toLowerCase();
	if (lower === 'readme.md') return 0;
	if (lower === 'implementation-plan.md') return 1;
	if (lower.startsWith('solution/')) return 2;
	if (lower.startsWith('scenario/')) return 3;
	if (lower.startsWith('automation/')) return 4;
	if (lower.endsWith('.md')) return 5;
	return 6;
}

export function orderPlanFiles<T extends { path: string }>(files: T[]): T[] {
	const folder = commonFolder(files.map((file) => file.path));
	const inner = (path: string) => path.slice(folder.length);
	return [...files].sort((a, b) => readingRank(inner(a.path)) - readingRank(inner(b.path))
		|| Number(readmePattern.test(b.path)) - Number(readmePattern.test(a.path))
		|| a.path.localeCompare(b.path, undefined, { numeric: true }));
}

export function planSourceLine(plan: HarnessRunPlan): string {
	if (plan.from === 'worktree') return 'Live from the harness\'s own copy. Not merged yet.';
	if (plan.from === 'branch') return `From branch ${plan.branch}. Not merged yet.`;
	return '';
}

export function RunPlanPanel({ client, run }: { client: HarnessClient; run: HarnessRun }) {
	const [plan, setPlan] = useState<HarnessRunPlan | undefined>();
	const [error, setError] = useState('');
	const [chosen, setChosen] = useState<string | undefined>();
	const [reload, setReload] = useState(0);
	const docRef = useRef<HTMLElement | null>(null);
	// Paging lands on the next document's start, because the reader just finished the one before.
	const turnTo = (path: string) => {
		setChosen(path);
		requestAnimationFrame(() => docRef.current?.scrollIntoView?.({ block: 'start', behavior: 'smooth' }));
	};
	const version = `${run.id}:${run.revision}:${run.state}:${run.currentStageId ?? ''}:${run.currentStage?.state ?? ''}:${reload}`;
	useEffect(() => {
		if (!client.runPlan) return;
		const controller = new AbortController();
		setError('');
		client.runPlan(run.id, controller.signal)
			.then((result) => { if (!controller.signal.aborted) setPlan(result); })
			.catch((caught) => { if (!controller.signal.aborted) setError(plainError(caught, 'The plan could not be read.')); });
		return () => controller.abort();
	}, [client, run.id, version]);
	const files = useMemo(() => orderPlanFiles(plan?.files ?? []), [plan]);
	const folder = useMemo(() => commonFolder(files.map((candidate) => candidate.path)), [files]);
	const file = files.find((candidate) => candidate.path === chosen) ?? files[0];
	const position = file ? files.indexOf(file) : -1;
	const previous = position > 0 ? files[position - 1] : undefined;
	const next = position >= 0 && position < files.length - 1 ? files[position + 1] : undefined;
	const inner = (path: string) => path.slice(folder.length);
	const fileName = (path: string) => inner(path).split('/').pop() ?? path;

	if (!client.runPlan) return <p className="hx-muted">The plan cannot be read here.</p>;
	if (error) return <p className="hx-banner" role="alert">{error}</p>;
	if (!plan) return <p className="hx-muted" aria-busy="true">Reading the plan…</p>;
	return (
		<section className="hx-run-plan" aria-label="The plan">
			<header className="hx-run-plan-head">
				<span className="hx-muted">{files.length > 0 ? planSourceLine(plan) : plan.from === '' ? 'This run\'s copy and branch are gone. Once merged, the plan is on the board.' : 'The Planner has not written a plan yet.'}</span>
				<button type="button" className="secondary hx-small-button" onClick={() => setReload((value) => value + 1)}><RefreshCw size={13} aria-hidden="true" />Refresh</button>
			</header>
			{files.length > 1 && (
				<div className="hx-run-plan-index">
					{folder && <p className="hx-run-plan-folder">{folder.replace(/^plans\//, '')}</p>}
					<ol className="hx-run-plan-files" aria-label="Plan files, in reading order">
						{files.map((candidate, index) => {
							const at = inner(candidate.path);
							const sub = at.includes('/') ? at.slice(0, at.lastIndexOf('/') + 1) : '';
							const current = candidate.path === file?.path;
							return (
								<li key={candidate.path}>
									<button type="button" className={current ? 'on' : undefined} aria-current={current ? 'true' : undefined} onClick={() => setChosen(candidate.path)}>
										<span className="hx-run-plan-step hx-num">{index + 1}</span>
										<span className="hx-run-plan-name">{sub && <span className="hx-run-plan-sub">{sub}</span>}{fileName(candidate.path)}</span>
										{candidate.status === 'added' && <span className="hx-run-plan-status added" title="New in this run">A</span>}
										{candidate.status === 'modified' && <span className="hx-run-plan-status modified" title="Changed in this run">M</span>}
									</button>
								</li>
							);
						})}
					</ol>
				</div>
			)}
			{file && (
				<article ref={docRef} className="hx-run-plan-doc" aria-label={file.path}>
					{files.length === 1 && <p className="hx-run-plan-path">{file.path.replace(/^plans\//, '')}</p>}
					{file.truncated
						? <p className="hx-muted">This file is too large to show here.</p>
						: /\.md$/i.test(file.path) ? <MarkdownPreview content={file.content} /> : <pre className="hx-run-plan-text">{file.content}</pre>}
				</article>
			)}
			{files.length > 1 && (
				<nav className="hx-run-plan-pager" aria-label="Read the plan in order">
					{previous ? <button type="button" className="secondary hx-small-button" onClick={() => turnTo(previous.path)}><ChevronLeft size={14} aria-hidden="true" /><span>Previous<span className="hx-run-plan-pager-name">{fileName(previous.path)}</span></span></button> : <span />}
					<span className="hx-muted hx-num">{position + 1} of {files.length}</span>
					{next ? <button type="button" className="secondary hx-small-button" onClick={() => turnTo(next.path)}><span>Next<span className="hx-run-plan-pager-name">{fileName(next.path)}</span></span><ChevronRight size={14} aria-hidden="true" /></button> : <span />}
				</nav>
			)}
		</section>
	);
}
