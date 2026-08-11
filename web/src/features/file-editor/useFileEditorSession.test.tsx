import { act, renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useFileEditorSession } from './useFileEditorSession';
import type { FileContent } from '../../lib/types';

const markdownFile: FileContent = {
  id: 'README_md', path: 'README.md', content: 'old', hash: 'one', kind: 'markdown', language: 'markdown', sizeBytes: 3, editable: true
};

const secondFile: FileContent = {
  ...markdownFile, id: 'second', path: 'second.md', content: 'second', hash: 'second-hash'
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

  it('ignores a pending save after another file is opened', async () => {
    let resolveSave!: (file: FileContent) => void;
    const save = vi.fn(() => new Promise<FileContent>((resolve) => { resolveSave = resolve; }));
    const { result } = renderHook(() => useFileEditorSession({ save }));
    act(() => result.current.open(markdownFile));
    act(() => result.current.setContent('draft A'));
    let pending!: Promise<boolean>;
    act(() => { pending = result.current.saveNow(); });
    act(() => result.current.open(secondFile));
    await act(async () => {
      resolveSave({ ...markdownFile, content: 'draft A', hash: 'saved-a' });
      await pending;
    });
    expect(result.current.file).toEqual(secondFile);
    expect(result.current.content).toBe('second');
    expect(result.current.dirty).toBe(false);
  });

  it('serializes rapid saves against the hash returned by the first save', async () => {
    let resolveFirst!: (file: FileContent) => void;
    const save = vi.fn()
      .mockImplementationOnce(() => new Promise<FileContent>((resolve) => { resolveFirst = resolve; }))
      .mockImplementationOnce(async (file: FileContent, content: string) => ({ ...file, content, hash: 'three' }));
    const { result } = renderHook(() => useFileEditorSession({ save }));
    act(() => result.current.open(markdownFile));
    act(() => result.current.setContent('first edit'));
    let first!: Promise<boolean>;
    act(() => { first = result.current.saveNow(); });
    act(() => result.current.setContent('second edit'));
    let second!: Promise<boolean>;
    act(() => { second = result.current.saveNow(); });
    await act(async () => {
      resolveFirst({ ...markdownFile, content: 'first edit', hash: 'two' });
      await Promise.all([first, second]);
    });
    expect(save).toHaveBeenNthCalledWith(1, markdownFile, 'first edit');
    expect(save).toHaveBeenNthCalledWith(2, expect.objectContaining({ hash: 'two' }), 'second edit');
    expect(result.current.file?.hash).toBe('three');
    expect(result.current.content).toBe('second edit');
    expect(result.current.dirty).toBe(false);
  });

  it('ignores completion after unmount', async () => {
    let resolveSave!: (file: FileContent) => void;
    const onSaved = vi.fn();
    const { result, unmount } = renderHook(() => useFileEditorSession({
      save: () => new Promise<FileContent>((resolve) => { resolveSave = resolve; }), onSaved
    }));
    act(() => result.current.open(markdownFile));
    act(() => result.current.setContent('draft'));
    act(() => { void result.current.saveNow(); });
    unmount();
    await act(async () => resolveSave({ ...markdownFile, content: 'draft', hash: 'two' }));
    expect(onSaved).not.toHaveBeenCalled();
  });
});
