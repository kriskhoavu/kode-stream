import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { useHorizontalDrag } from '../../components/useHorizontalDrag';
import { BookOpen, ChevronRight, FileText, Folder, GripVertical, PanelRightClose, PanelRightOpen, Search } from 'lucide-react';
import type { KnowledgePage, KnowledgeWarning } from '../../lib/types';
import { KnowledgeWarnings } from './KnowledgeWarnings';

export function KnowledgeBrowser({ pages, selectedSlug, warnings, onSelect, children, sidePanel, rootName = 'wiki' }: { pages: KnowledgePage[]; selectedSlug?: string; warnings: KnowledgeWarning[]; onSelect: (slug: string) => void; children?: ReactNode; sidePanel?: ReactNode; rootName?: string }) {
	const [query, setQuery] = useState('');
	const [sidePanelCollapsed, setSidePanelCollapsed] = useState(false);
	const [sidePanelWidth, setSidePanelWidth] = useState(300);
	const [expandedDomains, setExpandedDomains] = useState<Set<string>>(() => new Set([rootFolderPath]));
	const navigationRef = useRef<HTMLElement | null>(null);
	const startDrag = useHorizontalDrag();
	const filtered = useMemo(() => {
		const needle = query.trim().toLowerCase();
		return needle ? pages.filter((page) => [page.title, page.slug, page.summary ?? '', ...page.roles, ...page.topics].some((value) => value.toLowerCase().includes(needle))) : pages;
	}, [pages, query]);
	const tree = useMemo(() => buildFolderTree(filtered, pages, rootName), [filtered, pages, rootName]);
	useEffect(() => {
		if (!selectedSlug) return;
		const selectedPage = pages.find((page) => page.slug === selectedSlug);
		if (!selectedPage) return;
		setQuery('');
		const keys = ancestorFolders(selectedPage.path);
		setExpandedDomains((current) => {
			const next = new Set(current);
			for (const key of keys) next.add(key);
			return next;
		});
	}, [pages, selectedSlug]);
	useEffect(() => {
		if (!selectedSlug) return;
		const entry = Array.from(navigationRef.current?.querySelectorAll<HTMLButtonElement>('[data-knowledge-slug]') ?? []).find((candidate) => candidate.dataset.knowledgeSlug === selectedSlug);
		if (!entry) return;
		entry.scrollIntoView?.({ block: 'nearest' });
		entry.focus({ preventScroll: true });
	}, [expandedDomains, query, selectedSlug]);
	const moveFocus = (event: React.KeyboardEvent<HTMLButtonElement>, slug: string) => {
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
			event.preventDefault();
			const controls = Array.from(event.currentTarget.closest('nav')?.querySelectorAll<HTMLButtonElement>('[data-knowledge-entry]') ?? []);
			const current = controls.indexOf(event.currentTarget);
			controls[Math.max(0, Math.min(controls.length - 1, current + (event.key === 'ArrowDown' ? 1 : -1)))]?.focus();
		}
		if (event.key === 'Enter') { event.preventDefault(); onSelect(slug); }
	};
	const startSidePanelResize = (event: React.PointerEvent<HTMLButtonElement>) => {
		const startingWidth = sidePanelWidth;
		startDrag(event, { move: (deltaX) => setSidePanelWidth(Math.max(260, Math.min(640, startingWidth - deltaX))) });
	};
	const renderFolder = (node: FolderNode, level = 0): ReactNode => {
		const label = node.path || node.name;
		const collapsible = node.folders.length > 0 || node.pages.length > 0;
		const expanded = query.trim() !== '' || expandedDomains.has(node.path);
		const toggleFolder = () => setExpandedDomains((current) => {
			const next = new Set(current);
			if (next.has(node.path)) next.delete(node.path); else next.add(node.path);
			return next;
		});
		const openOrToggleLanding = () => {
			if (query.trim() === '' && node.landingPage?.slug === selectedSlug) {
				toggleFolder();
				return;
			}
			onSelect(node.landingPage!.slug);
			if (!expanded) setExpandedDomains((current) => new Set(current).add(node.path));
		};
		const handleLandingKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
			if (event.key === 'Enter') {
				event.preventDefault();
				openOrToggleLanding();
				return;
			}
			moveFocus(event, node.landingPage!.slug);
		};
		const renderPage = (page: KnowledgePage) => {
			const pageWarnings = warnings.filter((warning) => warning.slug === page.slug || warning.path === page.path).length;
			const showType = !page.tier;
			return <button data-knowledge-entry data-knowledge-slug={page.slug} className={page.slug === selectedSlug ? 'knowledge-page-row knowledge-tree-row active' : 'knowledge-page-row knowledge-tree-row'} style={treeDepth(level + 1)} key={page.slug} title={page.path} onClick={() => onSelect(page.slug)} onKeyDown={(event) => moveFocus(event, page.slug)}>
				<span className="knowledge-tree-spacer" aria-hidden="true" />
				<FileText className="knowledge-tree-icon" size={16} />
				<strong className="knowledge-page-title">{page.title}</strong>
				{(showType || pageWarnings > 0) && <small>{showType ? <span className="knowledge-page-type">{displayPageType(page.pageType)}</span> : null}{pageWarnings ? <span className="knowledge-page-warning">{pageWarnings} warning{pageWarnings === 1 ? '' : 's'}</span> : null}</small>}
			</button>;
		};
		const folderClass = ['knowledge-domain-link', tierClass(node.name), node.path === rootFolderPath ? 'knowledge-root-link' : ''].filter(Boolean).join(' ');
		return <section className={node.path === rootFolderPath ? 'knowledge-domain knowledge-domain-root' : 'knowledge-domain'} key={node.path}>
			<div className="knowledge-domain-header knowledge-tree-row" style={treeDepth(level)}>
				{collapsible
					? <button type="button" className={expanded ? 'knowledge-domain-toggle expanded' : 'knowledge-domain-toggle'} aria-label={`${expanded ? 'Collapse' : 'Expand'} ${label}`} aria-expanded={expanded} onClick={toggleFolder}><ChevronRight size={14} /></button>
					: <span className="knowledge-tree-spacer" aria-hidden="true" />}
				<h3>{node.landingPage
					? <button data-knowledge-entry type="button" className={folderClass} onClick={openOrToggleLanding} onKeyDown={handleLandingKeyDown} aria-label={`Open ${label} index`}><Folder className="knowledge-tree-icon" size={16} /><span>{node.name}</span></button>
					: <button data-knowledge-entry type="button" className={folderClass} aria-expanded={collapsible ? expanded : undefined} onClick={toggleFolder} onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); toggleFolder(); } }}><Folder className="knowledge-tree-icon" size={16} /><span>{node.name}</span></button>}
				</h3>
			</div>
			{expanded && node.folders.map((child) => renderFolder(child, level + 1))}
			{expanded && node.pages.map(renderPage)}
		</section>;
	};

	const browserStyle = sidePanel ? { '--knowledge-side-panel-width': `${sidePanelWidth}px` } as CSSProperties : undefined;
	return <div className={sidePanel ? `knowledge-browser with-side-panel${sidePanelCollapsed ? ' side-panel-collapsed' : ''}` : 'knowledge-browser'} style={browserStyle}>
		<div className="knowledge-browser-list">
			<label className="knowledge-search"><Search size={15} /><span className="knowledge-visually-hidden">Filter Knowledge pages</span><input aria-label="Filter Knowledge pages" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter pages" /></label>
			{pages.length === 0 && <div className="knowledge-empty"><h2>No valid pages indexed</h2><p>Add Markdown pages with <code>slug</code> and <code>title</code> front matter, then rescan.</p></div>}
			{pages.length > 0 && filtered.length === 0 && <p className="knowledge-empty">No pages match this filter.</p>}
			<nav ref={navigationRef} aria-label="Knowledge pages">{pages.length > 0 && renderFolder(tree)}</nav>
			<KnowledgeWarnings warnings={warnings} compact indexDiagnostics />
		</div>
		<section className="knowledge-content-pane" aria-label="Knowledge page content">{children ?? <div className="knowledge-welcome"><BookOpen size={28} /><h2>Select a page</h2><p>Choose an entry from the index to read its full content.</p></div>}</section>
		{sidePanel && <aside className={sidePanelCollapsed ? 'metadata-panel side-panel knowledge-side-panel collapsed' : 'metadata-panel side-panel knowledge-side-panel'} aria-label="Knowledge side panel"><div className="panel-header"><h2><BookOpen size={16} /> Knowledge</h2><button className="icon-button" type="button" title={sidePanelCollapsed ? 'Expand Knowledge Quality' : 'Collapse Knowledge Quality'} onClick={() => setSidePanelCollapsed((current) => !current)}>{sidePanelCollapsed ? <PanelRightOpen size={16} /> : <PanelRightClose size={16} />}</button></div>{!sidePanelCollapsed && sidePanel}{!sidePanelCollapsed && <button className="panel-resize-handle panel-resize-handle-right" type="button" aria-label="Resize Knowledge Quality panel" onPointerDown={startSidePanelResize}><GripVertical size={16} /></button>}</aside>}
	</div>;
}

