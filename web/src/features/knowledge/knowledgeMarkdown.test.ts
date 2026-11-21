import { describe, expect, it } from 'vitest';
import source from './knowledgeMarkdown.ts?raw';
import { knowledgeLinkPrefix, normalizeKnowledgeTarget, prepareKnowledgeMarkdown } from './knowledgeMarkdown';

describe('prepareKnowledgeMarkdown', () => {
	it('removes the front matter', () => {
		expect(prepareKnowledgeMarkdown('---\nslug: x\ntitle: X\n---\n# X\nBody')).toBe('# X\nBody');
		expect(prepareKnowledgeMarkdown('---\r\nslug: x\r\n---\r\n# X')).toBe('# X');
	});

	it('leaves a file without front matter unchanged apart from links', () => {
		expect(prepareKnowledgeMarkdown('# X\n\n---\nBody')).toBe('# X\n\n---\nBody');
		expect(prepareKnowledgeMarkdown('# X\nSee [[page]].')).toBe('# X\nSee [page](./__knowledge__/page).');
	});

	it('rewrites wiki links with and without a label', () => {
		expect(prepareKnowledgeMarkdown('[[page]]')).toBe('[page](./__knowledge__/page)');
		expect(prepareKnowledgeMarkdown('[[page|The page]]')).toBe('[The page](./__knowledge__/page)');
		expect(prepareKnowledgeMarkdown('[[ a b ]]')).toBe('[ a b ](./__knowledge__/a%20b)');
		expect(knowledgeLinkPrefix).toBe('./__knowledge__/');
	});

	it('does not convert Wiki syntax inside code', () => {
		const markdown = prepareKnowledgeMarkdown('---\nslug: x\ntitle: X\n---\n`[[inline]]`\n```\n[[fenced]]\n```\n[[page]]');
		expect(markdown).toContain('`[[inline]]`');
		expect(markdown).toContain('[[fenced]]');
		expect(markdown).toContain('[page](./__knowledge__/page)');
		expect(prepareKnowledgeMarkdown('~~~\n[[fenced]]\n~~~')).toBe('~~~\n[[fenced]]\n~~~');
	});
});

describe('normalizeKnowledgeTarget', () => {
	it('turns a knowledge link back into its slug', () => {
		expect(normalizeKnowledgeTarget('./__knowledge__/a%20b#x')).toBe('a b');
		expect(normalizeKnowledgeTarget(' target ')).toBe('target');
	});

	it('keeps a target ending in .md whole', () => {
		expect(normalizeKnowledgeTarget('./__knowledge__/ dir/page.md')).toBe(' dir/page.md');
	});
});

describe('knowledgeMarkdown module', () => {
	it('imports nothing', () => {
		expect(source.split('\n').filter((line) => line.trimStart().startsWith('import'))).toEqual([]);
	});
});
