import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { SourceCodeView } from './SourceCodeView';

describe('SourceCodeView', () => {
  const writeText = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    writeText.mockClear();
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } });
  });

  it('highlights known source and escapes unknown source', () => {
    const { rerender } = render(<SourceCodeView content="const value = 1;" language="typescript" />);
    expect(document.querySelector('.hljs-keyword')).toHaveTextContent('const');

    rerender(<SourceCodeView content="<script>" language="unknown" />);
    expect(screen.getByText('<script>')).toBeInTheDocument();
    expect(document.querySelector('script')).not.toBeInTheDocument();
  });

  it('toggles wrapping and line numbers and copies exact source', async () => {
    const source = 'one\ntwo';
    render(<SourceCodeView content={source} language="text" />);

    fireEvent.click(screen.getByRole('button', { name: 'Toggle line wrapping' }));
    expect(document.querySelector('.source-code-view')).toHaveClass('wrap');
    fireEvent.click(screen.getByRole('button', { name: 'Toggle line numbers' }));
    expect(document.querySelector('.source-line-number')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Copy source' }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(source));
  });
});
