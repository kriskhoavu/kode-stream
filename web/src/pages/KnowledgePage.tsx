import { lazy, Suspense, useEffect, useState } from 'react';
import { CheckCircle2, X } from 'lucide-react';
import type { KnowledgeLocation } from '../app/router';
import { KnowledgeActions } from '../features/knowledge/KnowledgeActions';
import { KnowledgeBrowser } from '../features/knowledge/KnowledgeBrowser';
import { KnowledgeReader } from '../features/knowledge/KnowledgeReader';
import { KnowledgeE2ESidePanel } from '../features/e2e-testing/KnowledgeE2ESidePanel';
import { useKnowledgeController } from '../features/knowledge/useKnowledgeController';
import type { KnowledgeActionResult, WorkspaceConfig } from '../lib/types';
import '../features/knowledge/knowledge.css';

const KnowledgeGraph = lazy(() => import('../features/knowledge/KnowledgeGraph').then((module) => ({ default: module.KnowledgeGraph })));

export function KnowledgePage({ workspaces, activeWorkspace, location, onLocationChange }: { workspaces: WorkspaceConfig[]; activeWorkspace?: WorkspaceConfig; location?: KnowledgeLocation; onLocationChange: (location: KnowledgeLocation) => void }) {
	const controller = useKnowledgeController(activeWorkspace ? [activeWorkspace] : workspaces, location, onLocationChange);
	if (!workspaces.length) return <section className="empty-state"><h1>Knowledge</h1><p>Add a workspace to discover structured Markdown Wikis.</p></section>;
	return <section className="knowledge-page">
		<header className="knowledge-header">
			<div className="knowledge-title"><h1>Knowledge</h1><p>Browse structured documentation, follow relationships, and inspect the Wiki graph.</p></div>
			<div className="knowledge-toolbar">
			<label className="knowledge-field"><span>Wiki</span><select aria-label="Knowledge Wiki" value={controller.wiki?.root ?? ''} disabled={!controller.wikis.length} onChange={(event) => controller.updateLocation({ root: event.target.value, slug: undefined, view: 'browse' })}>{controller.wikis.map((wiki) => <option key={wiki.root} value={wiki.root}>{wiki.displayName}</option>)}</select></label>
			<div className="knowledge-views" aria-label="Knowledge view"><button type="button" className={location?.view !== 'graph' ? 'active' : ''} onClick={() => controller.updateLocation({ view: controller.page ? 'read' : 'browse' })}>Pages</button><button type="button" className={location?.view === 'graph' ? 'active' : ''} disabled={!controller.pages.length} onClick={() => controller.updateLocation({ view: 'graph' })}>Graph</button></div>
			{controller.workspace && <KnowledgeActions workspaceId={controller.workspace.id} settings={controller.workspace.knowledge} root={controller.wiki?.root} busy={controller.actionBusy} onRun={controller.runAction} />}
			</div>
		</header>
		<KnowledgeActionNotification result={controller.actionResult} />
		{(controller.loading || controller.error || controller.notice) && <div aria-live="polite" className={controller.error ? 'knowledge-status error' : 'knowledge-status'}>{controller.loading ? 'Loading Knowledge…' : controller.error || controller.notice}</div>}
		{!controller.loading && !controller.error && !controller.wikis.length && <div className="empty-state"><h2>No structured Wikis detected</h2><p>A source qualifies when it contains <code>index.md</code> and valid pages with <code>slug</code> and <code>title</code> front matter.</p></div>}
		{controller.wiki && location?.view !== 'graph' && <KnowledgeBrowser pages={controller.pages} selectedSlug={controller.page?.slug} warnings={controller.warnings} onSelect={(slug) => controller.updateLocation({ slug, view: 'read' })} sidePanel={controller.detail && location?.view === 'read' && controller.workspace && controller.detail.path.startsWith('e2e-testing/') ? <KnowledgeE2ESidePanel workspaceId={controller.workspace.id} root={controller.wiki.root} detail={controller.detail} /> : undefined}>
			{controller.detailLoading ? <div className="knowledge-welcome">Loading page…</div> : controller.detail && location?.view === 'read' ? <KnowledgeReader detail={controller.detail} onNavigate={(slug) => controller.updateLocation({ slug, view: 'read' })} /> : undefined}
		</KnowledgeBrowser>}
		{controller.graphLoading && <div className="empty-state">Loading graph…</div>}
		{controller.graph && location?.view === 'graph' && <Suspense fallback={<div className="empty-state">Loading graph renderer…</div>}><KnowledgeGraph graph={controller.graph} pages={controller.pages} selectedSlug={controller.page?.slug} selectedDetail={controller.detail} onSelect={(slug) => controller.updateLocation({ slug, view: 'graph' })} onOpenDetails={(slug) => controller.updateLocation({ slug, view: 'read' })} /></Suspense>}
	</section>;
}

function KnowledgeActionNotification({ result }: { result: KnowledgeActionResult | null }) {
	const [visibleResult, setVisibleResult] = useState<KnowledgeActionResult | null>(null);
	useEffect(() => {
		if (!result) return;
		setVisibleResult(result);
		const timeout = window.setTimeout(() => setVisibleResult((current) => current?.completedAt === result.completedAt ? null : current), 5000);
		return () => window.clearTimeout(timeout);
	}, [result]);
	if (!visibleResult) return null;
	const label = visibleResult.ok ? `${visibleResult.operation} completed` : visibleResult.message || `${visibleResult.operation} failed`;
	return <aside className={visibleResult.ok ? 'knowledge-notification success' : 'knowledge-notification error'} role="status" aria-live="polite"><div><strong>{visibleResult.ok && <CheckCircle2 size={16} />} {label}</strong>{visibleResult.log && <details><summary>View action log{visibleResult.logTruncated ? ' (truncated)' : ''}</summary><pre>{visibleResult.log}</pre></details>}</div><button type="button" className="icon-button" aria-label="Dismiss Knowledge notification" onClick={() => setVisibleResult(null)}><X size={16} /></button></aside>;
}
