import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { FileContent } from '../../lib/types';
import { ContentViewer } from './ContentViewer';
import { renderMarkdown } from './renderers/MarkdownPreview';
import { SourceCodeView } from './renderers/SourceCodeView';

function file(overrides: Partial<FileContent> = {}): FileContent {
  return {
    id: 'README_md',
    path: 'README.md',
    content: '# Viewer',
    language: 'markdown',
    hash: 'hash',
    kind: 'markdown',
    sizeBytes: 8,
    editable: true,
    ...overrides
  };
}

describe('ContentViewer', () => {
  it('renders Markdown and switches to source', async () => {
    render(<ContentViewer file={file()} content="# Viewer" />);

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Viewer' })).toBeInTheDocument(), { timeout: 3000 });
    fireEvent.click(screen.getByRole('tab', { name: 'Source' }));
    await waitFor(() => expect(document.querySelector('.source-line-content')).toHaveTextContent('# Viewer'), { timeout: 3000 });
  });

  it('uses structured mode for JSON and preserves source fallback', async () => {
    render(<ContentViewer file={file({ id: 'data_json', path: 'data.json', kind: 'json', language: 'json', editable: false })} content='{"enabled":true}' />);

    expect(await screen.findByText('enabled:', {}, { timeout: 3000 })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('tab', { name: 'Source' }));
    await waitFor(() => expect(document.querySelector('.source-line-content')).toHaveTextContent('{"enabled":true}'), { timeout: 3000 });
  });

  it('does not run rich renderers for large files', async () => {
    render(<ContentViewer file={file({ sizeBytes: 2 << 20 })} content="# Large" />);

    expect(screen.getByText('Rich preview is paused for this large file.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Open source' }));
    await waitFor(() => expect(document.querySelector('.source-line-content')).toHaveTextContent('# Large'));
  });

  it('renders supported image data without a source mode', () => {
    const dataURL = 'data:image/png;base64,iVBORw0KGgo=';
    render(<ContentViewer file={file({ id: 'diagram_png', path: 'diagram.png', kind: 'image', language: 'image/png', editable: false })} content={dataURL} />);

    expect(screen.getByRole('img', { name: 'diagram.png' })).toHaveAttribute('src', dataURL);
    expect(screen.queryByRole('tab', { name: 'Source' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Zoom in' }));
    expect(screen.getByRole('button', { name: 'Reset zoom to 100%' })).toHaveTextContent('125%');
    expect(screen.getByRole('img', { name: 'diagram.png' })).toHaveStyle({ width: '125%' });
    const canvas = document.querySelector('.image-preview-canvas') as HTMLDivElement;
    canvas.scrollLeft = 50;
    canvas.scrollTop = 40;
    fireEvent(canvas, pointerEvent('pointerdown', { pointerId: 1, button: 0, clientX: 100, clientY: 100 }));
    fireEvent(canvas, pointerEvent('pointermove', { pointerId: 1, clientX: 75, clientY: 70 }));
    expect(canvas.scrollLeft).toBe(75);
    expect(canvas.scrollTop).toBe(70);
    fireEvent.pointerUp(canvas, { pointerId: 1 });
    fireEvent.click(screen.getByRole('button', { name: 'Fit image' }));
    expect(screen.getByRole('button', { name: 'Fit image' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('renders HTML in a sandboxed preview with a source fallback', async () => {
    render(<ContentViewer file={file({ id: 'page_html', path: 'page.html', kind: 'html', language: 'html', editable: true })} content='<h1>Preview</h1><script>alert(1)</script>' />);

    expect(await screen.findByTitle('HTML preview')).toHaveAttribute('sandbox', '');
    fireEvent.click(screen.getByRole('tab', { name: 'Source' }));
    await waitFor(() => expect(document.querySelector('.source-line-content')).toHaveTextContent('<h1>Preview</h1>'));
  });

  it('sanitizes Markdown output and marks external links', async () => {
    const html = await renderMarkdown('<script>alert("x")</script>\n\n[Site](https://example.test)');
    const parsed = new DOMParser().parseFromString(`<body>${html}</body>`, 'text/html');

    expect(parsed.querySelector('script')).toBeNull();
    expect(parsed.querySelector('a')?.getAttribute('target')).toBe('_blank');
    expect(parsed.querySelector('a')?.getAttribute('rel')).toContain('noopener');
  });

  it('pauses source highlighting for large files while preserving controls', () => {
    render(<SourceCodeView content={`const x = "${'x'.repeat(1 << 20)}";\n<unsafe>`} language="typescript" />);

    expect(screen.getByText('Highlighting paused for this large file.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Toggle line numbers' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'Toggle line wrapping' })).toHaveAttribute('aria-pressed', 'false');
    expect(document.querySelector('.source-line-content')?.textContent).toContain('const x = "');
  });
});

function pointerEvent(type: string, values: Record<string, number>) {
  const event = new Event(type, { bubbles: true });
  for (const [key, value] of Object.entries(values)) Object.defineProperty(event, key, { value });
  return event;
}
