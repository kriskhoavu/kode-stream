import type { MutableRefObject } from 'react';
import { ApiError } from '../../shared/api';

export function clearTimeoutRef(ref: MutableRefObject<number | null>) {
  if (ref.current === null) return;
  window.clearTimeout(ref.current);
  ref.current = null;
}

export function operationErrorMessage(caught: unknown, fallback: string) {
  const message = caught instanceof Error ? caught.message : fallback;
  const hint = caught instanceof ApiError ? caught.recoveryHint : '';
  return [message, hint].filter(Boolean).join(' ');
}

export function unique(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean))).sort((a, b) => a.localeCompare(b));
}

export function sameJSON(left: unknown, right: unknown): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}
