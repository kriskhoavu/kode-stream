import type { MouseEvent } from 'react';
import { ContentViewer } from '../content-viewer/ContentViewer';
import type { KnowledgePageDetail } from '../../lib/types';
import { KnowledgeWarnings } from './KnowledgeWarnings';
import { normalizeKnowledgeTarget, prepareKnowledgeMarkdown } from './knowledgeMarkdown';

export { prepareKnowledgeMarkdown };

// Front matter is index data, so the reader shows only the body and warnings.
export function KnowledgeReader({ detail, onNavigate }: { detail: KnowledgePageDetail; onNavigate: (slug: string) => void }) {
	const resolvedTargets = new Map(detail.links.filter((link) => link.resolution === 'resolved' && link.targetSlug).map((link) => [normalizeKnowledgeTarget(link.rawTarget), link.targetSlug!]));
	const interceptLink = (event: MouseEvent<HTMLDivElement>) => {
		const anchor = (event.target as HTMLElement).closest<HTMLAnchorElement>('a');
		if (!anchor) return;
		const raw = anchor.getAttribute('href') ?? '';
		if (/^(https?:|mailto:)/i.test(raw)) return;
		const target = resolvedTargets.get(normalizeKnowledgeTarget(raw));
		if (!target) { event.preventDefault(); return; }
		event.preventDefault(); onNavigate(target);
	};
	return <div className="knowledge-reader">
		{detail.warnings.length > 0 && <section className="knowledge-reader-warnings"><KnowledgeWarnings warnings={detail.warnings} /></section>}
		<article className="knowledge-reader-content" onClick={interceptLink}><ContentViewer file={detail.content} content={prepareKnowledgeMarkdown(detail.content.content)} /></article>
	</div>;
}
