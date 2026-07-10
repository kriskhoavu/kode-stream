import { createElement } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ItemWorkspacePage } from './ItemWorkspacePage';
import { parseGitDiff } from '../shared/domain/diff';

vi.mock('./WorkstreamExplorer', () => ({
  WorkstreamExplorer: ({ location, embeddedHeaderContent, rightPanel }: { location?: { workspaceId?: string; path?: string; mode?: string }; embeddedHeaderContent?: ReturnType<typeof createElement>; rightPanel?: { title?: ReturnType<typeof createElement>; content?: ReturnType<typeof createElement> } }) => createElement(
    'div',
    undefined,
    embeddedHeaderContent,
    createElement(
      'div',
      { 'data-testid': 'embedded-explorer' },
      `${location?.workspaceId ?? 'no-workspace'}|${location?.path ?? 'no-path'}|${location?.mode ?? 'no-mode'}`
    ),
    rightPanel?.title ? createElement('h2', undefined, rightPanel.title) : null,
    rightPanel?.content
  )
}));

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('parseGitDiff', () => {
  it('parses additions and deletions with line numbers', () => {
    const files = parseGitDiff(`diff --git a/plans/platform/PM-003/README.md b/plans/platform/PM-003/README.md
index 1111111..2222222 100644
--- a/plans/platform/PM-003/README.md
+++ b/plans/platform/PM-003/README.md
@@ -1,3 +1,3 @@
 # PM-003
-Old text
+New text
 Context
`);

    expect(files).toHaveLength(1);
    expect(files[0].path).toBe('plans/platform/PM-003/README.md');
    expect(files[0].additions).toBe(1);
    expect(files[0].deletions).toBe(1);
    expect(files[0].lines.filter((line) => line.type === 'add')).toEqual([{ type: 'add', text: 'New text', newLine: 2 }]);
    expect(files[0].lines.filter((line) => line.type === 'delete')).toEqual([{ type: 'delete', text: 'Old text', oldLine: 2 }]);
  });

  it('preserves rename old and new paths', () => {
    const files = parseGitDiff(`diff --git a/docs/old.md b/docs/new.md
similarity index 80%
rename from docs/old.md
rename to docs/new.md
--- a/docs/old.md
+++ b/docs/new.md
@@ -1 +1 @@
-Old
+New
`);

    expect(files).toHaveLength(1);
    expect(files[0].oldPath).toBe('docs/old.md');
    expect(files[0].path).toBe('docs/new.md');
  });
});

