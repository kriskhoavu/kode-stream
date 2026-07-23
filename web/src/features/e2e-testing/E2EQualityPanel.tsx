import { useState } from 'react';
import { Bot, RefreshCw } from 'lucide-react';
import type { E2ERunbook } from '../../lib/types';
import { AISessionLaunchDialog } from '../ai-session/AISessionLaunchDialog';

export function E2EQualityPanel({ workspaceId, runbooks = [], diagnostic, onRefresh }: { workspaceId: string; runbooks?: E2ERunbook[]; diagnostic?: string; onRefresh?: () => void }) {
	const [selected, setSelected] = useState<E2ERunbook | null>(null);
	return <section className="automation-test-panel e2e-quality-panel" aria-label="E2E Test">
		<div className="quality-section-heading"><strong>E2E Test</strong><span>Agent-run browser journeys</span></div>
		{runbooks.length === 0 ? <span className="verification-note">{diagnostic || 'No E2E coverage.'}</span> : <div className="automation-discovered-specs" aria-label="E2E runbooks">{runbooks.map((runbook) => <article className="automation-suggestion-card e2e-runbook-card" key={`${runbook.source}-${runbook.path}`}>
			<div><strong>{runbook.title}</strong><span>{runbook.source} · {runbook.path}</span>{runbook.latestResult ? <span>{runbook.latestResult.status}{runbook.latestResult.provider ? ` · ${runbook.latestResult.provider}` : ''}{runbook.latestResult.environment ? ` · ${runbook.latestResult.environment}` : ''}{runbook.latestResult.failedStep ? ` · failed: ${runbook.latestResult.failedStep}` : ''}</span> : <span>{runbook.diagnostic || 'Not run'}</span>}</div>
			<button className="secondary" type="button" onClick={() => setSelected(runbook)}><Bot size={14} /> Run E2E test</button>
		</article>)}</div>}
		{onRefresh && <button className="secondary" type="button" onClick={onRefresh}><RefreshCw size={14} /> Refresh E2E results</button>}
		{selected && <AISessionLaunchDialog workspaceTarget={{ workspaceId, contextPath: selected.path }} e2eRunbook={selected} onClose={() => setSelected(null)} onLaunched={() => { setSelected(null); onRefresh?.(); }} />}
	</section>;
}
