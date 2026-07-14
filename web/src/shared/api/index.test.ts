import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, api } from '.';

describe('shared api facade', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('normalizes workspace sources', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [{ id: 'w1', name: 'Workspace', path: '/repo', baselineBranch: 'main', createdAt: '2026-06-20T00:00:00Z' }]
    }));

    await expect(api.workspaces()).resolves.toEqual([
      {
        id: 'w1',
        name: 'Workspace',
        path: '/repo',
        location: 'local_path',
        baselineBranch: 'main',
        createdAt: '2026-06-20T00:00:00Z',
        registrationMode: 'local_path',
        remoteUrl: '',
        clonePathManaged: false,
        sources: [],
        runtime: undefined
      }
    ]);
  });

	it('normalizes import previews and sends import selections', async () => {
		const fetchMock = vi.fn()
			.mockResolvedValueOnce({ ok: true, json: async () => ({
				sourcePath: '/source/workspaces.yaml', destinationPath: '/data/workspaces.yaml', sourceFingerprint: 'abc',
				candidates: [{ candidateKey: 'one', position: 1, status: 'valid', selected: true, workspace: { name: 'One', path: '/one', baselineBranch: 'main' } }],
				summary: { valid: 1 }
			}) })
			.mockResolvedValueOnce({ ok: true, json: async () => [{ candidateKey: 'one', status: 'indexed', workspace: { id: 'one', name: 'One', path: '/one', baselineBranch: 'main', registrationMode: 'existing_workspace', createdAt: '' }, scan: { workspaceId: 'one' } }] });
		vi.stubGlobal('fetch', fetchMock);

		await expect(api.previewWorkspaceImport('/source/workspaces.yaml')).resolves.toMatchObject({
			candidates: [{ candidateKey: 'one', selected: true, issues: [], workspace: { registrationMode: 'existing_workspace', sources: [] } }],
			summary: { valid: 1, invalid: 0, duplicate: 0, alreadyRegistered: 0 }
		});
		await expect(api.importWorkspaces({ sourcePath: '/source/workspaces.yaml', candidateKeys: ['one'] })).resolves.toMatchObject([
			{ candidateKey: 'one', status: 'indexed', workspace: { registrationMode: 'existing_workspace', sources: [] }, scan: { warnings: [] }, message: '' }
		]);
		expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workspaces/import-preview', expect.objectContaining({ method: 'POST', body: JSON.stringify({ sourcePath: '/source/workspaces.yaml' }) }));
		expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workspaces/import', expect.objectContaining({ method: 'POST', body: JSON.stringify({ sourcePath: '/source/workspaces.yaml', candidateKeys: ['one'] }) }));
	});

	it('treats file picker cancellation as an empty path response', async () => {
		const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ path: '' }) });
		vi.stubGlobal('fetch', fetchMock);
		await expect(api.selectYAMLFile()).resolves.toEqual({ path: '' });
		expect(fetchMock).toHaveBeenCalledWith('/api/system/select-file', expect.objectContaining({ method: 'POST' }));
	});

  it('normalizes Git status defaults', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ workspaceId: 'w1', branch: 'main' })
    }));

    await expect(api.gitStatus('w1')).resolves.toEqual({
      workspaceId: 'w1',
      branch: 'main',
      ahead: 0,
      behind: 0,
      dirty: false,
      conflicted: false,
      changes: []
    });
  });

  it('normalizes workspace branch responses', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ workspaceId: 'workspace/one', current: 'main', branches: null })
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.workspaceBranches('workspace/one')).resolves.toEqual({ workspaceId: 'workspace/one', current: 'main', branches: [] });
    expect(fetchMock).toHaveBeenCalledWith('/api/workspaces/workspace%2Fone/git/branches', expect.any(Object));
  });

  it('loads and normalizes a Workspace branch snapshot', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        workspaceId: 'workspace/one',
        branch: 'feature',
        selectedBranch: 'feature',
        branchRef: 'refs/heads/feature',
        commit: 'abc',
        currentCheckoutBranch: 'main',
        mode: 'snapshot',
        itemCount: 0,
        warnings: null,
        items: [{ id: 'item-1', workspaceId: 'workspace/one', workspaceName: 'Workspace', branch: 'feature', sourceMode: 'snapshot', title: 'Item', tags: null }]
      })
    });
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.loadWorkstreamBranch('workspace/one', { branch: 'feature', force: true })).resolves.toMatchObject({
      workspaceId: 'workspace/one',
      branch: 'feature',
      sourceMode: 'snapshot',
      mode: 'snapshot',
      currentCheckoutBranch: 'main',
      editable: false,
      warnings: [],
      items: [{ id: 'item-1', sourceMode: 'snapshot', editable: false, tags: [] }]
    });
    expect(fetchMock).toHaveBeenCalledWith('/api/workspaces/workspace%2Fone/workstream/branch', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ branch: 'feature', force: true })
    }));
  });

  it('normalizes workspace directory listings and encodes file paths', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ workspaceId: 'w1', entries: [{ id: 'one', name: 'one', path: 'one', type: 'directory' }] }) })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ path: 'docs/a b.md' }) });
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.workspaceTree('w1', '', true)).resolves.toEqual({
      workspaceId: 'w1', path: '', hiddenCount: 0,
      entries: [{ id: 'one', name: 'one', path: 'one', type: 'directory', hasChildren: false, ignored: false, hidden: false, editable: false }]
    });
    await api.workspaceFile('w1', 'docs/a b.md');
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workspaces/w1/tree?path=&includeIgnored=true', expect.any(Object));
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workspaces/w1/files?path=docs%2Fa%20b.md', expect.any(Object));
  });

  it('normalizes Explorer productivity responses', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => ({ truncated: 0 }) })
      .mockResolvedValueOnce({ ok: true, json: async () => [{ path: 'README.md', status: 'modified' }] })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ workspaceId: 'w1', path: 'docs/new.md', type: 'file' }) });
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.searchWorkspacePaths({ q: 'read me', workspaceId: 'w1', includeIgnored: true })).resolves.toEqual({ results: [], truncated: false });
    await expect(api.workspacePathGitStates('w1')).resolves.toEqual([{ path: 'README.md', status: 'modified', staged: false, conflict: false }]);
    await expect(api.createWorkspaceFile('w1', { parentPath: 'docs', name: 'new.md', content: '' })).resolves.toEqual({
      workspaceId: 'w1', path: 'docs/new.md', type: 'file', invalidatedPaths: [], refreshed: false
    });
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workspaces/files/search?q=read+me&workspaceId=w1&includeIgnored=true', expect.any(Object));
    expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/workspaces/w1/files', expect.objectContaining({ method: 'POST' }));
  });

  it('encodes and normalizes item and Explorer content searches', async () => {
		const fetchMock = vi.fn()
			.mockResolvedValueOnce({ ok: true, json: async () => ({ results: null, truncated: 1 }) })
			.mockResolvedValueOnce({ ok: true, json: async () => ({ results: [], filesVisited: 3, bytesRead: 42, skippedFiles: 1 }) });
		vi.stubGlobal('fetch', fetchMock);

		await expect(api.searchItemContent('item/one', { q: 'read me', caseSensitive: true })).resolves.toEqual({
			results: [], truncated: true, filesVisited: 0, bytesRead: 0, skippedFiles: 0
		});
		await expect(api.searchWorkspaceContent({ q: 'needle', mode: 'sources', workspaceId: 'w1', includeIgnored: true })).resolves.toEqual({
			results: [], truncated: false, filesVisited: 3, bytesRead: 42, skippedFiles: 1
		});
		expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/items/item%2Fone/content-search?q=read+me&caseSensitive=true', expect.any(Object));
		expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workspaces/files/content-search?q=needle&mode=sources&workspaceId=w1&includeIgnored=true', expect.any(Object));
	});

  it('normalizes audit and workspace health responses', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: async () => [{ id: 'event-1', time: '2026-06-20T00:00:00Z', operation: 'scan', status: 'unknown', message: 'done' }]
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ workspaceId: 'w1', checkedAt: '2026-06-20T00:00:00Z', summary: 'unknown' })
      });
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.auditEvents({ workspaceId: 'w1', limit: 5 })).resolves.toEqual([
      { id: 'event-1', time: '2026-06-20T00:00:00Z', operation: 'scan', status: 'success', message: 'done', paths: [], durationMs: 0 }
    ]);
    await expect(api.workspaceHealth('w1')).resolves.toEqual({
      workspaceId: 'w1', checkedAt: '2026-06-20T00:00:00Z', summary: 'ok', checks: []
    });
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/audit-events?workspaceId=w1&limit=5', expect.any(Object));
  });

  it('preserves recovery hints on API errors', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: async () => ({ error: 'File changed', recoveryHint: 'Reload the file.' })
    }));

    const error = await api.saveFile('item-1', 'README_md', { content: 'new' }).catch((caught) => caught);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ message: 'File changed', recoveryHint: 'Reload the file.' });
  });

  it('normalizes search, saved filter, and recent item responses', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => [{ id: 'one', type: 'unknown', title: 'One', route: '/items/one' }] })
      .mockResolvedValueOnce({ ok: true, json: async () => [{ id: 'filter', name: 'Drafts', route: '/workstream' }] })
      .mockResolvedValueOnce({ ok: true, json: async () => [{ itemId: 'one', workspaceId: 'w1', title: 'One', openedAt: '2026-06-20T00:00:00Z' }] });
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.search({ q: 'one', workspaceId: 'w1', limit: 5 })).resolves.toEqual([
      { id: 'one', type: 'item', title: 'One', subtitle: '', context: '', route: '/items/one', score: 0 }
    ]);
    await expect(api.savedFilters()).resolves.toEqual([{ id: 'filter', name: 'Drafts', route: '/workstream', filters: {} }]);
    await expect(api.recentItems()).resolves.toEqual([
      { itemId: 'one', workspaceId: 'w1', title: 'One', subtitle: '', route: '/items/one', openedAt: '2026-06-20T00:00:00Z' }
    ]);
  });

  it('coalesces identical reads while the first request is in flight', async () => {
    let resolveResponse!: (value: { ok: boolean; json: () => Promise<never[]> }) => void;
    const fetchMock = vi.fn().mockReturnValue(new Promise((resolve) => {
      resolveResponse = resolve;
    }));
    vi.stubGlobal('fetch', fetchMock);

    const first = api.savedFilters();
    const second = api.savedFilters();
    expect(fetchMock).toHaveBeenCalledTimes(1);

    resolveResponse({ ok: true, json: async () => [] });
    await expect(Promise.all([first, second])).resolves.toEqual([[], []]);
  });

  it('coalesces identical workstream branch loads', async () => {
    let resolveResponse!: (value: { ok: boolean; json: () => Promise<Record<string, unknown>> }) => void;
    const fetchMock = vi.fn().mockReturnValue(new Promise((resolve) => {
      resolveResponse = resolve;
    }));
    vi.stubGlobal('fetch', fetchMock);

    const first = api.loadWorkstreamBranch('w1', { branch: 'main' });
    const second = api.loadWorkstreamBranch('w1', { branch: 'main' });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    resolveResponse({ ok: true, json: async () => ({ workspaceId: 'w1', branch: 'main', items: [] }) });
    await Promise.all([first, second]);
  });
});
