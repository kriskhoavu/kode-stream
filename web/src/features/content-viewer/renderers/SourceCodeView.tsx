import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
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
  const rich = content.length <= richPreviewThresholdBytes && new TextEncoder().encode(content).length <= richPreviewThresholdBytes;
  const lines = useMemo(() => content.split('\n'), [content]);
  const scrollRef = useRef<HTMLPreElement>(null);
	const copyResetTimer = useRef<number | undefined>(undefined);
	const mounted = useRef(true);
  const [windowStart, setWindowStart] = useState(0);
	const [measurements, setMeasurements] = useState<{ content: string; wrap: boolean; lineNumbers: boolean; heights: Record<number, number> }>(() => ({ content, wrap, lineNumbers, heights: {} }));
	const rowHeights = measurements.content === content && measurements.wrap === wrap && measurements.lineNumbers === lineNumbers ? measurements.heights : {};
  const selectedLine = selection?.lineNumber ?? 0;
  const windowEnd = Math.min(lines.length, windowStart + renderedLineWindow);
	const offsets = useMemo(() => {
		const values = new Array<number>(lines.length + 1); values[0] = 0;
		for (let index = 0; index < lines.length; index++) values[index + 1] = values[index] + (rowHeights[index] ?? estimatedLineHeight);
		return values;
	}, [lines.length, rowHeights]);

	useEffect(() => {
		mounted.current = true;
		return () => {
			mounted.current = false;
			if (copyResetTimer.current !== undefined) window.clearTimeout(copyResetTimer.current);
		};
	}, []);

	useEffect(() => {
		const invalidateMeasurements = () => setMeasurements({ content, wrap, lineNumbers, heights: {} });
		window.addEventListener('resize', invalidateMeasurements);
		const target = scrollRef.current;
		if (target && typeof ResizeObserver !== 'undefined') {
			const observer = new ResizeObserver(invalidateMeasurements);
			observer.observe(target);
			return () => { window.removeEventListener('resize', invalidateMeasurements); observer.disconnect(); };
		}
		return () => window.removeEventListener('resize', invalidateMeasurements);
	}, [content, wrap, lineNumbers]);

	useLayoutEffect(() => {
		const measured: Record<number, number> = {};
		for (const node of scrollRef.current?.querySelectorAll<HTMLSpanElement>('.source-code-line') ?? []) {
			const index = Number(node.dataset.lineIndex);
			const height = node.getBoundingClientRect().height;
			if (height > 0) measured[index] = height;
		}
		if (Object.keys(measured).length === 0) return;
		setMeasurements((current) => {
			const sameSource = current.content === content && current.wrap === wrap && current.lineNumbers === lineNumbers;
			const heights = sameSource ? current.heights : {};
			if (Object.entries(measured).every(([index, height]) => heights[Number(index)] === height)) return current;
			return { content, wrap, lineNumbers, heights: { ...heights, ...measured } };
		});
	});

  useEffect(() => {
    if (!selectedLine || selectedLine > lines.length) return;
    const start = Math.max(0, Math.min(Math.max(0, lines.length - renderedLineWindow), selectedLine - Math.floor(renderedLineWindow / 2) - 1));
    setWindowStart((current) => current === start ? current : start);
    const frame = requestAnimationFrame(() => { if (scrollRef.current) scrollRef.current.scrollTop = Math.max(0, offsets[selectedLine - 1] - scrollRef.current.clientHeight / 2); });
    return () => cancelAnimationFrame(frame);
  }, [lines.length, offsets, selectedLine]);

  const updateWindow = () => {
    const top = scrollRef.current?.scrollTop ?? 0;
		let low = 0, high = lines.length;
		while (low < high) { const middle = Math.floor((low + high) / 2); if (offsets[middle + 1] <= top) low = middle + 1; else high = middle; }
    const start = Math.max(0, Math.min(Math.max(0, lines.length - renderedLineWindow), low - 40));
    setWindowStart((current) => current === start ? current : start);
  };
  const copy = async () => {
		await navigator.clipboard.writeText(content);
		if (!mounted.current) return;
		setCopied(true);
		if (copyResetTimer.current !== undefined) window.clearTimeout(copyResetTimer.current);
		copyResetTimer.current = window.setTimeout(() => { copyResetTimer.current = undefined; setCopied(false); }, 1200);
	};

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
        return <span data-line-index={lineIndex} className={`source-code-line${selected ? ' selected' : ''}`} key={lineIndex} aria-current={selected ? 'true' : undefined}>
          {lineNumbers && <span className="source-line-number" aria-hidden="true">{lineIndex + 1}</span>}
          <span className="source-line-content" dangerouslySetInnerHTML={{ __html: (rich ? highlightLine(line, language) : escapeHTML(line)) || ' ' }} />
        </span>;
      })}
    </div></code></pre>
  </div>;
}
function highlightLine(line: string, language: string): string { const normalized = languageAliases[language] ?? language; return hljs.getLanguage(normalized) ? hljs.highlight(line, { language: normalized, ignoreIllegals: true }).value : escapeHTML(line); }
function escapeHTML(value: string): string { return value.replace(/[&<>"']/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' })[character] ?? character); }
