import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { BookMarked, BookOpen, ChevronRight, GripVertical, Library, PanelRightClose, PanelRightOpen, Search } from 'lucide-react';
import type { KnowledgePage, KnowledgeWarning } from '../../lib/types';
import { KnowledgeWarnings } from './KnowledgeWarnings';
import { areaKey, bucketKey, groupingOf, rootGroupKey } from './taxonomy';

export function KnowledgeBrowser({ pages, selectedSlug, warnings, onSelect, children, sidePanel }: { pages: KnowledgePage[]; selectedSlug?: string; warnings: KnowledgeWarning[]; onSelect: (slug: string) => void; children?: ReactNode; sidePanel?: ReactNode }) {
	const [query, setQuery] = useState('');
	const [sidePanelCollapsed, setSidePanelCollapsed] = useState(false);
	const [sidePanelWidth, setSidePanelWidth] = useState(300);
	const [expandedDomains, setExpandedDomains] = useState<Set<string>>(() => new Set(['root']));
	const navigationRef = useRef<HTMLElement | null>(null);
	const filtered = useMemo(() => {
		const needle = query.trim().toLowerCase();
		return needle ? pages.filter((page) => [page.title, page.slug, page.summary ?? '', ...page.roles, ...page.topics].some((value) => value.toLowerCase().includes(needle))) : pages;
	}, [pages, query]);
	const domains = useMemo(() => buildDomainTree(filtered, pages), [filtered, pages]);
	useEffect(() => {
		if (!selectedSlug) return;
		const selectedPage = pages.find((page) => page.slug === selectedSlug);
		if (!selectedPage) return;
		setQuery('');
		const grouping = groupingOf(selectedPage);
		const keys = [bucketKey(grouping), areaKey(grouping)].filter(Boolean) as string[];
		setExpandedDomains((current) => {
			const next = new Set(current);
			for (const key of keys) next.add(key.toLowerCase());
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
		event.preventDefault();
		const startX = event.clientX;
		const startingWidth = sidePanelWidth;
		const resize = (moveEvent: PointerEvent) => setSidePanelWidth(Math.max(260, Math.min(640, startingWidth - (moveEvent.clientX - startX))));
		const finish = () => {
			document.body.classList.remove('is-resizing-panel');
			window.removeEventListener('pointermove', resize);
			window.removeEventListener('pointerup', finish);
		};
		document.body.classList.add('is-resizing-panel');
		window.addEventListener('pointermove', resize);
		window.addEventListener('pointerup', finish);
	};
	const renderDomain = (node: DomainNode): ReactNode => {
		const tiers = node.tiers
			.map((group) => ({ ...group, pages: node.landingPage ? group.pages.filter((page) => page !== node.landingPage) : group.pages }))
			.filter((group) => group.pages.length > 0);
		const collapsible = tiers.length > 0 || node.children.length > 0;
		const expanded = query.trim() !== '' || expandedDomains.has(node.path.toLowerCase());
		const toggleDomain = () => setExpandedDomains((current) => {
			const next = new Set(current);
			const key = node.path.toLowerCase();
			if (next.has(key)) next.delete(key); else next.add(key);
			return next;
		});
		const openOrToggleLanding = () => {
			if (query.trim() === '' && node.landingPage?.slug === selectedSlug) {
				toggleDomain();
				return;
			}
			onSelect(node.landingPage!.slug);
			if (!expanded) setExpandedDomains((current) => new Set(current).add(node.path.toLowerCase()));
		};
		const handleLandingKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
			if (event.key === 'Enter') {
				event.preventDefault();
				openOrToggleLanding();
				return;
			}
			moveFocus(event, node.landingPage!.slug);
		};
		const depth = node.path.includes('/') ? 'area' : 'bucket';
		const Marker = depth === 'bucket' ? Library : BookMarked;
		const renderPage = (page: KnowledgePage) => {
			const pageWarnings = warnings.filter((warning) => warning.slug === page.slug || warning.path === page.path).length;
			return <button data-knowledge-entry data-knowledge-slug={page.slug} className={page.slug === selectedSlug ? 'knowledge-page-row active' : 'knowledge-page-row'} key={page.slug} onClick={() => onSelect(page.slug)} onKeyDown={(event) => moveFocus(event, page.slug)}>
				<span><strong className="knowledge-page-title">{page.title}</strong><small><span className="knowledge-page-type">{displayPageType(page.pageType)}</span>{pageWarnings ? <span className="knowledge-page-warning">· {pageWarnings} warning{pageWarnings === 1 ? '' : 's'}</span> : null}</small></span>
			</button>;
		};
		return <section className={`knowledge-domain knowledge-domain-${depth}`} key={node.path}>
			<div className="knowledge-domain-header">
				<h3>{node.landingPage
					? <button data-knowledge-entry data-knowledge-slug={node.landingPage.slug} type="button" className={node.landingPage.slug === selectedSlug ? 'knowledge-domain-link active' : 'knowledge-domain-link'} onClick={openOrToggleLanding} onKeyDown={handleLandingKeyDown} aria-label={`Open ${node.path} index`}><Marker size={13} /><span>{node.name}</span></button>
					: <button data-knowledge-entry type="button" className="knowledge-domain-link knowledge-domain-link-toggle" aria-expanded={collapsible ? expanded : undefined} onClick={toggleDomain} onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); toggleDomain(); } }}><Marker size={13} /><span>{node.name}</span></button>}
				</h3>
				{collapsible && <button type="button" className={expanded ? 'knowledge-domain-toggle expanded' : 'knowledge-domain-toggle'} aria-label={`${expanded ? 'Collapse' : 'Expand'} ${node.path}`} aria-expanded={expanded} onClick={toggleDomain}><ChevronRight size={14} /></button>}
			</div>
			{expanded && tiers.map((group) => <div className="knowledge-tier" key={group.tier || '_'}>
				{group.tier && <p className="knowledge-tier-label" data-testid="knowledge-tier-label">{group.tier}</p>}
				{group.pages.map(renderPage)}
			</div>)}
			{expanded && node.children.length > 0 && <div className="knowledge-domain-children">{node.children.map(renderDomain)}</div>}
		</section>;
	};

	const browserStyle = sidePanel ? { '--knowledge-side-panel-width': `${sidePanelWidth}px` } as CSSProperties : undefined;
	return <div className={sidePanel ? `knowledge-browser with-side-panel${sidePanelCollapsed ? ' side-panel-collapsed' : ''}` : 'knowledge-browser'} style={browserStyle}>
		<div className="knowledge-browser-list">
			<label className="knowledge-search"><Search size={15} /><span className="knowledge-visually-hidden">Filter Knowledge pages</span><input aria-label="Filter Knowledge pages" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter pages" /></label>
			{pages.length === 0 && <div className="knowledge-empty"><h2>No valid pages indexed</h2><p>Add Markdown pages with <code>slug</code> and <code>title</code> front matter, then rescan.</p></div>}
			{pages.length > 0 && filtered.length === 0 && <p className="knowledge-empty">No pages match this filter.</p>}
			<nav ref={navigationRef} aria-label="Knowledge pages">{domains.map(renderDomain)}</nav>
			<KnowledgeWarnings warnings={warnings} compact indexDiagnostics />
		</div>
		<section className="knowledge-content-pane" aria-label="Knowledge page content">{children ?? <div className="knowledge-welcome"><BookOpen size={28} /><h2>Select a page</h2><p>Choose an entry from the index to read its full content.</p></div>}</section>
		{sidePanel && <aside className={sidePanelCollapsed ? 'metadata-panel side-panel knowledge-side-panel collapsed' : 'metadata-panel side-panel knowledge-side-panel'} aria-label="Knowledge side panel"><div className="panel-header"><h2><BookOpen size={16} /> Knowledge</h2><button className="icon-button" type="button" title={sidePanelCollapsed ? 'Expand Knowledge Quality' : 'Collapse Knowledge Quality'} onClick={() => setSidePanelCollapsed((current) => !current)}>{sidePanelCollapsed ? <PanelRightOpen size={16} /> : <PanelRightClose size={16} />}</button></div>{!sidePanelCollapsed && sidePanel}{!sidePanelCollapsed && <button className="panel-resize-handle panel-resize-handle-right" type="button" aria-label="Resize Knowledge Quality panel" onPointerDown={startSidePanelResize}><GripVertical size={16} /></button>}</aside>}
	</div>;
}