const rootFolderPath = '';

interface FolderNode {
	name: string;
	path: string;
	folders: FolderNode[];
	pages: KnowledgePage[];
	landingPage?: KnowledgePage;
}

function folderOf(path: string): string {
	const slash = path.lastIndexOf('/');
	return slash < 0 ? rootFolderPath : path.slice(0, slash);
}

function ancestorFolders(path: string): string[] {
	const folders = [rootFolderPath];
	const parts = path.split('/').slice(0, -1);
	for (let index = 1; index <= parts.length; index += 1) folders.push(parts.slice(0, index).join('/'));
	return folders;
}

function findLandingPage(pages: KnowledgePage[]): KnowledgePage | undefined {
	return pages.find((page) => /(?:^|\/)index\.md$/i.test(page.path)) ?? pages.find((page) => /(?:^|\/)readme\.md$/i.test(page.path));
}

// Only folders holding a visible page appear, so the filter prunes the tree.
function buildFolderTree(visiblePages: KnowledgePage[], allPages: KnowledgePage[], rootName: string): FolderNode {
	const root: FolderNode = { name: rootName, path: rootFolderPath, folders: [], pages: [] };
	const folders = new Map<string, FolderNode>([[rootFolderPath, root]]);
	const folderFor = (path: string): FolderNode => {
		const existing = folders.get(path);
		if (existing) return existing;
		const node: FolderNode = { name: path.slice(path.lastIndexOf('/') + 1), path, folders: [], pages: [] };
		folders.set(path, node);
		folderFor(folderOf(path)).folders.push(node);
		return node;
	};
	for (const page of visiblePages) folderFor(folderOf(page.path)).pages.push(page);
	for (const node of folders.values()) {
		node.landingPage = findLandingPage(allPages.filter((page) => folderOf(page.path) === node.path));
		node.folders.sort((left, right) => left.name.localeCompare(right.name));
		node.pages.sort((left, right) => left.title.localeCompare(right.title));
	}
	return root;
}

// Any other tier stays muted instead of taking a colour nobody chose.
const markedTiers: Record<string, string> = { concepts: 'tier-concepts', reference: 'tier-reference' };

function tierClass(folderName: string): string {
	return markedTiers[folderName.toLowerCase()] ?? '';
}

function treeDepth(level: number): CSSProperties {
	return { '--knowledge-depth': level } as CSSProperties;
}

function displayPageType(pageType?: string): string {
	return (pageType || 'PAGE').replaceAll('_', '-');
}
