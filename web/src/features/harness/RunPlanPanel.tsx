import { useEffect, useMemo, useState } from 'react';
import { FileText, RefreshCw } from 'lucide-react';
import type { HarnessRun, HarnessRunPlan } from '../../lib/types';
import { MarkdownPreview } from '../content-viewer/renderers/MarkdownPreview';
import type { HarnessClient } from './harnessClient';
import { plainError } from './useHarness';

/** README first, then the rest by name: the order a person reads a plan folder in. */
export function orderPlanFiles<T extends { path: string }>(files: T[]): T[] {
	const isReadme = (path: string) => /(^|\/)readme\.md$/i.test(path);
	return [...files].sort((a, b) => Number(isReadme(b.path)) - Number(isReadme(a.path)) || a.path.localeCompare(b.path, undefined, { numeric: true }));
}

/** Where the plan was read from, in plain words. */
export function planSourceLine(plan: HarnessRunPlan): string {
	if (plan.from === 'worktree') return 'Live from the harness\'s own copy. Not merged yet.';
	if (plan.from === 'branch') return `From branch ${plan.branch}. Not merged yet.`;
	return '';
}

/**
 * The plan a harness run wrote, readable at any time before it is merged: live
 * while the run works, from its branch once it stopped. Read only: the run
 * owns its copy until a person takes over or the branch is merged.
 */
export function RunPlanPanel({ client, run }: { client: HarnessClient; run: HarnessRun }) {
	const [plan, setPlan] = useState<HarnessRunPlan | undefined>();
	const [error, setError] = useState('');
	const [chosen, setChosen] = useState<string | undefined>();
	const [reload, setReload] = useState(0);
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
	const file = files.find((candidate) => candidate.path === chosen) ?? files[0];

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
				<ul className="hx-run-plan-files" aria-label="Plan files">
					{files.map((candidate) => (
						<li key={candidate.path}>
							<button type="button" className={candidate.path === file?.path ? 'on' : undefined} aria-current={candidate.path === file?.path ? 'true' : undefined} onClick={() => setChosen(candidate.path)}>
								<FileText size={13} aria-hidden="true" />{candidate.path.replace(/^plans\//, '')}{candidate.status === 'added' ? ' · new' : ''}
							</button>
						</li>
					))}
				</ul>
			)}
			{file && (
				<article className="hx-run-plan-doc" aria-label={file.path}>
					{files.length === 1 && <p className="hx-run-plan-path">{file.path.replace(/^plans\//, '')}</p>}
					{file.truncated
						? <p className="hx-muted">This file is too large to show here.</p>
						: /\.md$/i.test(file.path) ? <MarkdownPreview content={file.content} /> : <pre className="hx-run-plan-text">{file.content}</pre>}
				</article>
			)}
		</section>
	);
}