interface TierGroup {
	tier: string;
	pages: KnowledgePage[];
}

interface DomainNode {
	name: string;
	path: string;
	tiers: TierGroup[];
	landingPage?: KnowledgePage;
	children: DomainNode[];
}

function findLandingPage(pages: KnowledgePage[]): KnowledgePage | undefined {
	return pages.find((page) => /(?:^|\/)index\.md$/i.test(page.path)) ?? pages.find((page) => /(?:^|\/)readme\.md$/i.test(page.path));
}

// Pages are grouped bucket then area, and an area's pages are partitioned by
// tier. A tier is never a node: it has no landing page anywhere in a corpus, so
// as a node it could only be opened through its disclosure control.
function buildDomainTree(visiblePages: KnowledgePage[], allPages: KnowledgePage[]): DomainNode[] {
	const buckets = new Map<string, DomainNode>();
	const areas = new Map<string, DomainNode>();
	const order: string[] = [];
	const nodeFor = (key: string, name: string, into?: DomainNode[]): DomainNode => {
		const existing = areas.get(key) ?? buckets.get(key);
		if (existing) return existing;
		const node: DomainNode = { name, path: key, tiers: [], children: [] };
		(into ?? []).push(node);
		return node;
	};
	for (const page of visiblePages) {
		const grouping = groupingOf(page);
		const bucket = bucketKey(grouping);
		let bucketNode = buckets.get(bucket);
		if (!bucketNode) {
			bucketNode = { name: grouping.bucket || rootGroupKey, path: bucket, tiers: [], children: [] };
			buckets.set(bucket, bucketNode);
			order.push(bucket);
		}
		const area = areaKey(grouping);
		let target = bucketNode;
		if (area) {
			let areaNode = areas.get(area);
			if (!areaNode) {
				areaNode = nodeFor(area, grouping.area.split('/').filter(Boolean).join(' / '), bucketNode.children);
				areas.set(area, areaNode);
			}
			target = areaNode;
		}
		const tier = grouping.tier;
		const group = target.tiers.find((candidate) => candidate.tier === tier);
		if (group) group.pages.push(page);
		else target.tiers.push({ tier, pages: [page] });
	}
	for (const node of [...buckets.values(), ...areas.values()]) {
		node.landingPage = findLandingPage(allPages.filter((page) => nodeKeyOf(page) === node.path));
		node.tiers.sort((left, right) => left.tier.localeCompare(right.tier));
		node.children.sort((left, right) => left.name.localeCompare(right.name));
	}
	const roots = order.map((key) => buckets.get(key)!).sort((left, right) => left.name.localeCompare(right.name));
	const rootIndex = roots.findIndex((node) => node.path === rootGroupKey);
	if (rootIndex > 0) roots.unshift(...roots.splice(rootIndex, 1));
	return roots;
}

function nodeKeyOf(page: KnowledgePage): string {
	const grouping = groupingOf(page);
	return areaKey(grouping) ?? bucketKey(grouping);
}

function displayPageType(pageType?: string): string {
	return (pageType || 'PAGE').replaceAll('_', '-');
}
