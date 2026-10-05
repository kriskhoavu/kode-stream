import type { MouseEvent } from 'react';
import { ContentViewer } from '../content-viewer/ContentViewer';
import type { KnowledgePageDetail } from '../../lib/types';
import { KnowledgeWarnings } from './KnowledgeWarnings';

// The page's front matter (roles, topics, sources, links) is index data, not
// reading matter, so the reader shows only the page body and any warnings.
export function KnowledgeReader({ detail, onNavigate }: { detail: KnowledgePageDetail; onNavigate: (slug: string) => void }) {
	const resolvedTargets = new Map(detail.links.filter((link) => link.resolution === 'resolved' && link.targetSlug).map((link) => [normalizeTarget(link.rawTarget), link.targetSlug!]));
	const interceptLink = (event: MouseEvent<HTMLDivElement>) => {
		const anchor = (event.target as HTMLElement).closest<HTMLAnchorElement>('a');
		if (!anchor) return;
		const raw = anchor.getAttribute('href') ?? '';
		if (/^(https?:|mailto:)/i.test(raw)) return;
		const target = resolvedTargets.get(normalizeTarget(raw));
		if (!target) { event.preventDefault(); return; }
		event.preventDefault(); onNavigate(target);
	};
	return <div className="knowledge-reader">
		{detail.warnings.length > 0 && <section className="knowledge-reader-warnings"><KnowledgeWarnings warnings={detail.warnings} /></section>}
		<article className="knowledge-reader-content" onClick={interceptLink}><ContentViewer file={detail.content} content={prepareKnowledgeMarkdown(detail.content.content)} /></article>
	</div>;
}

export function prepareKnowledgeMarkdown(content: string): string {
	let body = content.replace(/^---\r?\n[\s\S]*?\r?\n---\r?\n/, '');
	let fenced = false;
	body = body.split('\n').map((line) => {
		if (/^\s*(```|~~~)/.test(line)) { fenced = !fenced; return line; }
		if (fenced) return line;
		const segments = line.split(/(`+[^`]*`+)/g);
		return segments.map((segment, index) => index % 2 ? segment : segment.replace(/\[\[([^\]|]+)(?:\|([^\]]+))?\]\]/g, (_, slug: string, label?: string) => `[${label || slug}](./__knowledge__/${encodeURIComponent(slug.trim())})`)).join('');
	}).join('\n');
	return body;
}

function normalizeTarget(target: string): string {
	const cleaned = decodeURIComponent(target).replace(/^\.\/__knowledge__\//, '').split('#')[0];
	return cleaned.endsWith('.md') ? cleaned : cleaned.trim();
}
