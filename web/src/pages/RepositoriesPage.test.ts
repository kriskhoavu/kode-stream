import { describe, expect, it } from 'vitest';
import { normalizeDroppedPath, parsePlanDirectories } from './RepositoriesPage';

describe('normalizeDroppedPath', () => {
  it('decodes file URLs dropped onto the path field', () => {
    expect(normalizeDroppedPath('file:///Users/me/My%20Repo')).toBe('/Users/me/My Repo');
  });

  it('keeps plain paths intact', () => {
    expect(normalizeDroppedPath('"/Users/me/repo"')).toBe('/Users/me/repo');
  });
});

describe('parsePlanDirectories', () => {
  it('parses comma-separated plan roots', () => {
    expect(parsePlanDirectories('plans, docs, docs/plans')).toEqual(['plans', 'docs', 'docs/plans']);
  });

  it('deduplicates and ignores empty entries', () => {
    expect(parsePlanDirectories('plans, , docs, plans')).toEqual(['plans', 'docs']);
  });
});
