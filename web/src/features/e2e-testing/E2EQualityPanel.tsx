import { useMemo, useState } from 'react';
import { Bot, ChevronDown, ChevronRight, ExternalLink, RefreshCw, Search, X } from 'lucide-react';
import type { E2ERunbook, FileContent } from '../../lib/types';
import { api } from '../../shared/api';
import { ContentViewer } from '../content-viewer/ContentViewer';
import { AISessionLaunchDialog } from '../ai-session/AISessionLaunchDialog';

type EvidencePreview = {
	path: string;
	file?: FileContent;
	loading: boolean;
	error?: string;
};

export function E2EQualityPanel({
	workspaceId,
	runbooks = [],
	diagnostic,
	onRefresh,
	variant = 'standalone'
}: {
	workspaceId: string;
	runbooks?: E2ERunbook[];
	diagnostic?: string;
	onRefresh?: () => void;
	variant?: 'standalone' | 'nested';
}) {
	const [selected, setSelected] = useState<E2ERunbook | null>(null);
	const [evidencePreview, setEvidencePreview] = useState<EvidencePreview | null>(null);
	const [query, setQuery] = useState('');
	const [openSources, setOpenSources] = useState<Set<string>>(() => new Set(['plan']));
	const groups = useMemo(() => {
		const needle = query.trim().toLowerCase();
		const matches = runbooks.filter((runbook) => !needle || `${runbook.title} ${runbook.path} ${runbook.source} ${runbook.latestResult?.status ?? ''}`.toLowerCase().includes(needle));
		return ['plan', 'wiki']
			.map((source) => ({ source, runbooks: matches.filter((runbook) => runbook.source === source) }))
			.filter((group) => group.runbooks.length > 0);
	}, [query, runbooks]);

	const toggleSource = (source: string) => setOpenSources((current) => {
		const next = new Set(current);
		if (next.has(source)) next.delete(source);
		else next.add(source);
		return next;
	});

	const openEvidence = async (path: string) => {
		setEvidencePreview({ path, loading: true });
		try {
			const file = await api.workspaceFile(workspaceId, path);
			setEvidencePreview({ path, file, loading: false });
		} catch (caught) {
			setEvidencePreview({ path, loading: false, error: caught instanceof Error ? caught.message : 'Evidence could not be opened.' });
		}
	};

	return <section className={variant === 'nested' ? 'automation-test-panel e2e-quality-panel nested' : 'automation-test-panel e2e-quality-panel'} aria-label="E2E Test">
		<div className="quality-section-heading"><strong>E2E Test</strong><span>Agent-run browser journeys</span></div>
		{diagnostic && <span className="verification-note e2e-coverage-diagnostic" role="status">{diagnostic}</span>}
		{runbooks.length === 0
			? !diagnostic && <span className="verification-note">No E2E coverage.</span>
			: <>
				<label className="e2e-runbook-search">
					<Search size={14} />
					<span className="knowledge-visually-hidden">Filter E2E runbooks</span>
					<input aria-label="Filter E2E runbooks" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Find a runbook" />
				</label>
				{groups.length === 0
					? <span className="verification-note">No E2E runbooks match this filter.</span>
					: groups.map((group) => {
						const expanded = query.trim() !== '' || openSources.has(group.source);
						return <section className="e2e-runbook-group" key={group.source}>
							<button className="e2e-runbook-group-toggle" type="button" aria-expanded={expanded} onClick={() => toggleSource(group.source)}>
								{expanded ? <ChevronDown size={15} /> : <ChevronRight size={15} />}
								<span>{group.source === 'plan' ? 'Plan runbooks' : 'Canonical wiki journeys'}</span>
								<small>{group.runbooks.length}</small>
							</button>
							{expanded && <div className="automation-discovered-specs" aria-label={`${group.source} E2E runbooks`}>
								{group.runbooks.map((runbook) => <article className="automation-suggestion-card e2e-runbook-card" key={`${runbook.source}-${runbook.path}`}>
									<div>
										<strong>{runbook.title}</strong>
										<span>{runbook.source} · {runbook.path}</span>
										{runbook.latestResult
											? <span>{runbook.latestResult.status}{runbook.latestResult.provider ? ` · ${runbook.latestResult.provider}` : ''}{runbook.latestResult.environment ? ` · ${runbook.latestResult.environment}` : ''}{runbook.latestResult.failedStep ? ` · failed: ${runbook.latestResult.failedStep}` : ''}</span>
											: <span>{runbook.diagnostic || 'Not run'}</span>}
										{runbook.latestResult?.evidence.length
											? <span className="e2e-evidence-links" aria-label={`Evidence for ${runbook.title}`}>
												{runbook.latestResult.evidence.map((path) => <button type="button" className="e2e-evidence-link" key={path} onClick={() => void openEvidence(path)}><ExternalLink size={12} />{path}</button>)}
											</span>
											: null}
									</div>
									<button className="secondary" type="button" onClick={() => setSelected(runbook)}><Bot size={14} /> Run E2E test</button>
								</article>)}
							</div>}
						</section>;
					})}
			</>}
		{onRefresh && <button className="secondary" type="button" onClick={onRefresh}><RefreshCw size={14} /> Refresh E2E results</button>}
		{evidencePreview && <section className="e2e-evidence-preview" role="dialog" aria-modal="true" aria-labelledby="e2e-evidence-title">
			<header>
				<div><strong id="e2e-evidence-title">E2E evidence</strong><span>{evidencePreview.path}</span></div>
				<button className="icon-button" type="button" aria-label="Close evidence preview" onClick={() => setEvidencePreview(null)}><X size={16} /></button>
			</header>
			{evidencePreview.loading && <span role="status">Loading evidence...</span>}
			{evidencePreview.error && <span className="error" role="alert">{evidencePreview.error}</span>}
			{evidencePreview.file && <ContentViewer file={evidencePreview.file} content={evidencePreview.file.content} compact />}
		</section>}
		{selected && <AISessionLaunchDialog workspaceTarget={{ workspaceId, contextPath: selected.path }} e2eRunbook={selected} onClose={() => setSelected(null)} onLaunched={() => { setSelected(null); onRefresh?.(); }} />}
	</section>;
}
