import { describe, expect, it } from 'vitest';
import { applySegmentRole, inferCompatibilityFields, normalizeDroppedPath, parseSources, previewPathSegments, settingsEditorFromResult } from './WorkspacesPage';

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
  it('maps the source root and item variable from the path pattern', () => {
    expect(inferCompatibilityFields('{folder}/feature/{item}', 'docs')).toEqual({
      scope: 'docs',
      identifier: '{item}'
    });
  });

  it('uses the source name when only an item variable exists', () => {
    expect(inferCompatibilityFields('{item}', 'docs')).toEqual({
      scope: 'docs',
      identifier: '{item}'
    });
  });

  it('keeps legacy service and ticket variables compatible', () => {
    expect(inferCompatibilityFields('{service}/{ticket}', 'plans')).toEqual({
      scope: 'plans',
      identifier: '{ticket}'
    });
  });
});

describe('source item path helpers', () => {
  it('returns preview path segments relative to the source directory', () => {
    expect(previewPathSegments('docs/api/feature/DI-101', 'docs')).toEqual(['api', 'feature', 'DI-101']);
  });

  it('applies a clicked segment role to the path pattern', () => {
    expect(applySegmentRole('api/feature/DI-101', ['api', 'feature', 'DI-101'], 0, 'folder')).toBe('{folder}/feature/DI-101');
    expect(applySegmentRole('{folder}/feature/DI-101', ['api', 'feature', 'DI-101'], 2, 'item')).toBe('{folder}/feature/{item}');
    expect(applySegmentRole('{folder}/{item}/DI-101', ['api', 'feature', 'DI-101'], 1, 'literal')).toBe('{folder}/feature/DI-101');
  });
});

describe('settingsEditorFromResult', () => {
  const workspace = { id: 'ws', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['docs'], createdAt: '' };

  it('starts new source settings from the best actual proposal', () => {
    const editor = settingsEditorFromResult(workspace, 'docs', {
      directory: 'docs',
      exists: false,
      settings: {
        version: 1,
        cards: [{
          pathPattern: '{folder}/feature/{item}',
          fields: { source: 'docs', item: '{item}', scope: 'docs', identifier: '{item}', title: 'readme_heading', status: 'draft', tags: ['docs'] }
        }]
      },
      warnings: [],
      proposals: [{
        id: 'actual-identifier',
        label: 'Top-level folders',
        summary: 'Creates 1 card, for example docs/a12.',
        confidence: 'high',
        card: {
          pathPattern: '{item}',
          fields: { source: 'docs', item: '{item}', scope: 'docs', identifier: '{item}', title: 'readme_heading', status: 'draft', tags: ['docs'] }
        },
        preview: [{ path: 'docs/a12', source: 'docs', item: 'a12', scope: 'docs', identifier: 'a12', title: 'A12', status: 'draft', tags: ['docs'] }]
      }],
      preview: []
    });

    expect(editor.card.pathPattern).toBe('{item}');
    expect(editor.selectedProposalId).toBe('actual-identifier');
    expect(editor.preview).toHaveLength(1);
    expect(editor.preview[0].path).toBe('docs/a12');
  });

  it('uses an unsorted source preview when no suggestion is selected', () => {
    const editor = settingsEditorFromResult(workspace, 'docs', {
      directory: 'docs',
      exists: false,
      settings: { version: 1, cards: [] },
      warnings: [],
      proposals: [],
      preview: []
    });

    expect(editor.selectedProposalId).toBe('unsorted');
    expect(editor.preview).toEqual([{
      path: 'docs',
      source: 'docs',
      item: 'docs',
      scope: 'docs',
      identifier: 'docs',
      title: 'docs',
      status: 'unsorted',
      tags: ['docs']
    }]);
  });
});
