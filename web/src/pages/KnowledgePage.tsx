import { lazy, Suspense } from 'react';
import type { KnowledgeLocation } from '../app/router';
import { KnowledgeActions } from '../features/knowledge/KnowledgeActions';
import { KnowledgeBrowser } from '../features/knowledge/KnowledgeBrowser';
import { KnowledgeReader } from '../features/knowledge/KnowledgeReader';
import { useKnowledgeController } from '../features/knowledge/useKnowledgeController';
import type { WorkspaceConfig } from '../lib/types';
import '../features/knowledge/knowledge.css';

const KnowledgeGraph = lazy(() => import('../features/knowledge/KnowledgeGraph').then((module) => ({ default: module.KnowledgeGraph })));

export function KnowledgePage({ workspaces, location, onLocationChange, onOpenExplorer }: { workspaces: WorkspaceConfig[]; location?: KnowledgeLocation; onLocationChange: (location: KnowledgeLocation) => void; onOpenExplorer: (workspaceId: string, path: string) => void }) {
	const controller = useKnowledgeController(workspaces, location, onLocationChange);
	if (!workspaces.length) return <section className="empty-state"><h1>Knowledge</h1><p>Add a workspace to discover structured Markdown Wikis.</p></section>;
	return <section className="knowledge-page">
		<header className="knowledge-toolbar"><h1>Knowledge</h1>
			<label><span className="sr-only">Knowledge workspace</span><select aria-label="Knowledge workspace" value={controller.workspace?.id ?? ''} onChange={(event) => controller.updateLocation({ workspaceId: event.target.value, root: undefined, slug: undefined, view: 'browse' })}>{workspaces.map((workspace) => <option key={workspace.id} value={workspace.id}>{workspace.name}</option>)}</select></label>
			<label><span className="sr-only">Knowledge Wiki</span><select aria-label="Knowledge Wiki" value={controller.wiki?.root ?? ''} disabled={!controller.wikis.length} onChange={(event) => controller.updateLocation({ root: event.target.value, slug: undefined, view: 'browse' })}>{controller.wikis.map((wiki) => <option key={wiki.root} value={wiki.root}>{wiki.displayName}</option>)}</select></label>
			<div className="knowledge-views" aria-label="Knowledge view">{(['browse', 'read', 'graph'] as const).map((view) => <button key={view} className={(location?.view ?? 'browse') === view ? 'active' : ''} disabled={view !== 'browse' && !controller.page} onClick={() => controller.updateLocation({ view })}>{view[0].toUpperCase() + view.slice(1)}</button>)}</div>
			{controller.workspace && <KnowledgeActions workspaceId={controller.workspace.id} settings={controller.workspace.knowledge} root={controller.wiki?.root} busy={controller.actionBusy} result={controller.actionResult} onRun={controller.runAction} />}
		</header>
		<div aria-live="polite" className="knowledge-status">{controller.loading ? 'Loading Knowledge…' : controller.error || controller.notice}</div>
		{!controller.loading && !controller.error && !controller.wikis.length && <div className="empty-state"><h2>No structured Wikis detected</h2><p>A source qualifies when it contains <code>index.md</code> and valid pages with <code>slug</code> and <code>title</code> front matter.</p></div>}
		{controller.wiki && (location?.view ?? 'browse') === 'browse' && <KnowledgeBrowser pages={controller.pages} selectedSlug={controller.page?.slug} warnings={controller.warnings} onSelect={(slug) => controller.updateLocation({ slug, view: 'browse' })} onOpen={(slug) => controller.updateLocation({ slug, view: 'read' })} />}
		{controller.detailLoading && <div className="empty-state">Loading page…</div>}
		{controller.detail && location?.view === 'read' && controller.workspace && controller.wiki && <KnowledgeReader detail={controller.detail} onNavigate={(slug) => controller.updateLocation({ slug, view: 'read' })} onOpenExplorer={() => onOpenExplorer(controller.workspace!.id, `${controller.wiki!.root}/${controller.detail!.path}`)} />}
		{controller.graphLoading && <div className="empty-state">Loading graph…</div>}
		{controller.graph && location?.view === 'graph' && <Suspense fallback={<div className="empty-state">Loading graph renderer…</div>}><KnowledgeGraph graph={controller.graph} selectedSlug={controller.page?.slug} onSelect={(slug) => controller.updateLocation({ slug, view: 'graph' })} /></Suspense>}
	</section>;
}
