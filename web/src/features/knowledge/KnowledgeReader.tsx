import type { MouseEvent } from 'react';
import { ExternalLink, FolderOpen } from 'lucide-react';
import { ContentViewer } from '../content-viewer/ContentViewer';
import type { KnowledgePageDetail } from '../../lib/types';
import { KnowledgeWarnings } from './KnowledgeWarnings';

export function KnowledgeReader({ detail, onNavigate, onOpenExplorer }: { detail: KnowledgePageDetail; onNavigate: (slug: string) => void; onOpenExplorer: () => void }) {
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
		<article className="knowledge-reader-content" onClick={interceptLink}><ContentViewer file={detail.content} content={prepareKnowledgeMarkdown(detail.content.content)} /></article>
		<aside className="knowledge-reader-meta"><p className="eyebrow">{detail.domain} · {detail.pageType || 'PAGE'}</p><h2>{detail.title}</h2><p>{detail.summary || 'No summary provided.'}</p><button className="secondary" onClick={onOpenExplorer}><FolderOpen size={15} /> Open in Explorer</button>
			<Metadata title="Roles" values={detail.roles} /><Metadata title="Topics" values={detail.topics} /><Metadata title="Source references" values={detail.sourceRefs} />
			<section><h3>Outgoing links</h3>{detail.links.length ? <ul>{detail.links.map((link, index) => <li key={`${link.rawTarget}-${index}`}>{link.resolution === 'resolved' && link.targetSlug ? <button className="link-button" onClick={() => onNavigate(link.targetSlug!)}>{link.label || link.targetSlug}</button> : <span className="knowledge-unresolved">{link.label || link.rawTarget} (unresolved)</span>}</li>)}</ul> : <p>None</p>}</section>
			<section><h3>Backlinks</h3>{detail.backlinks.length ? <ul>{detail.backlinks.map((slug) => <li key={slug}><button className="link-button" onClick={() => onNavigate(slug)}>{slug}</button></li>)}</ul> : <p>None</p>}</section>
			<KnowledgeWarnings warnings={detail.warnings} />
			{detail.links.some((link) => /^https?:/i.test(link.rawTarget)) && <p><ExternalLink size={14} /> External links open in a new tab.</p>}
		</aside>
	</div>;
}

function Metadata({ title, values }: { title: string; values: string[] }) {
	return <section><h3>{title}</h3>{values.length ? <ul>{values.map((value) => <li key={value}>{value}</li>)}</ul> : <p>None</p>}</section>;
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
