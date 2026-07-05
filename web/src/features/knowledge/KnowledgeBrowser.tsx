import { useMemo, useRef, useState, type ReactNode } from 'react';
import { BookOpen, ChevronRight, Search } from 'lucide-react';
import type { KnowledgePage, KnowledgeWarning } from '../../lib/types';
import { KnowledgeWarnings } from './KnowledgeWarnings';

export function KnowledgeBrowser({ pages, selectedSlug, warnings, onSelect, children }: { pages: KnowledgePage[]; selectedSlug?: string; warnings: KnowledgeWarning[]; onSelect: (slug: string) => void; children?: ReactNode }) {
	const [query, setQuery] = useState('');
	const buttons = useRef<Array<HTMLButtonElement | null>>([]);
	const filtered = useMemo(() => {
		const needle = query.trim().toLowerCase();
		return needle ? pages.filter((page) => [page.title, page.slug, page.summary ?? '', ...page.roles, ...page.topics].some((value) => value.toLowerCase().includes(needle))) : pages;
	}, [pages, query]);
	const domains = useMemo(() => groupByDomain(filtered), [filtered]);
	let buttonIndex = 0;
	const moveFocus = (event: React.KeyboardEvent, index: number, slug: string) => {
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); const next = Math.max(0, Math.min(filtered.length - 1, index + (event.key === 'ArrowDown' ? 1 : -1))); buttons.current[next]?.focus(); }
		if (event.key === 'Enter') { event.preventDefault(); onSelect(slug); }
	};

	return <div className="knowledge-browser">
		<div className="knowledge-browser-list">
			<label className="knowledge-search"><Search size={15} /><span className="knowledge-visually-hidden">Filter Knowledge pages</span><input aria-label="Filter Knowledge pages" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter pages" /></label>
			{pages.length === 0 && <div className="knowledge-empty"><h2>No valid pages indexed</h2><p>Add Markdown pages with <code>slug</code> and <code>title</code> front matter, then rescan.</p></div>}
			{pages.length > 0 && filtered.length === 0 && <p className="knowledge-empty">No pages match this filter.</p>}
			<nav aria-label="Knowledge pages">{Array.from(domains.entries()).map(([domain, domainPages]) => {
				const landingPage = findLandingPage(domainPages);
				const childPages = landingPage ? domainPages.filter((page) => page !== landingPage) : domainPages;
				const landingIndex = landingPage ? buttonIndex++ : -1;
				return <section className="knowledge-domain" key={domain}><h3>{landingPage ? <button ref={(node) => { buttons.current[landingIndex] = node; }} type="button" className={landingPage.slug === selectedSlug ? 'knowledge-domain-link active' : 'knowledge-domain-link'} onClick={() => onSelect(landingPage.slug)} onKeyDown={(event) => moveFocus(event, landingIndex, landingPage.slug)} aria-label={`Open ${domain} index`}><span>{domain}</span><ChevronRight size={13} /></button> : domain}</h3>{childPages.map((page) => { const index = buttonIndex++; const pageWarnings = warnings.filter((warning) => warning.slug === page.slug || warning.path === page.path).length; return <button ref={(node) => { buttons.current[index] = node; }} className={page.slug === selectedSlug ? 'knowledge-page-row active' : 'knowledge-page-row'} key={page.slug} onClick={() => onSelect(page.slug)} onKeyDown={(event) => moveFocus(event, index, page.slug)}><BookOpen size={15} /><span><strong>{page.title}</strong><small>{page.pageType || 'PAGE'}{pageWarnings ? ` · ${pageWarnings} warning${pageWarnings === 1 ? '' : 's'}` : ''}</small></span></button>; })}</section>;
			})}</nav>
			<KnowledgeWarnings warnings={warnings} compact indexDiagnostics />
		</div>
		<section className="knowledge-content-pane" aria-label="Knowledge page content">{children ?? <div className="knowledge-welcome"><BookOpen size={28} /><h2>Select a page</h2><p>Choose an entry from the index to read its full content.</p></div>}</section>
	</div>;
}

function findLandingPage(pages: KnowledgePage[]): KnowledgePage | undefined {
	return pages.find((page) => /(?:^|\/)index\.md$/i.test(page.path)) ?? pages.find((page) => /(?:^|\/)readme\.md$/i.test(page.path));
}

function groupByDomain(pages: KnowledgePage[]): Map<string, KnowledgePage[]> {
	const groups = new Map<string, KnowledgePage[]>();
	for (const page of pages) {
		const domain = page.domain || 'root';
		groups.set(domain, [...(groups.get(domain) ?? []), page]);
	}
	return groups;
}
