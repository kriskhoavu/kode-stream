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
  const generationRef = useRef(0);
  const activeFileIDRef = useRef<string | null>(null);
  const mountedRef = useRef(true);
  const activeFileRef = useRef<FileContent | null>(null);
  const contentRef = useRef('');
  const savedContentRef = useRef('');
  const inFlightRef = useRef<Promise<boolean> | null>(null);

  saveRef.current = save;
  onSavedRef.current = onSaved;
  onErrorRef.current = onError;
  activeFileRef.current = file;
  contentRef.current = content;
  savedContentRef.current = savedContent;

  const clearTimers = useCallback(() => {
    if (timerRef.current !== null) window.clearTimeout(timerRef.current);
    if (settledTimerRef.current !== null) window.clearTimeout(settledTimerRef.current);
    timerRef.current = null;
    settledTimerRef.current = null;
  }, []);

  const open = useCallback((nextFile: FileContent | null) => {
    clearTimers();
    generationRef.current += 1;
    activeFileIDRef.current = nextFile?.id ?? null;
    activeFileRef.current = nextFile;
    contentRef.current = nextFile?.content ?? '';
    savedContentRef.current = nextFile?.content ?? '';
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
    const generation = generationRef.current;
    const isCurrent = () => mountedRef.current && generationRef.current === generation && activeFileIDRef.current === targetFile.id;
    setSaving(true);
    setState('saving');
    try {
      const updated = await saveRef.current(targetFile, nextContent);
      if (!isCurrent()) return false;
      activeFileRef.current = updated;
      savedContentRef.current = nextContent;
      setFile(updated);
      setSavedContent(nextContent);
      setState('saved');
      settledTimerRef.current = window.setTimeout(() => setState('idle'), 1600);
      await onSavedRef.current?.(updated);
      return true;
    } catch (error) {
      if (!isCurrent()) return false;
      setState('error');
      onErrorRef.current?.(error);
      return false;
    } finally {
      if (isCurrent()) setSaving(false);
    }
  }, []);

  const startSave = useCallback((targetFile: FileContent, nextContent: string): Promise<boolean> => {
    const pending = inFlightRef.current;
    if (pending) {
      return pending.then(() => {
        const currentFile = activeFileRef.current;
        if (!currentFile || contentRef.current === savedContentRef.current) return true;
        return startSave(currentFile, contentRef.current);
      });
    }
    const operation = saveContent(targetFile, nextContent);
    inFlightRef.current = operation;
    void operation.finally(() => {
      if (inFlightRef.current === operation) inFlightRef.current = null;
    });
    return operation;
  }, [saveContent]);

  const saveNow = useCallback(async () => {
    const currentFile = activeFileRef.current;
    if (!currentFile || contentRef.current === savedContentRef.current) return true;
    return startSave(currentFile, contentRef.current);
  }, [startSave]);

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
    timerRef.current = window.setTimeout(() => void startSave(file, content), debounceMs);
    return () => {
      if (timerRef.current !== null) window.clearTimeout(timerRef.current);
      timerRef.current = null;
    };
  }, [content, debounceMs, file, savedContent, saving, startSave]);

  useEffect(() => () => {
    mountedRef.current = false;
    generationRef.current += 1;
    clearTimers();
  }, [clearTimers]);

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
