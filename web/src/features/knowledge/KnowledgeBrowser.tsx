import { useMemo, useRef, useState } from 'react';
import { BookOpen, Search } from 'lucide-react';
import type { KnowledgePage, KnowledgeWarning } from '../../lib/types';
import { KnowledgeWarnings } from './KnowledgeWarnings';

export function KnowledgeBrowser({ pages, selectedSlug, warnings, onSelect, onOpen }: { pages: KnowledgePage[]; selectedSlug?: string; warnings: KnowledgeWarning[]; onSelect: (slug: string) => void; onOpen: (slug: string) => void }) {
	const [query, setQuery] = useState('');
	const buttons = useRef<Array<HTMLButtonElement | null>>([]);
	const filtered = useMemo(() => {
		const needle = query.trim().toLowerCase();
		return needle ? pages.filter((page) => [page.title, page.slug, page.summary ?? '', ...page.roles, ...page.topics].some((value) => value.toLowerCase().includes(needle))) : pages;
	}, [pages, query]);
	const domains = useMemo(() => groupByDomain(filtered), [filtered]);
	const selected = pages.find((page) => page.slug === selectedSlug);
	let buttonIndex = 0;
	const moveFocus = (event: React.KeyboardEvent, index: number, slug: string) => {
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); const next = Math.max(0, Math.min(filtered.length - 1, index + (event.key === 'ArrowDown' ? 1 : -1))); buttons.current[next]?.focus(); }
		if (event.key === 'Enter') { event.preventDefault(); onOpen(slug); }
	};

	return <div className="knowledge-browser">
		<div className="knowledge-browser-list">
			<label className="knowledge-search"><Search size={15} /><span className="sr-only">Filter Knowledge pages</span><input aria-label="Filter Knowledge pages" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter title, topic, role..." /></label>
			{pages.length === 0 && <div className="knowledge-empty"><h2>No valid pages indexed</h2><p>Add Markdown pages with <code>slug</code> and <code>title</code> front matter, then rescan.</p></div>}
			{pages.length > 0 && filtered.length === 0 && <p className="knowledge-empty">No pages match this filter.</p>}
			<nav aria-label="Knowledge pages">{Array.from(domains.entries()).map(([domain, domainPages]) => <section className="knowledge-domain" key={domain}><h3>{domain}</h3>{domainPages.map((page) => { const index = buttonIndex++; const pageWarnings = warnings.filter((warning) => warning.slug === page.slug || warning.path === page.path).length; return <button ref={(node) => { buttons.current[index] = node; }} className={page.slug === selectedSlug ? 'knowledge-page-row active' : 'knowledge-page-row'} key={page.slug} onClick={() => onSelect(page.slug)} onDoubleClick={() => onOpen(page.slug)} onKeyDown={(event) => moveFocus(event, index, page.slug)}><BookOpen size={15} /><span><strong>{page.title}</strong><small>{page.pageType || 'PAGE'}{pageWarnings ? ` · ${pageWarnings} warning${pageWarnings === 1 ? '' : 's'}` : ''}</small></span></button>; })}</section>)}</nav>
		</div>
		<section className="knowledge-summary" aria-label="Selected page summary">{selected ? <><p className="eyebrow">{selected.domain} · {selected.pageType || 'PAGE'}</p><h2>{selected.title}</h2><p>{selected.summary || 'No summary provided.'}</p><div className="knowledge-tags">{selected.roles.map((role) => <span key={`role-${role}`}>{role}</span>)}{selected.topics.map((topic) => <span key={`topic-${topic}`}>{topic}</span>)}</div><button className="primary-button" onClick={() => onOpen(selected.slug)}>Read page</button></> : <><h2>Select a page</h2><p>Choose a page to inspect its summary and metadata.</p></>}<KnowledgeWarnings warnings={warnings} /></section>
	</div>;
}

function groupByDomain(pages: KnowledgePage[]): Map<string, KnowledgePage[]> {
	const groups = new Map<string, KnowledgePage[]>();
	for (const page of pages) {
		const domain = page.domain || 'root';
		groups.set(domain, [...(groups.get(domain) ?? []), page]);
	}
	return groups;
}
