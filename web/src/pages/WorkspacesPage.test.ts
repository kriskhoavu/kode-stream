import { describe, expect, it } from 'vitest';
import { applySegmentRole, inferCompatibilityFields, normalizeDroppedPath, parseSources, previewPathSegments } from './WorkspacesPage';

describe('normalizeDroppedPath', () => {
  it('decodes file URLs dropped onto the path field', () => {
    expect(normalizeDroppedPath('file:///Users/me/My%20Repo')).toBe('/Users/me/My Repo');
  });

  it('keeps plain paths intact', () => {
    expect(normalizeDroppedPath('"/Users/me/repo"')).toBe('/Users/me/repo');
  });
});

describe('parseSources', () => {
  it('parses comma-separated plan roots', () => {
    expect(parseSources('plans, docs, docs/plans')).toEqual(['plans', 'docs', 'docs/plans']);
  });

  it('deduplicates and ignores empty entries', () => {
    expect(parseSources('plans, , docs, plans')).toEqual(['plans', 'docs']);
  });
});

describe('inferCompatibilityFields', () => {
  it('maps scope and identifier variables from the path pattern', () => {
    expect(inferCompatibilityFields('{scope}/feature/{identifier}', 'docs')).toEqual({
      scope: '{scope}',
      identifier: '{identifier}'
    });
  });

  it('uses the source name as scope when only an identifier variable exists', () => {
    expect(inferCompatibilityFields('{identifier}', 'docs')).toEqual({
      scope: 'docs',
      identifier: '{identifier}'
    });
  });

  it('keeps legacy service and ticket variables compatible', () => {
    expect(inferCompatibilityFields('{service}/{ticket}', 'plans')).toEqual({
      scope: '{service}',
      identifier: '{ticket}'
    });
  });
});

describe('source structure path helpers', () => {
  it('returns preview path segments relative to the source directory', () => {
    expect(previewPathSegments('docs/api/feature/DI-101', 'docs')).toEqual(['api', 'feature', 'DI-101']);
  });

  it('applies a clicked segment role to the path pattern', () => {
    expect(applySegmentRole('api/feature/DI-101', ['api', 'feature', 'DI-101'], 0, 'scope')).toBe('{scope}/feature/DI-101');
    expect(applySegmentRole('{scope}/feature/DI-101', ['api', 'feature', 'DI-101'], 2, 'identifier')).toBe('{scope}/feature/{identifier}');
    expect(applySegmentRole('{scope}/{identifier}/DI-101', ['api', 'feature', 'DI-101'], 1, 'literal')).toBe('{scope}/feature/DI-101');
  });
});
