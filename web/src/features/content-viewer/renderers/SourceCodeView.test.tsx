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

	it('renders a bounded window and brings a selected match line into it', async () => {
		const content = Array.from({ length: 2000 }, (_, index) => `line ${index + 1} ${'wrapped text '.repeat(40)}`).join('\n');
		render(<SourceCodeView content={content} language="text" selection={{ workspaceId: 'ws', path: 'large.txt', lineNumber: 1800, columnStart: 1, columnEnd: 5 }} />);
		await waitFor(() => expect(screen.getByText((value) => value.startsWith('line 1800 '))).toBeInTheDocument());
		expect(document.querySelectorAll('.source-code-line').length).toBeLessThanOrEqual(240);
		expect(document.querySelector('.source-code-line.selected')).toHaveAttribute('aria-current', 'true');
		fireEvent.click(screen.getByRole('button', { name: 'Toggle line wrapping' }));
		expect(document.querySelector('.source-code-view')).toHaveClass('wrap');
		fireEvent(window, new Event('resize'));
		await waitFor(() => expect(document.querySelector('.source-code-line.selected')).toHaveAttribute('aria-current', 'true'));
		expect(document.querySelectorAll('.source-code-line').length).toBeLessThanOrEqual(240);
	});
});
