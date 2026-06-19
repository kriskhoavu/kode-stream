import type { SourceStructureCard } from '../../lib/types';

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

export function inferCompatibilityFields(pathPattern: string, directory: string): Pick<SourceStructureCard['fields'], 'scope' | 'identifier'> {
  const variables = Array.from(new Set(Array.from(pathPattern.matchAll(/\{([A-Za-z][A-Za-z0-9_]*)\}/g)).map((match) => match[1])));
  const sourceName = lastPathSegment(directory) || 'source';
  const scopeVariable = preferredVariable(variables, ['scope', 'service']) ?? (variables.length > 1 ? variables[0] : '');
  const identifierVariable = preferredVariable(variables, ['identifier', 'ticket']) ?? (variables.length > 1 ? variables[variables.length - 1] : variables[0] ?? '');
  return {
    scope: scopeVariable ? `{${scopeVariable}}` : sourceName,
    identifier: identifierVariable ? `{${identifierVariable}}` : lastLiteralPathSegment(pathPattern) || sourceName
  };
}

export function lastPathSegment(value: string): string {
  return value.split(/[\\/]/).filter(Boolean).at(-1) ?? '';
}

function preferredVariable(variables: string[], preferred: string[]): string | undefined {
  return preferred.find((name) => variables.includes(name));
}

function lastLiteralPathSegment(pathPattern: string): string {
  return pathPattern.split('/').map((segment) => segment.trim()).filter(Boolean).filter((segment) => !segment.includes('{') && !segment.includes('}')).at(-1) ?? '';
}
