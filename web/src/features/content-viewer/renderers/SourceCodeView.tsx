import { useEffect, useMemo, useRef, useState } from 'react';
import { Check, Copy, ListOrdered, WrapText } from 'lucide-react';
import hljs from 'highlight.js/lib/core';
import bash from 'highlight.js/lib/languages/bash';
import c from 'highlight.js/lib/languages/c';
import cpp from 'highlight.js/lib/languages/cpp';
import csharp from 'highlight.js/lib/languages/csharp';
import css from 'highlight.js/lib/languages/css';
import dockerfile from 'highlight.js/lib/languages/dockerfile';
import go from 'highlight.js/lib/languages/go';
import java from 'highlight.js/lib/languages/java';
import javascript from 'highlight.js/lib/languages/javascript';
import json from 'highlight.js/lib/languages/json';
import kotlin from 'highlight.js/lib/languages/kotlin';
import makefile from 'highlight.js/lib/languages/makefile';
import python from 'highlight.js/lib/languages/python';
import ruby from 'highlight.js/lib/languages/ruby';
import rust from 'highlight.js/lib/languages/rust';
import sql from 'highlight.js/lib/languages/sql';
import typescript from 'highlight.js/lib/languages/typescript';
import xml from 'highlight.js/lib/languages/xml';
import yaml from 'highlight.js/lib/languages/yaml';
import { richPreviewThresholdBytes } from '../types';
import type { ContentSearchSelection } from '../../../lib/types';

const languages = { bash, c, cpp, csharp, css, dockerfile, go, java, javascript, json, kotlin, makefile, python, ruby, rust, sql, typescript, xml, yaml };
for (const [name, definition] of Object.entries(languages)) hljs.registerLanguage(name, definition);
const languageAliases: Record<string, string> = { shell: 'bash', jsx: 'javascript', tsx: 'typescript', html: 'xml' };
const estimatedLineHeight = 20;
const renderedLineWindow = 240;

export function SourceCodeView({ content, language, truncated = false, selection }: { content: string; language: string; truncated?: boolean; selection?: ContentSearchSelection | null }) {
  const [wrap, setWrap] = useState(false);
  const [lineNumbers, setLineNumbers] = useState(true);
  const [copied, setCopied] = useState(false);
  const rich = new TextEncoder().encode(content).length <= richPreviewThresholdBytes;
  const lines = useMemo(() => content.split('\n'), [content]);
  const scrollRef = useRef<HTMLPreElement>(null);
  const [windowStart, setWindowStart] = useState(0);
	const [rowHeights, setRowHeights] = useState<Record<number, number>>({});
  const selectedLine = selection?.lineNumber ?? 0;
  const windowEnd = Math.min(lines.length, windowStart + renderedLineWindow);
	const offsets = useMemo(() => {
		const values = new Array<number>(lines.length + 1); values[0] = 0;
		for (let index = 0; index < lines.length; index++) values[index + 1] = values[index] + (rowHeights[index] ?? estimatedLineHeight);
		return values;
	}, [lines.length, rowHeights]);

	useEffect(() => {
		const invalidateMeasurements = () => setRowHeights({});
		window.addEventListener('resize', invalidateMeasurements);
		const target = scrollRef.current;
		if (target && typeof ResizeObserver !== 'undefined') {
			const observer = new ResizeObserver(invalidateMeasurements);
			observer.observe(target);
			return () => { window.removeEventListener('resize', invalidateMeasurements); observer.disconnect(); };
		}
		return () => window.removeEventListener('resize', invalidateMeasurements);
	}, [content, wrap]);

  useEffect(() => {
    if (!selectedLine || selectedLine > lines.length) return;
    const start = Math.max(0, Math.min(Math.max(0, lines.length - renderedLineWindow), selectedLine - Math.floor(renderedLineWindow / 2) - 1));
    setWindowStart(start);
    requestAnimationFrame(() => { if (scrollRef.current) scrollRef.current.scrollTop = Math.max(0, offsets[selectedLine - 1] - scrollRef.current.clientHeight / 2); });
  }, [lines.length, offsets, selectedLine]);

  const updateWindow = () => {
    const top = scrollRef.current?.scrollTop ?? 0;
		let low = 0, high = lines.length;
		while (low < high) { const middle = Math.floor((low + high) / 2); if (offsets[middle + 1] <= top) low = middle + 1; else high = middle; }
    const start = Math.max(0, Math.min(Math.max(0, lines.length - renderedLineWindow), low - 40));
    setWindowStart((current) => current === start ? current : start);
  };
	const measure = (index: number, node: HTMLSpanElement | null) => {
		if (!node) return;
		const height = node.getBoundingClientRect().height;
		if (height > 0) setRowHeights((current) => current[index] === height ? current : { ...current, [index]: height });
	};
  const copy = async () => { await navigator.clipboard.writeText(content); setCopied(true); window.setTimeout(() => setCopied(false), 1200); };

  return <div className={`source-code-view ${wrap ? 'wrap' : ''}`}>
    <div className="viewer-toolbar source-toolbar" aria-label="Source controls">
      {!rich && <span className="viewer-notice">Highlighting paused for this large file.</span>}
      {truncated && <span className="viewer-notice">Showing the first part of this file.</span>}
      <span className="viewer-toolbar-spacer" />
      <button type="button" className={lineNumbers ? 'active' : ''} title="Toggle line numbers" aria-label="Toggle line numbers" aria-pressed={lineNumbers} onClick={() => setLineNumbers((current) => !current)}><ListOrdered size={15} /></button>
      <button type="button" className={wrap ? 'active' : ''} title="Toggle line wrapping" aria-label="Toggle line wrapping" aria-pressed={wrap} onClick={() => setWrap((current) => !current)}><WrapText size={15} /></button>
      <button type="button" title="Copy source" aria-label="Copy source" onClick={() => void copy()}>{copied ? <Check size={15} /> : <Copy size={15} />}</button>
      <span className="sr-only" aria-live="polite">{copied ? 'Source copied' : ''}</span>
    </div>
    <pre ref={scrollRef} onScroll={updateWindow} className="source-code-scroll" data-language={language}><code style={{ height: `${offsets[lines.length]}px`, position: 'relative' }}><div style={{ position: 'absolute', top: `${offsets[windowStart]}px`, left: 0, right: 0 }}>
      {lines.slice(windowStart, windowEnd).map((line, index) => {
        const lineIndex = windowStart + index;
        const selected = lineIndex + 1 === selectedLine;
        return <span ref={(node) => measure(lineIndex, node)} className={`source-code-line${selected ? ' selected' : ''}`} key={lineIndex} aria-current={selected ? 'true' : undefined}>
          {lineNumbers && <span className="source-line-number" aria-hidden="true">{lineIndex + 1}</span>}
          <span className="source-line-content" dangerouslySetInnerHTML={{ __html: (rich ? highlightLine(line, language) : escapeHTML(line)) || ' ' }} />
        </span>;
      })}
    </div></code></pre>
  </div>;
}
function highlightLine(line: string, language: string): string { const normalized = languageAliases[language] ?? language; return hljs.getLanguage(normalized) ? hljs.highlight(line, { language: normalized, ignoreIllegals: true }).value : escapeHTML(line); }
function escapeHTML(value: string): string { return value.replace(/[&<>"']/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' })[character] ?? character); }
