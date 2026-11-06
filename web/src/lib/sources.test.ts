import { describe, expect, it } from 'vitest';
import { defaultShownIn, normalizeSources, sourcePaths, sourcesShownIn, toWorkspaceSource } from './sources';

describe('workspace sources', () => {
  it('gives a bare path the named defaults', () => {
    expect(defaultShownIn('plans')).toEqual(['workstream']);
    expect(defaultShownIn('docs/wiki/')).toEqual(['knowledge']);
    expect(defaultShownIn('docs')).toEqual(['workstream', 'knowledge']);
    expect(toWorkspaceSource('wiki')).toEqual({ path: 'wiki', shownIn: ['knowledge'] });
  });

  it('keeps an object form, drops unknown surfaces and orders the rest', () => {
    expect(toWorkspaceSource({ path: 'wiki', shownIn: ['knowledge', 'workstream'] })).toEqual({ path: 'wiki', shownIn: ['workstream', 'knowledge'] });
    expect(toWorkspaceSource({ path: 'wiki', shownIn: ['board' as never] })).toEqual({ path: 'wiki', shownIn: ['knowledge'] });
  });

  it('lists the paths shown on a surface', () => {
    const workspace = { sources: normalizeSources(['plans', 'wiki', { path: 'docs', shownIn: ['knowledge'] }]) };
    expect(sourcePaths(workspace)).toEqual(['plans', 'wiki', 'docs']);
    expect(sourcesShownIn(workspace, 'workstream')).toEqual(['plans']);
    expect(sourcesShownIn(workspace, 'knowledge')).toEqual(['wiki', 'docs']);
    expect(normalizeSources(undefined)).toEqual([]);
  });
});
