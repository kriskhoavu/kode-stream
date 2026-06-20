import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { FileContent } from '../../lib/types';
import { ContentViewer } from './ContentViewer';

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

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Viewer' })).toBeInTheDocument());
    fireEvent.click(screen.getByRole('tab', { name: 'Source' }));
    expect(screen.getByText('# Viewer')).toBeInTheDocument();
  });

  it('uses structured mode for JSON and preserves source fallback', () => {
    render(<ContentViewer file={file({ id: 'data_json', path: 'data.json', kind: 'json', language: 'json', editable: false })} content='{"enabled":true}' />);

    expect(screen.getByText('enabled:')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('tab', { name: 'Source' }));
    expect(document.querySelector('.source-line-content')).toHaveTextContent('{"enabled":true}');
  });

  it('does not run rich renderers for large files', () => {
    render(<ContentViewer file={file({ sizeBytes: 2 << 20 })} content="# Large" />);

    expect(screen.getByText('Rich preview is paused for this large file.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Open source' }));
    expect(screen.getByText('# Large')).toBeInTheDocument();
  });
});
