import { useCallback, useEffect, useRef, useState } from 'react';
import type { FileContent } from '../../lib/types';

export type AutoSaveState = 'idle' | 'pending' | 'saving' | 'saved' | 'error';

interface FileEditorSessionOptions {
  save: (file: FileContent, content: string) => Promise<FileContent>;
  onSaved?: (file: FileContent) => void | Promise<void>;
  onError?: (error: unknown) => void;
  debounceMs?: number;
}

export function useFileEditorSession({ save, onSaved, onError, debounceMs = 900 }: FileEditorSessionOptions) {
  const [file, setFile] = useState<FileContent | null>(null);
  const [content, setContent] = useState('');
  const [savedContent, setSavedContent] = useState('');
  const [state, setState] = useState<AutoSaveState>('idle');
  const [saving, setSaving] = useState(false);
  const saveRef = useRef(save);
  const onSavedRef = useRef(onSaved);
  const onErrorRef = useRef(onError);
  const timerRef = useRef<number | null>(null);
  const settledTimerRef = useRef<number | null>(null);

  saveRef.current = save;
  onSavedRef.current = onSaved;
  onErrorRef.current = onError;

  const clearTimers = useCallback(() => {
    if (timerRef.current !== null) window.clearTimeout(timerRef.current);
    if (settledTimerRef.current !== null) window.clearTimeout(settledTimerRef.current);
    timerRef.current = null;
    settledTimerRef.current = null;
  }, []);

  const open = useCallback((nextFile: FileContent | null) => {
    clearTimers();
    setFile(nextFile);
    setContent(nextFile?.content ?? '');
    setSavedContent(nextFile?.content ?? '');
    setState('idle');
    setSaving(false);
  }, [clearTimers]);

  const saveContent = useCallback(async (targetFile: FileContent, nextContent: string) => {
    if (timerRef.current !== null) window.clearTimeout(timerRef.current);
    if (settledTimerRef.current !== null) window.clearTimeout(settledTimerRef.current);
    timerRef.current = null;
    settledTimerRef.current = null;
    setSaving(true);
    setState('saving');
    try {
      const updated = await saveRef.current(targetFile, nextContent);
      setFile(updated);
      setSavedContent(nextContent);
      setState('saved');
      settledTimerRef.current = window.setTimeout(() => setState('idle'), 1600);
      await onSavedRef.current?.(updated);
      return true;
    } catch (error) {
      setState('error');
      onErrorRef.current?.(error);
      return false;
    } finally {
      setSaving(false);
    }
  }, []);

  const saveNow = useCallback(async () => {
    if (!file || content === savedContent) return true;
    return saveContent(file, content);
  }, [content, file, saveContent, savedContent]);

  useEffect(() => {
    if (!file) {
      setState('idle');
      return;
    }
    if (content === savedContent) {
      setState((current) => current === 'pending' ? 'idle' : current);
      return;
    }
    if (saving) {
      setState('pending');
      return;
    }
    if (timerRef.current !== null) window.clearTimeout(timerRef.current);
    setState('pending');
    timerRef.current = window.setTimeout(() => void saveContent(file, content), debounceMs);
    return () => {
      if (timerRef.current !== null) window.clearTimeout(timerRef.current);
      timerRef.current = null;
    };
  }, [content, debounceMs, file, savedContent, saveContent, saving]);

  useEffect(() => clearTimers, [clearTimers]);

  return {
    file,
    content,
    setContent,
    savedContent,
    dirty: file !== null && content !== savedContent,
    saving,
    state,
    open,
    saveNow
  };
}

export function autoSaveLabel(state: AutoSaveState): string {
  switch (state) {
    case 'pending': return 'Autosave pending';
    case 'saving': return 'Saving...';
    case 'saved': return 'Saved';
    case 'error': return 'Autosave failed';
    default: return 'Autosave on';
  }
}
