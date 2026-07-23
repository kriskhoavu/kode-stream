import type { MouseEvent } from 'react';
import { ExternalLink } from 'lucide-react';
import { ContentViewer } from '../content-viewer/ContentViewer';
import type { KnowledgePageDetail } from '../../lib/types';
import { KnowledgeWarnings } from './KnowledgeWarnings';

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
		<section className="knowledge-reader-summary"><p className="eyebrow">{detail.domain} · {detail.pageType || 'PAGE'}</p><h2>{detail.title}</h2><p>{detail.summary || 'No summary provided.'}</p>
			<div className="knowledge-reader-metadata"><Metadata title="Roles" values={detail.roles} compact /><Metadata title="Topics" values={detail.topics} compact /><Metadata title="Source references" values={detail.sourceRefs} compact /><Metadata title="Outgoing links" values={detail.links.map((link) => link.resolution === 'resolved' && link.targetSlug ? (link.label || link.targetSlug) : `${link.label || link.rawTarget} (unresolved)`)} compact interactiveValues={detail.links.map((link) => link.resolution === 'resolved' && link.targetSlug ? { label: link.label || link.targetSlug, onClick: () => onNavigate(link.targetSlug!) } : undefined)} /><Metadata title="Backlinks" values={detail.backlinks} compact interactiveValues={detail.backlinks.map((slug) => ({ label: slug, onClick: () => onNavigate(slug) }))} /></div>
			<KnowledgeWarnings warnings={detail.warnings} />
			{detail.links.some((link) => /^https?:/i.test(link.rawTarget)) && <p className="knowledge-reader-note"><ExternalLink size={14} /> External links open in a new tab.</p>}
		</section>
		<article className="knowledge-reader-content" onClick={interceptLink}><ContentViewer file={detail.content} content={prepareKnowledgeMarkdown(detail.content.content)} /></article>
	</div>;
}

function Metadata({ title, values, compact = false, interactiveValues }: { title: string; values: string[]; compact?: boolean; interactiveValues?: Array<{ label: string; onClick: () => void } | undefined> }) {
	return <section className={compact ? 'knowledge-reader-meta-block compact' : 'knowledge-reader-meta-block'}><h3>{title}</h3>{values.length ? <ul>{values.map((value, index) => { const interactive = interactiveValues?.[index]; return <li key={`${title}-${value}-${index}`}>{interactive ? <button className="link-button" onClick={interactive.onClick}>{interactive.label}</button> : value}</li>; })}</ul> : <p>None</p>}</section>;
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
