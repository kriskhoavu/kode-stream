import { useEffect, useState } from 'react';
import rehypeExternalLinks from 'rehype-external-links';
import rehypeHighlight from 'rehype-highlight';
import rehypeKatex from 'rehype-katex';
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize';
import type { Options as SanitizeSchema } from 'rehype-sanitize';
import rehypeStringify from 'rehype-stringify';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import remarkParse from 'remark-parse';
import remarkRehype from 'remark-rehype';
import { unified } from 'unified';

const markdownSchema: SanitizeSchema = {
  ...defaultSchema,
  attributes: {
    ...defaultSchema.attributes,
    code: [
      ...(defaultSchema.attributes?.code ?? []),
      ['className', /^language-./, 'math-inline', 'math-display']
    ],
    span: [
      ...(defaultSchema.attributes?.span ?? []),
      ['className', 'math', 'math-inline', 'math-display']
    ]
  }
};

export async function renderMarkdown(content: string): Promise<string> {
  const result = await unified()
    .use(remarkParse)
    .use(remarkGfm)
    .use(remarkMath)
    .use(remarkRehype)
    .use(rehypeSanitize, markdownSchema)
    .use(rehypeKatex)
    .use(rehypeHighlight, { detect: false })
    .use(rehypeExternalLinks, { rel: ['noreferrer', 'noopener'], target: '_blank' })
    .use(rehypeStringify)
    .process(content);
  return String(result);
}

export function MarkdownPreview({ content }: { content: string }) {
  const [html, setHtml] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    setError('');
    void renderMarkdown(content).then((rendered) => {
      if (active) setHtml(rendered);
    }).catch(() => {
      if (active) setError('Markdown preview could not be rendered.');
    });
    return () => {
      active = false;
    };
  }, [content]);

  if (error) return <p className="viewer-error">{error}</p>;
  return <article className="content-viewer-markdown" dangerouslySetInnerHTML={{ __html: html }} />;
}
