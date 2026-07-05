import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { BookMarked, BookOpen, ChevronRight, Search } from 'lucide-react';
import type { KnowledgePage, KnowledgeWarning } from '../../lib/types';
import { KnowledgeWarnings } from './KnowledgeWarnings';

export function KnowledgeBrowser({ pages, selectedSlug, warnings, onSelect, children }: { pages: KnowledgePage[]; selectedSlug?: string; warnings: KnowledgeWarning[]; onSelect: (slug: string) => void; children?: ReactNode }) {
	const [query, setQuery] = useState('');
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
		const parts = (selectedPage.domain || 'root').split('/').filter(Boolean);
		setExpandedDomains((current) => {
			const next = new Set(current);
			for (let index = 0; index < parts.length; index++) next.add(parts.slice(0, index + 1).join('/').toLowerCase());
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
	const renderDomain = (node: DomainNode): ReactNode => {
		const childPages = node.landingPage ? node.pages.filter((page) => page !== node.landingPage) : node.pages;
		const collapsible = childPages.length > 0 || node.children.length > 0;
		const expanded = query.trim() !== '' || expandedDomains.has(node.path.toLowerCase());
		const toggleDomain = () => setExpandedDomains((current) => {
			const next = new Set(current);
			const key = node.path.toLowerCase();
			if (next.has(key)) next.delete(key); else next.add(key);
			return next;
		});
		return <section className="knowledge-domain" key={node.path}>
			<div className="knowledge-domain-header"><h3>{node.landingPage ? <button data-knowledge-entry data-knowledge-slug={node.landingPage.slug} type="button" className={node.landingPage.slug === selectedSlug ? 'knowledge-domain-link active' : 'knowledge-domain-link'} onClick={() => onSelect(node.landingPage!.slug)} onKeyDown={(event) => moveFocus(event, node.landingPage!.slug)} aria-label={`Open ${node.path} index`}><BookMarked size={13} /><span>{node.name}</span></button> : <span className="knowledge-domain-label"><BookMarked size={13} /><span>{node.name}</span></span>}</h3>{collapsible && <button type="button" className={expanded ? 'knowledge-domain-toggle expanded' : 'knowledge-domain-toggle'} aria-label={`${expanded ? 'Collapse' : 'Expand'} ${node.path}`} aria-expanded={expanded} onClick={toggleDomain}><ChevronRight size={14} /></button>}</div>
			{expanded && childPages.map((page) => { const pageWarnings = warnings.filter((warning) => warning.slug === page.slug || warning.path === page.path).length; return <button data-knowledge-entry data-knowledge-slug={page.slug} className={page.slug === selectedSlug ? 'knowledge-page-row active' : 'knowledge-page-row'} key={page.slug} onClick={() => onSelect(page.slug)} onKeyDown={(event) => moveFocus(event, page.slug)}><span><strong className="knowledge-page-title">{page.title}</strong><small><span className="knowledge-page-type">{displayPageType(page.pageType)}</span>{pageWarnings ? <span className="knowledge-page-warning">· {pageWarnings} warning{pageWarnings === 1 ? '' : 's'}</span> : null}</small></span></button>; })}
			{expanded && node.children.length > 0 && <div className="knowledge-domain-children">{node.children.map(renderDomain)}</div>}
		</section>;
	};

	return <div className="knowledge-browser">
		<div className="knowledge-browser-list">
			<label className="knowledge-search"><Search size={15} /><span className="knowledge-visually-hidden">Filter Knowledge pages</span><input aria-label="Filter Knowledge pages" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter pages" /></label>
			{pages.length === 0 && <div className="knowledge-empty"><h2>No valid pages indexed</h2><p>Add Markdown pages with <code>slug</code> and <code>title</code> front matter, then rescan.</p></div>}
			{pages.length > 0 && filtered.length === 0 && <p className="knowledge-empty">No pages match this filter.</p>}
			<nav ref={navigationRef} aria-label="Knowledge pages">{domains.map(renderDomain)}</nav>
			<KnowledgeWarnings warnings={warnings} compact indexDiagnostics />
		</div>
		<section className="knowledge-content-pane" aria-label="Knowledge page content">{children ?? <div className="knowledge-welcome"><BookOpen size={28} /><h2>Select a page</h2><p>Choose an entry from the index to read its full content.</p></div>}</section>
	</div>;
}

interface DomainNode {
	name: string;
	path: string;
	pages: KnowledgePage[];
	landingPage?: KnowledgePage;
	children: DomainNode[];
}

function findLandingPage(pages: KnowledgePage[]): KnowledgePage | undefined {
	return pages.find((page) => /(?:^|\/)index\.md$/i.test(page.path)) ?? pages.find((page) => /(?:^|\/)readme\.md$/i.test(page.path));
}

function buildDomainTree(visiblePages: KnowledgePage[], allPages: KnowledgePage[]): DomainNode[] {
	const roots: DomainNode[] = [];
	const nodes = new Map<string, DomainNode>();
	for (const page of visiblePages) {
		const parts = (page.domain || 'root').split('/').filter(Boolean);
		let siblings = roots;
		for (let index = 0; index < parts.length; index++) {
			const path = parts.slice(0, index + 1).join('/');
			let node = nodes.get(path);
			if (!node) {
				node = { name: parts[index], path, pages: [], children: [] };
				nodes.set(path, node);
				siblings.push(node);
			}
			siblings = node.children;
		}
		nodes.get(parts.join('/'))!.pages.push(page);
	}
	for (const node of nodes.values()) node.landingPage = findLandingPage(allPages.filter((page) => (page.domain || 'root') === node.path));
	const rootIndex = roots.findIndex((node) => node.path.toLowerCase() === 'root');
	if (rootIndex > 0) roots.unshift(...roots.splice(rootIndex, 1));
	return roots;
}

function displayPageType(pageType?: string): string {
	return (pageType || 'PAGE').replaceAll('_', '-');
}
