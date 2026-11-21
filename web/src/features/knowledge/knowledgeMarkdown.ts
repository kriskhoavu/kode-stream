// Prepares a wiki page for MarkdownPreview. This module imports nothing, so
// another application can copy it verbatim beside MarkdownPreview.tsx and read
// a page exactly as the Knowledge page reads it.

export const knowledgeLinkPrefix = './__knowledge__/';

// Drops the front matter and turns [[slug]] and [[slug|label]] into links,
// leaving inline and fenced code untouched.
export function prepareKnowledgeMarkdown(content: string): string {
	let body = content.replace(/^---\r?\n[\s\S]*?\r?\n---\r?\n/, '');
	let fenced = false;
	body = body.split('\n').map((line) => {
		if (/^\s*(```|~~~)/.test(line)) { fenced = !fenced; return line; }
		if (fenced) return line;
		const segments = line.split(/(`+[^`]*`+)/g);
		return segments.map((segment, index) => index % 2 ? segment : segment.replace(/\[\[([^\]|]+)(?:\|([^\]]+))?\]\]/g, (_, slug: string, label?: string) => `[${label || slug}](${knowledgeLinkPrefix}${encodeURIComponent(slug.trim())})`)).join('');
	}).join('\n');
	return body;
}

// Turns a clicked link's href back into the slug it names.
export function normalizeKnowledgeTarget(target: string): string {
	const decoded = decodeURIComponent(target);
	const cleaned = (decoded.startsWith(knowledgeLinkPrefix) ? decoded.slice(knowledgeLinkPrefix.length) : decoded).split('#')[0];
	return cleaned.endsWith('.md') ? cleaned : cleaned.trim();
}
