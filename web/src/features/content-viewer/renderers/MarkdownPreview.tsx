import { useEffect, useRef, useState } from 'react';
import type { MouseEvent } from 'react';
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
import 'katex/dist/katex.min.css';

const markdownCache = new Map<string, string>();
const maxMarkdownCacheChars = 2 << 20;
const maxMarkdownCacheEntryChars = 512 << 10;
let markdownCacheChars = 0;

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
  const cached = markdownCache.get(content);
  if (cached !== undefined) return cached;
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
  const html = decorateCodeBlocks(String(result));
  const entryChars = content.length + html.length;
  if (entryChars <= maxMarkdownCacheEntryChars) {
    const previous = markdownCache.get(content);
    if (previous !== undefined) {
      markdownCacheChars -= content.length + previous.length;
      markdownCache.delete(content);
    }
    markdownCache.set(content, html);
    markdownCacheChars += entryChars;
    while (markdownCache.size > 20 || markdownCacheChars > maxMarkdownCacheChars) {
      const oldest = markdownCache.keys().next().value;
      if (oldest === undefined) break;
      markdownCacheChars -= oldest.length + markdownCache.get(oldest)!.length;
      markdownCache.delete(oldest);
    }
  }
  return html;
}

export function MarkdownPreview({ content }: { content: string }) {
  const [view, setView] = useState<{ source: string; html: string; error: string }>();
	const copyTimers = useRef(new Set<number>());

	useEffect(() => () => {
		for (const timer of copyTimers.current) window.clearTimeout(timer);
		copyTimers.current.clear();
	}, []);

  useEffect(() => {
    let active = true;
    void renderMarkdown(content).then((rendered) => {
      if (active) setView({ source: content, html: rendered, error: '' });
    }).catch(() => {
      if (active) setView({ source: content, html: '', error: 'Markdown preview could not be rendered.' });
    });
    return () => {
      active = false;
    };
  }, [content]);

  const copyCode = (event: MouseEvent<HTMLElement>) => {
    const button = (event.target as HTMLElement).closest<HTMLButtonElement>('.markdown-code-copy');
    if (!button) return;
    void navigator.clipboard.writeText(button.dataset.code ?? '');
    button.textContent = 'Copied';
		const timer = window.setTimeout(() => {
			copyTimers.current.delete(timer);
      button.textContent = 'Copy';
    }, 1200);
		copyTimers.current.add(timer);
  };

  if (view?.source !== content) return <div className="viewer-loading">Rendering preview...</div>;
  if (view.error) return <p className="viewer-error">{view.error}</p>;
  return <article className="content-viewer-markdown" onClick={copyCode} dangerouslySetInnerHTML={{ __html: view.html }} />;
}

function decorateCodeBlocks(html: string): string {
  const document = new DOMParser().parseFromString(`<body>${html}</body>`, 'text/html');
  for (const block of document.querySelectorAll('pre')) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'markdown-code-copy';
    button.setAttribute('aria-label', 'Copy code block');
    button.textContent = 'Copy';
    button.dataset.code = block.querySelector('code')?.textContent ?? block.textContent ?? '';
    block.prepend(button);
  }
  return document.body.innerHTML;
}
