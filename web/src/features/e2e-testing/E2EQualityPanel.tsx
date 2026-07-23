import { useMemo, useState } from 'react';
import { Bot, ChevronDown, ChevronRight, RefreshCw, Search } from 'lucide-react';
import type { E2ERunbook } from '../../lib/types';
import { AISessionLaunchDialog } from '../ai-session/AISessionLaunchDialog';

export function E2EQualityPanel({ workspaceId, runbooks = [], diagnostic, onRefresh }: { workspaceId: string; runbooks?: E2ERunbook[]; diagnostic?: string; onRefresh?: () => void }) {
	const [selected, setSelected] = useState<E2ERunbook | null>(null);
	const [query, setQuery] = useState('');
	const [openSources, setOpenSources] = useState<Set<string>>(() => new Set(['plan']));
	const groups = useMemo(() => {
		const needle = query.trim().toLowerCase();
		const matches = runbooks.filter((runbook) => !needle || `${runbook.title} ${runbook.path} ${runbook.source} ${runbook.latestResult?.status ?? ''}`.toLowerCase().includes(needle));
		return ['plan', 'wiki'].map((source) => ({ source, runbooks: matches.filter((runbook) => runbook.source === source) })).filter((group) => group.runbooks.length > 0);
	}, [query, runbooks]);
	const toggleSource = (source: string) => setOpenSources((current) => { const next = new Set(current); if (next.has(source)) next.delete(source); else next.add(source); return next; });
	return <section className="automation-test-panel e2e-quality-panel" aria-label="E2E Test">
		<div className="quality-section-heading"><strong>E2E Test</strong><span>Agent-run browser journeys</span></div>
		{runbooks.length === 0 ? <span className="verification-note">{diagnostic || 'No E2E coverage.'}</span> : <><label className="e2e-runbook-search"><Search size={14} /><span className="knowledge-visually-hidden">Filter E2E runbooks</span><input aria-label="Filter E2E runbooks" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Find a runbook" /></label>{groups.length === 0 ? <span className="verification-note">No E2E runbooks match this filter.</span> : groups.map((group) => { const expanded = query.trim() !== '' || openSources.has(group.source); return <section className="e2e-runbook-group" key={group.source}><button className="e2e-runbook-group-toggle" type="button" aria-expanded={expanded} onClick={() => toggleSource(group.source)}>{expanded ? <ChevronDown size={15} /> : <ChevronRight size={15} />}<span>{group.source === 'plan' ? 'Plan runbooks' : 'Canonical wiki journeys'}</span><small>{group.runbooks.length}</small></button>{expanded && <div className="automation-discovered-specs" aria-label={`${group.source} E2E runbooks`}>{group.runbooks.map((runbook) => <article className="automation-suggestion-card e2e-runbook-card" key={`${runbook.source}-${runbook.path}`}>
			<div><strong>{runbook.title}</strong><span>{runbook.source} · {runbook.path}</span>{runbook.latestResult ? <span>{runbook.latestResult.status}{runbook.latestResult.provider ? ` · ${runbook.latestResult.provider}` : ''}{runbook.latestResult.environment ? ` · ${runbook.latestResult.environment}` : ''}{runbook.latestResult.failedStep ? ` · failed: ${runbook.latestResult.failedStep}` : ''}</span> : <span>{runbook.diagnostic || 'Not run'}</span>}</div>
			<button className="secondary" type="button" onClick={() => setSelected(runbook)}><Bot size={14} /> Run E2E test</button>
		</article>)}</div>}</section>; })}</>}
		{onRefresh && <button className="secondary" type="button" onClick={onRefresh}><RefreshCw size={14} /> Refresh E2E results</button>}
		{selected && <AISessionLaunchDialog workspaceTarget={{ workspaceId, contextPath: selected.path }} e2eRunbook={selected} onClose={() => setSelected(null)} onLaunched={() => { setSelected(null); onRefresh?.(); }} />}
	</section>;
}
