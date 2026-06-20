import { useMemo, useState } from 'react';
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

export const richPreviewThresholdBytes = 1 << 20;

const languages = { bash, c, cpp, csharp, css, dockerfile, go, java, javascript, json, kotlin, makefile, python, ruby, rust, sql, typescript, xml, yaml };
for (const [name, definition] of Object.entries(languages)) hljs.registerLanguage(name, definition);

const languageAliases: Record<string, string> = {
  shell: 'bash',
  jsx: 'javascript',
  tsx: 'typescript',
  html: 'xml'
};

export function SourceCodeView({ content, language, truncated = false }: { content: string; language: string; truncated?: boolean }) {
  const [wrap, setWrap] = useState(false);
  const [lineNumbers, setLineNumbers] = useState(true);
  const [copied, setCopied] = useState(false);
  const rich = new TextEncoder().encode(content).length <= richPreviewThresholdBytes;
  const lines = useMemo(() => content.split('\n').map((line) => rich ? highlightLine(line, language) : escapeHTML(line)), [content, language, rich]);

  const copy = async () => {
    await navigator.clipboard.writeText(content);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1200);
  };

  return (
    <div className={`source-code-view ${wrap ? 'wrap' : ''}`}>
      <div className="viewer-toolbar source-toolbar" aria-label="Source controls">
        {!rich && <span className="viewer-notice">Highlighting paused for this large file.</span>}
        {truncated && <span className="viewer-notice">Showing the first part of this file.</span>}
        <span className="viewer-toolbar-spacer" />
        <button type="button" className={lineNumbers ? 'active' : ''} title="Toggle line numbers" aria-label="Toggle line numbers" aria-pressed={lineNumbers} onClick={() => setLineNumbers((current) => !current)}><ListOrdered size={15} /></button>
        <button type="button" className={wrap ? 'active' : ''} title="Toggle line wrapping" aria-label="Toggle line wrapping" aria-pressed={wrap} onClick={() => setWrap((current) => !current)}><WrapText size={15} /></button>
        <button type="button" title="Copy source" aria-label="Copy source" onClick={() => void copy()}>{copied ? <Check size={15} /> : <Copy size={15} />}</button>
        <span className="sr-only" aria-live="polite">{copied ? 'Source copied' : ''}</span>
      </div>
      <pre className="source-code-scroll" data-language={language}>
        <code>
          {lines.map((line, index) => (
            <span className="source-code-line" key={index}>
              {lineNumbers && <span className="source-line-number" aria-hidden="true">{index + 1}</span>}
              <span className="source-line-content" dangerouslySetInnerHTML={{ __html: line || ' ' }} />
            </span>
          ))}
        </code>
      </pre>
    </div>
  );
}

function highlightLine(line: string, language: string): string {
  const normalized = languageAliases[language] ?? language;
  if (!hljs.getLanguage(normalized)) return escapeHTML(line);
  return hljs.highlight(line, { language: normalized, ignoreIllegals: true }).value;
}

function escapeHTML(value: string): string {
  return value.replace(/[&<>"']/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' })[character] ?? character);
}
