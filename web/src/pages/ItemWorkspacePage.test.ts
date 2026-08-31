import { createElement } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ItemWorkspacePage } from './ItemWorkspacePage';
import { parseGitDiff } from '../shared/domain/diff';

vi.mock('./WorkstreamExplorer', () => ({
  WorkstreamExplorer: ({ location, embeddedHeaderContent, leftPanelContent, rightPanel }: { location?: { workspaceId?: string; path?: string; mode?: string }; embeddedHeaderContent?: ReturnType<typeof createElement>; leftPanelContent?: ReturnType<typeof createElement>; rightPanel?: { title?: ReturnType<typeof createElement>; content?: ReturnType<typeof createElement> } }) => createElement(
    'div',
    undefined,
    embeddedHeaderContent,
    leftPanelContent,
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
  it('removes operational controls when a stale snapshot route is rejected', async () => {
    const requests: string[] = [];
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      requests.push(url);
      if (url === '/api/items/item-1') return Promise.resolve(response({
        id: 'item-1', workspaceId: 'ws-1', workspaceName: 'Workspace', scope: 'platform', branch: 'main',
        identifier: 'PM-038', title: 'Checkout item', status: 'draft', tags: [], metadataSource: 'plan.yaml',
        itemPath: 'plans/platform/PM-038', counts: { files: 0 }, warnings: []
      }));
      if (url === '/api/items/item-1/files') return Promise.resolve(response([]));
      if (url === '/api/items/item-1/diff') return Promise.resolve(response({ diff: '' }));
      if (url === '/api/items/snapshot-item') return Promise.resolve(errorResponse(409, 'snapshot item is available only in Branch Review', 'snapshot_review_only'));
      return Promise.resolve(response({}));
    }));
    const props = { refreshKey: 0, workspaces: [{ id: 'ws-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '2026-07-10T00:00:00Z' }], onBack: vi.fn(), onOpenItem: vi.fn() };
    const view = render(createElement(ItemWorkspacePage, { ...props, itemId: 'item-1' }));
    expect(await screen.findByRole('button', { name: 'Plan' })).toBeInTheDocument();

    view.rerender(createElement(ItemWorkspacePage, { ...props, itemId: 'snapshot-item' }));
    expect(await screen.findByText('snapshot item is available only in Branch Review')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Plan' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'New file' })).not.toBeInTheDocument();
    expect(requests).not.toContain('/api/items/snapshot-item/files');
    expect(requests).not.toContain('/api/items/snapshot-item/diff');
  });

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

    expect(await screen.findByRole('button', { name: 'Plan' })).toHaveClass('active');
    expect(await screen.findByText('Checkout: main')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Explorer' }));

    await waitFor(() => expect(screen.getByTestId('embedded-explorer')).toHaveTextContent('ws-1|items/platform/PM-012/README.md|all'));
    expect(screen.getByRole('heading', { name: 'Work Item' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Info/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Git' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Quality/i }));
    expect(screen.getByLabelText('Quality')).toBeInTheDocument();
  });

  it('opens a Git changed path in the main explorer without leaving the Git panel', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === '/api/items/item-1') {
        return Promise.resolve(response({
          id: 'item-1',
          workspaceId: 'ws-1',
          workspaceName: 'Workspace',
          scope: 'api',
          branch: 'main',
          identifier: 'DI-170',
          title: 'Custom assortment',
          status: 'draft',
          tags: [],
          metadataSource: 'plan.yaml',
          itemPath: 'plans/api/DI-170',
          counts: { files: 1 },
          warnings: []
        }));
      }
      if (url === '/api/items/item-1/files') return Promise.resolve(response([{ id: 'readme', name: 'README.md', path: 'README.md', type: 'file', editable: true, kind: 'markdown' }]));
      if (url === '/api/items/item-1/files/readme') return Promise.resolve(response({ id: 'readme', path: 'README.md', content: '# DI-170', hash: 'hash', kind: 'markdown', sizeBytes: 8, editable: true, truncated: false }));
      if (url === '/api/items/item-1/diff') return Promise.resolve(response({ diff: '' }));
      if (url === '/api/workspaces/ws-1/git/status') return Promise.resolve(response({
        workspaceId: 'ws-1',
        branch: 'main',
        ahead: 0,
        behind: 0,
        dirty: true,
        conflicted: false,
        changes: [{ path: 'plans/api/DI-170/README.md', status: 'modified', staged: false }]
      }));
      if (url === '/api/workspaces/ws-1/git/branches') return Promise.resolve(response({ workspaceId: 'ws-1', current: 'main', branches: ['main'] }));
      if (url === '/api/workspaces/ws-1/git/activity?path=plans%2Fapi%2FDI-170&limit=8') return Promise.resolve(response([]));
      if (url === '/api/items/item-1/jira') return Promise.resolve(response({ state: 'not_configured' }));
      return Promise.resolve(response({}));
    }));

    render(createElement(ItemWorkspacePage, { itemId: 'item-1', refreshKey: 0, workspaces: [{ id: 'ws-1', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['plans'], createdAt: '2026-07-10T00:00:00Z' }], onBack: vi.fn(), onOpenItem: vi.fn(), onContentChanged: vi.fn() }));

    const gitTab = await screen.findByRole('button', { name: 'Git' });
    fireEvent.click(gitTab);
    fireEvent.click(await screen.findByRole('button', { name: 'plans/api/DI-170/README.md' }));

    await waitFor(() => expect(screen.getByTestId('embedded-explorer')).toHaveTextContent('ws-1|plans/api/DI-170/README.md|all'));
    expect(gitTab).toHaveClass('active');
    expect(screen.getByText('0 ahead')).toBeInTheDocument();
  });

  it('selects discovered automation specs and starts an automation verification job', async () => {
    const requests: Array<{ url: string; method: string; body?: unknown }> = [];
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      requests.push({ url, method: init?.method ?? 'GET', body: init?.body ? JSON.parse(String(init.body)) : undefined });
      if (url === '/api/items/item-1') {
        return Promise.resolve(response({
          id: 'item-1',
          workspaceId: 'ws-1',
          workspaceName: 'Workspace',
          scope: 'platform',
          branch: 'main',
          identifier: 'PM-029',
          title: 'Automation runner',
          status: 'draft',
          tags: [],
          metadataSource: 'plan.yaml',
          itemPath: 'plans/platform/PM-029',
          counts: { files: 1 },
          warnings: []
        }));
      }
      if (url === '/api/items/item-1/files') return Promise.resolve(response([]));
      if (url === '/api/items/item-1/diff') return Promise.resolve(response({ diff: '' }));
      if (url === '/api/workspaces/ws-1/git/status') return Promise.resolve(response({ workspaceId: 'ws-1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/ws-1/git/branches') return Promise.resolve(response({ workspaceId: 'ws-1', current: 'main', branches: ['main'] }));
      if (url === '/api/workspaces/ws-1/git/activity?path=plans%2Fplatform%2FPM-029&limit=8') return Promise.resolve(response([]));
      if (url === '/api/items/item-1/jira') return Promise.resolve(response({ state: 'not_configured' }));
      if (url === '/api/items/item-1/verification-tests' && (init?.method ?? 'GET') === 'GET') {
        return Promise.resolve(response({
          selection: { selectedSpecs: [], environment: 'local' },
          discoveredSpecs: [{ path: 'cypress/e2e/create.cy.ts', runner: 'cypress', sourcePath: 'plans/PM-029/test-plan.md' }]
        }));
      }
      if (url === '/api/items/item-1/verification-tests' && init?.method === 'PUT') {
        const body = init.body ? JSON.parse(String(init.body)) : {};
        return Promise.resolve(response({
          selection: { selectedSpecs: body.selectedSpecs ?? ['cypress/e2e/create.cy.ts'], environment: body.environment ?? 'local', displayMode: body.displayMode ?? 'silent', updatedAt: '2026-07-11T00:00:00Z' },
          discoveredSpecs: [{ path: 'cypress/e2e/create.cy.ts', runner: 'cypress', sourcePath: 'plans/PM-029/test-plan.md' }]
        }));
      }
      if (url === '/api/workspaces/ws-1/verification-jobs' && init?.method === 'POST') {
        return Promise.resolve(response({
          id: 'verify-1',
          workspaceId: 'ws-1',
          mode: 'automation',
          profile: 'smoke',
          environment: 'local',
          selectedSpecs: ['cypress/e2e/create.cy.ts'],
          status: 'passed',
          exitCode: 0,
          steps: [],
          artifacts: [
            { kind: 'automation_log', root: 'workspace', path: '.artifacts/verification/verify-1/automation.log', sizeBytes: 1234, createdAt: '2026-07-12T12:00:00Z' },
				{ kind: 'runtime_log', root: 'workspace', path: '.artifacts/verification/verify-1/runtime.log', sizeBytes: 456, createdAt: '2026-07-12T12:00:01Z' },
				{ kind: 'playwright_report', root: 'automation', path: 'reports/result.txt', sizeBytes: 10, createdAt: '2026-07-12T12:00:01Z' },
				{ kind: 'playwright_video', root: 'automation', path: 'videos/result.txt', sizeBytes: 10, createdAt: '2026-07-12T12:00:01Z' }
          ]
        }));
      }
      return Promise.resolve(response({}));
    }));

    render(createElement(ItemWorkspacePage, {
      itemId: 'item-1',
      refreshKey: 0,
      workspaces: [{
        id: 'ws-1',
        name: 'Workspace',
        path: '/repo',
        baselineBranch: 'main',
        sources: ['plans'],
        createdAt: '2026-07-10T00:00:00Z',
        runtime: {
          type: 'custom',
          commands: { up: 'true', down: 'true', verify: { smoke: 'true' } },
          automation: { enabled: true, repositoryPath: '/automation', runner: 'cypress', defaultEnvironment: 'local', commandTemplate: 'npx cypress run --spec "{specs}"', artifactPaths: [] }
        }
      }],
      onBack: vi.fn(),
      onOpenItem: vi.fn(),
      onContentChanged: vi.fn()
    }));

    /*
     * The Info/Jira/Quality tab bar renders in both branches of the page's
     * `plan && workspaceConfig` gate, and the two branches are different
     * elements at that position, so when the gate flips React replaces every
     * node in the bar. A tab captured before the flip is detached by the time
     * it is clicked, and the click dispatches into nothing.
     *
     * Waiting for the explorer proves the gate is true, so the tab queried
     * after it is the one that survives. Asserting `active` then pins that the
     * click landed, rather than leaving a downstream query to time out.
     */
    await screen.findByTestId('embedded-explorer');
    fireEvent.click(screen.getByRole('button', { name: /Quality/i }));
    await waitFor(() => expect(screen.getByRole('button', { name: /Quality/i })).toHaveClass('active'));
    const runAutomation = await screen.findByRole('button', { name: 'Run automation tests' });
    expect(runAutomation).toBeDisabled();
    expect(await screen.findByText('Suggested specs')).toBeInTheDocument();
    expect(screen.getByText('cypress/e2e/create.cy.ts')).toBeInTheDocument();
    expect(screen.getByText('cypress · plans/PM-029/test-plan.md')).toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: 'Select' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Visible browser' }));
    await waitFor(() => expect(screen.getByRole('button', { name: 'Run automation tests' })).not.toBeDisabled());
    fireEvent.click(screen.getByRole('button', { name: 'Run automation tests' }));

    await waitFor(() => expect(requests.some((request) => request.url === '/api/workspaces/ws-1/verification-jobs' && request.method === 'POST')).toBe(true));
    const jobRequest = requests.find((request) => request.url === '/api/workspaces/ws-1/verification-jobs' && request.method === 'POST');
    expect(jobRequest?.body).toMatchObject({ mode: 'automation', environment: 'local', displayMode: 'visible', selectedSpecs: ['cypress/e2e/create.cy.ts'] });
    expect(await screen.findByText('automation · passed')).toBeInTheDocument();
    expect(screen.getByText('Automation log')).toBeInTheDocument();
    expect(screen.getByText('Runtime setup log')).toBeInTheDocument();
		fireEvent.click(screen.getAllByRole('button', { name: 'Preview' })[0]);
		await waitFor(() => expect(requests.some((request) => request.url.includes('path=.artifacts%2Fverification%2Fverify-1%2Fautomation.log'))).toBe(true));
		const openButtons = screen.getAllByRole('button', { name: 'Open' });
		fireEvent.click(openButtons[2]);
		fireEvent.click(openButtons[3]);
		await waitFor(() => expect(requests.filter((request) => request.url === '/api/system/open-path').map((request) => request.body)).toEqual(expect.arrayContaining([{ path: '/automation/reports/result.txt' }, { path: '/automation/videos/result.txt' }])));
  });

  it('browses multiple automation specs from the registered automation repository', async () => {
    const requests: Array<{ url: string; method: string; body?: unknown }> = [];
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const body = init?.body ? JSON.parse(String(init.body)) : undefined;
      requests.push({ url, method: init?.method ?? 'GET', body });
      if (url === '/api/items/item-1') {
        return Promise.resolve(response({
          id: 'item-1',
          workspaceId: 'ws-1',
          workspaceName: 'Workspace',
          scope: 'platform',
          branch: 'main',
          identifier: 'PM-029',
          title: 'Automation runner',
          status: 'draft',
          tags: [],
          metadataSource: 'plan.yaml',
          itemPath: 'plans/platform/PM-029',
          counts: { files: 1 },
          warnings: []
        }));
      }
      if (url === '/api/items/item-1/files') return Promise.resolve(response([]));
      if (url === '/api/items/item-1/diff') return Promise.resolve(response({ diff: '' }));
      if (url === '/api/workspaces/ws-1/git/status') return Promise.resolve(response({ workspaceId: 'ws-1', branch: 'main', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] }));
      if (url === '/api/workspaces/ws-1/git/branches') return Promise.resolve(response({ workspaceId: 'ws-1', current: 'main', branches: ['main'] }));
      if (url === '/api/workspaces/ws-1/git/activity?path=plans%2Fplatform%2FPM-029&limit=8') return Promise.resolve(response([]));
      if (url === '/api/items/item-1/jira') return Promise.resolve(response({ state: 'not_configured' }));
      if (url === '/api/items/item-1/verification-tests' && (init?.method ?? 'GET') === 'GET') {
        return Promise.resolve(response({ selection: { selectedSpecs: [], environment: 'local' }, discoveredSpecs: [] }));
      }
      if (url === '/api/items/item-1/verification-tests' && init?.method === 'PUT') {
        return Promise.resolve(response({ selection: { ...body, updatedAt: '2026-07-11T00:00:00Z' }, discoveredSpecs: [] }));
      }
      if (url === '/api/workspaces/automation-ws/tree?path=') {
        return Promise.resolve(response({
          workspaceId: 'automation-ws',
          path: '',
          hiddenCount: 0,
          entries: [{ id: 'cypress', name: 'cypress', path: 'cypress', type: 'directory', hasChildren: true, ignored: false, hidden: false, editable: false }]
        }));
      }
      if (url === '/api/workspaces/automation-ws/tree?path=cypress') {
        return Promise.resolve(response({
          workspaceId: 'automation-ws',
          path: 'cypress',
          hiddenCount: 0,
          entries: [{ id: 'e2e', name: 'e2e', path: 'cypress/e2e', type: 'directory', hasChildren: true, ignored: false, hidden: false, editable: false }]
        }));
      }
      if (url === '/api/workspaces/automation-ws/tree?path=cypress%2Fe2e') {
        return Promise.resolve(response({
          workspaceId: 'automation-ws',
          path: 'cypress/e2e',
          hiddenCount: 0,
          entries: [
            { id: 'create', name: 'create.cy.ts', path: 'cypress/e2e/create.cy.ts', type: 'file', hasChildren: false, ignored: false, hidden: false, editable: true },
            { id: 'edit', name: 'edit.cy.ts', path: 'cypress/e2e/edit.cy.ts', type: 'file', hasChildren: false, ignored: false, hidden: false, editable: true },
            { id: 'notes', name: 'notes.md', path: 'cypress/e2e/notes.md', type: 'file', hasChildren: false, ignored: false, hidden: false, editable: true }
          ]
        }));
      }
      return Promise.resolve(response({}));
    }));

    render(createElement(ItemWorkspacePage, {
      itemId: 'item-1',
      refreshKey: 0,
      workspaces: [
        {
          id: 'ws-1',
          name: 'Workspace',
          path: '/repo',
          baselineBranch: 'main',
          sources: ['plans'],
          createdAt: '2026-07-10T00:00:00Z',
          runtime: {
            type: 'custom',
            commands: { up: 'true', down: 'true', verify: { smoke: 'true' } },
            automation: { enabled: true, repositoryPath: '/automation', runner: 'cypress', defaultEnvironment: 'local', commandTemplate: 'npx cypress run --spec "{specs}"', artifactPaths: [] }
          }
        },
        { id: 'automation-ws', name: 'Automation', path: '/automation', baselineBranch: 'main', sources: [], createdAt: '2026-07-10T00:00:00Z' }
      ],
      onBack: vi.fn(),
      onOpenItem: vi.fn(),
      onContentChanged: vi.fn()
    }));

    await waitFor(() => expect(screen.getByTestId('embedded-explorer')).toHaveTextContent('ws-1|plans/platform/PM-029|all'));
    fireEvent.click(screen.getByRole('button', { name: /Quality/i }));
    await waitFor(() => expect(screen.getByRole('button', { name: /Quality/i })).toHaveClass('active'));
    fireEvent.click(await screen.findByRole('button', { name: 'Browse' }));
    fireEvent.click(await screen.findByRole('button', { name: 'cypress' }));
    fireEvent.click(await screen.findByRole('button', { name: 'e2e' }));
    fireEvent.click(await screen.findByLabelText('cypress/e2e/create.cy.ts'));
    fireEvent.click(await screen.findByLabelText('cypress/e2e/edit.cy.ts'));
    fireEvent.click(screen.getByRole('button', { name: 'Add 2 specs' }));

    await waitFor(() => expect(requests.some((request) => request.url === '/api/items/item-1/verification-tests' && request.method === 'PUT')).toBe(true));
    const saveRequest = requests.filter((request) => request.url === '/api/items/item-1/verification-tests' && request.method === 'PUT').at(-1);
    expect(saveRequest?.body).toMatchObject({ selectedSpecs: ['cypress/e2e/create.cy.ts', 'cypress/e2e/edit.cy.ts'], environment: 'local', displayMode: 'silent' });
    expect(await screen.findByText('cypress/e2e/create.cy.ts')).toBeInTheDocument();
    expect(await screen.findByText('cypress/e2e/edit.cy.ts')).toBeInTheDocument();
    expect(screen.queryByText('cypress/e2e/notes.md')).not.toBeInTheDocument();
  });
});

function response(payload: unknown) {
  return {
    ok: true,
    json: async () => payload
  };
}

function errorResponse(status: number, error: string, code: string) {
  return {
    ok: false,
    status,
    json: async () => ({ error, code })
  };
}
