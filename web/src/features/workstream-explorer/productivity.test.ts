import { describe, expect, it } from 'vitest';
import { ancestorDirectoryPaths, buildWorkspaceGitStateMap } from './productivity';

describe('Explorer productivity helpers', () => {
  it('aggregates the highest-priority Git state to parent directories', () => {
    const states = buildWorkspaceGitStateMap('ws', [
      { path: 'docs/guide.md', status: 'modified', staged: false, conflict: false },
      { path: 'docs/conflict.md', status: 'conflicted', staged: false, conflict: true }
    ]);
    expect(states.get('ws:docs')?.status).toBe('conflicted');
    expect(states.get('ws:docs/guide.md')?.status).toBe('modified');
  });

  it('builds ancestor paths for files and directories', () => {
    expect(ancestorDirectoryPaths('docs/guides/start.md')).toEqual(['', 'docs', 'docs/guides']);
    expect(ancestorDirectoryPaths('docs/guides', 'directory')).toEqual(['', 'docs', 'docs/guides']);
  });
});
