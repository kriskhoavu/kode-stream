import type { SourceStructureCard } from '../../lib/types';

export type SourceStructureSegmentRole = 'folder' | 'item' | 'literal';

export function normalizeDroppedPath(value?: string): string {
  if (!value) return '';
  const trimmed = value.trim().replace(/^["']|["']$/g, '');
  if (!trimmed.startsWith('file://')) return trimmed;
  try {
    return decodeURIComponent(new URL(trimmed).pathname);
  } catch {
    return trimmed;
  }
}

export function parseSources(value: string): string[] {
  return Array.from(new Set(value.split(',').map((item) => item.trim()).filter(Boolean)));
}

export function lastPathSegment(value: string): string {
  return value.split(/[\\/]/).filter(Boolean).at(-1) ?? '';
}

export function previewPathSegments(path: string, directory: string): string[] {
  const pathSegments = path.split('/').map((segment) => segment.trim()).filter(Boolean);
  const directorySegments = directory.split('/').map((segment) => segment.trim()).filter(Boolean);
  if (directorySegments.every((segment, index) => pathSegments[index] === segment)) {
    return pathSegments.slice(directorySegments.length);
  }
  return pathSegments;
}

export function applySegmentRole(pathPattern: string, sampleSegments: string[], index: number, role: SourceStructureSegmentRole): string {
  if (index < 0) return pathPattern;
  const segments = pathPattern.split('/').map((segment) => segment.trim()).filter(Boolean);
  const maxLength = Math.max(segments.length, sampleSegments.length, index + 1);
  const next = Array.from({ length: maxLength }, (_, segmentIndex) => segments[segmentIndex] || sampleSegments[segmentIndex] || 'segment');
  if (role === 'folder') {
    next[index] = `{folder${index > 0 ? index + 1 : ''}}`;
  } else if (role === 'item') {
    next[index] = '{item}';
  } else {
    next[index] = literalPathSegment(sampleSegments[index] || next[index]);
  }
  return next.join('/');
}

function literalPathSegment(value: string): string {
  return value.replace(/[{}*?]/g, '').trim() || 'segment';
}