describe('ItemWorkspacePage', () => {
  it('switches from item files to embedded workspace tree mode', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/items/item-1') {
        return Promise.resolve(response({
          id: 'item-1',
          workspaceId: 'ws-1',
          workspaceName: 'Workspace',
          scope: 'platform',
          branch: 'main',
          identifier: 'PM-012',
          title: 'Drag cards',
          status: 'draft',
          tags: [],
          metadataSource: 'plan.yaml',
          itemPath: 'items/platform/PM-012',
          counts: { files: 1 },
          warnings: []
        }));
      }
      if (url === '/api/items/item-1/files') {
        return Promise.resolve(response([
          { id: 'readme', name: 'README.md', path: 'README.md', type: 'file', editable: true, kind: 'markdown' }
        ]));
      }
      if (url === '/api/items/item-1/files/readme') {
        return Promise.resolve(response({
          id: 'readme',
          path: 'README.md',
          content: '# Drag cards',
          hash: 'hash',
          kind: 'markdown',
          sizeBytes: 12,
          editable: true,
          truncated: false
        }));
      }
      if (url === '/api/items/item-1/diff') return Promise.resolve(response({ diff: '' }));
      if (url === '/api/workspaces/ws-1/git/status') {
        return Promise.resolve(response({ workspaceId: 'ws-1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      }
      if (url === '/api/workspaces/ws-1/git/branches') {
        return Promise.resolve(response({ workspaceId: 'ws-1', current: 'main', branches: ['feature/DI-2026-ai-assistant-showcases', 'main'] }));
      }
      if (url === '/api/workspaces/ws-1/git/activity?path=items%2Fplatform%2FPM-012&limit=8') return Promise.resolve(response([]));
      if (url === '/api/workspaces') {
        return Promise.resolve(response([
          { id: 'ws-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['items'], createdAt: '2026-07-10T00:00:00Z' }
        ]));
      }
      if (url === '/api/items/item-1/jira') return Promise.resolve(response({ state: 'not_configured' }));
      return Promise.resolve(response({}));
    }));

    render(createElement(ItemWorkspacePage, { itemId: 'item-1', refreshKey: 0, workspaces: [{ id: 'ws-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['items'], createdAt: '2026-07-10T00:00:00Z' }], onBack: vi.fn(), onOpenItem: vi.fn(), onContentChanged: vi.fn() }));

    expect(await screen.findByRole('button', { name: 'Plan files' })).toHaveClass('active');
    expect(await screen.findByRole('button', { name: 'Select item branch' })).toHaveTextContent('main');
    fireEvent.click(screen.getByRole('button', { name: 'Explorer' }));

    await waitFor(() => expect(screen.getByTestId('embedded-explorer')).toHaveTextContent('ws-1|items/platform/PM-012/README.md|all'));
    expect(screen.getByRole('heading', { name: 'Work Item' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Info/i })).toBeInTheDocument();
    expect(screen.getByText('Verification Harness')).toBeInTheDocument();
  });

  it('loads a branch snapshot and opens the matching branch-scoped item', async () => {
    const onOpenItem = vi.fn();
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/items/item-1') {
        return Promise.resolve(response({
          id: 'item-1',
          workspaceId: 'ws-1',
          workspaceName: 'Workspace',
          scope: 'platform',
          branch: 'main',
          identifier: 'PM-012',
          title: 'Drag cards',
          status: 'draft',
          tags: [],
          metadataSource: 'plan.yaml',
          itemPath: 'items/platform/PM-012',
          sourceMode: 'working_tree',
          counts: { files: 1 },
          warnings: []
        }));
      }
      if (url === '/api/items/item-1/files') return Promise.resolve(response([]));
      if (url === '/api/items/item-1/diff') return Promise.resolve(response({ diff: '' }));
      if (url === '/api/workspaces/ws-1/git/status') return Promise.resolve(response({ workspaceId: 'ws-1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/ws-1/git/branches') return Promise.resolve(response({ workspaceId: 'ws-1', current: 'main', branches: ['feature/a', 'main'] }));
      if (url === '/api/workspaces/ws-1/git/activity?path=items%2Fplatform%2FPM-012&limit=8') return Promise.resolve(response([]));
      if (url === '/api/workspaces/ws-1/workstream/branch') {
        return Promise.resolve(response({
          workspaceId: 'ws-1',
          branch: 'feature/a',
          selectedBranch: 'feature/a',
          branchRef: 'refs/heads/feature/a',
          commit: 'abc',
          currentCheckoutBranch: 'main',
          sourceMode: 'snapshot',
          mode: 'snapshot',
          editable: false,
          scannedAt: '',
          itemCount: 1,
          warnings: [],
          items: [{ id: 'item-feature', workspaceId: 'ws-1', workspaceName: 'Workspace', branch: 'feature/a', scope: 'platform', identifier: 'PM-012', title: 'Drag cards', status: 'draft', tags: [], metadataSource: 'plan.yaml', itemPath: 'items/platform/PM-012', sourceMode: 'snapshot', editable: false }]
        }));
      }
      if (url === '/api/items/item-1/jira') return Promise.resolve(response({ state: 'not_configured' }));
      return Promise.resolve(response({}));
    }));

    render(createElement(ItemWorkspacePage, { itemId: 'item-1', refreshKey: 0, workspaces: [{ id: 'ws-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['items'], createdAt: '2026-07-10T00:00:00Z' }], onBack: vi.fn(), onOpenItem, onContentChanged: vi.fn() }));
    const branch = await screen.findByRole('button', { name: 'Select item branch' });
    fireEvent.click(branch);
    fireEvent.change(screen.getByRole('textbox', { name: 'Search branches' }), { target: { value: 'feature' } });
    await waitFor(() => expect(screen.getByRole('option', { name: 'feature/a' })).toBeInTheDocument());

    fireEvent.click(screen.getByRole('option', { name: 'feature/a' }));

    await waitFor(() => expect(onOpenItem).toHaveBeenCalledWith('item-feature'));
    expect(fetch).toHaveBeenCalledWith('/api/workspaces/ws-1/workstream/branch', expect.objectContaining({ method: 'POST', body: JSON.stringify({ branch: 'feature/a' }) }));
    expect(fetch).not.toHaveBeenCalledWith('/api/workspaces/ws-1/git/switch', expect.anything());
  });

  it('shows an empty branch state when the selected snapshot branch lacks the item', async () => {
    const onOpenItem = vi.fn();
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/items/item-1') return Promise.resolve(response({
        id: 'item-1',
        workspaceId: 'ws-1',
        workspaceName: 'Workspace',
        scope: 'api',
        branch: 'feature/current',
        identifier: 'DI-AI-08',
        title: 'Assistant Showcase',
        status: 'draft',
        tags: [],
        metadataSource: 'plan.yaml',
        itemPath: 'plans/api/DI-AI-08',
        sourceMode: 'working_tree',
        counts: { files: 1 },
        warnings: []
      }));
      if (url === '/api/items/item-1/files') return Promise.resolve(response([{ id: 'readme', name: 'README.md', path: 'README.md', type: 'file', editable: true, kind: 'markdown' }]));
      if (url === '/api/items/item-1/files/readme') return Promise.resolve(response({ id: 'readme', path: 'README.md', content: '# Assistant Showcase', hash: 'hash', kind: 'markdown', sizeBytes: 20, editable: true, truncated: false }));
      if (url === '/api/items/item-1/diff') return Promise.resolve(response({ diff: '' }));
      if (url === '/api/workspaces/ws-1/git/status') return Promise.resolve(response({ workspaceId: 'ws-1', branch: 'feature/current', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/ws-1/git/branches') return Promise.resolve(response({ workspaceId: 'ws-1', current: 'feature/current', branches: ['feature/current', 'main'] }));
      if (url === '/api/workspaces/ws-1/git/activity?path=plans%2Fapi%2FDI-AI-08&limit=8') return Promise.resolve(response([]));
      if (url === '/api/workspaces/ws-1/workstream/branch') return Promise.resolve(response({
        workspaceId: 'ws-1',
        branch: 'main',
        selectedBranch: 'main',
        branchRef: 'refs/heads/main',
        commit: 'abc',
        currentCheckoutBranch: 'feature/current',
        sourceMode: 'snapshot',
        mode: 'snapshot',
        editable: false,
        scannedAt: '',
        itemCount: 0,
        warnings: [],
        items: []
      }));
      if (url === '/api/items/item-1/jira') return Promise.resolve(response({ state: 'not_configured' }));
      return Promise.resolve(response({}));
    }));

    render(createElement(ItemWorkspacePage, { itemId: 'item-1', refreshKey: 0, workspaces: [{ id: 'ws-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '2026-07-10T00:00:00Z' }], onBack: vi.fn(), onOpenItem, onContentChanged: vi.fn() }));
    const branch = await screen.findByRole('button', { name: 'Select item branch' });
    fireEvent.click(branch);
    await waitFor(() => expect(screen.getByRole('option', { name: 'main' })).toBeInTheDocument());

    fireEvent.click(screen.getByRole('option', { name: 'main' }));

    expect(await screen.findByText('DI-AI-08 is not on main')).toBeInTheDocument();
    expect(screen.getByText(/snapshot -> feature\/current/)).toBeInTheDocument();
    expect(branch).toHaveTextContent('main');
    expect(onOpenItem).not.toHaveBeenCalled();
    expect(screen.queryByText('# Assistant Showcase')).not.toBeInTheDocument();
  });
});

function response(payload: unknown) {
  return {
    ok: true,
    json: async () => payload
  };
}
