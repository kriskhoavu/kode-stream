import { act, renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useFileEditorSession } from './useFileEditorSession';
import type { FileContent } from '../../lib/types';

const markdownFile: FileContent = {
  id: 'README_md', path: 'README.md', content: 'old', hash: 'one', kind: 'markdown', language: 'markdown', sizeBytes: 3, editable: true
};

describe('useFileEditorSession', () => {
  it('autosaves edits and keeps the returned hash', async () => {
    vi.useFakeTimers();
    const save = vi.fn(async (_file: FileContent, content: string) => ({ ...markdownFile, content, hash: 'two' }));
    const { result } = renderHook(() => useFileEditorSession({ save, debounceMs: 20 }));
    act(() => result.current.open(markdownFile));
    act(() => result.current.setContent('new'));
    expect(result.current.state).toBe('pending');
    await act(async () => vi.advanceTimersByTimeAsync(20));
    expect(save).toHaveBeenCalledWith(markdownFile, 'new');
    expect(result.current.file?.hash).toBe('two');
    expect(result.current.dirty).toBe(false);
    vi.useRealTimers();
  });

  it('reports stale save failures without replacing editor content', async () => {
    const onError = vi.fn();
    const { result } = renderHook(() => useFileEditorSession({ save: async () => { throw new Error('stale'); }, onError }));
    act(() => result.current.open(markdownFile));
    act(() => result.current.setContent('draft'));
    await act(async () => { await result.current.saveNow(); });
    expect(result.current.state).toBe('error');
    expect(result.current.content).toBe('draft');
    expect(onError).toHaveBeenCalled();
  });
});
