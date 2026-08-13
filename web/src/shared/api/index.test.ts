import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, ApiResponseError, api, apiURL, isExtensionSurface, localAPIOrigin } from '.';

describe('shared api facade', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  it('resolves API URLs for normal and extension surfaces', () => {
    expect(isExtensionSurface('http:')).toBe(false);
    expect(isExtensionSurface('chrome-extension:')).toBe(true);
    expect(apiURL('/api/state')).toBe('/api/state');
    expect(apiURL('/api/state', true)).toBe('http://127.0.0.1:4317/api/state');
    expect(apiURL('https://example.com/api/state')).toBe('https://example.com/api/state');
  });

  it('normalizes local API origin override values', () => {
    expect(localAPIOrigin()).toBe('http://127.0.0.1:4317');
    localStorage.setItem('kodeStreamApiOrigin', ' http://127.0.0.1:9999/ ');
    expect(localAPIOrigin()).toBe('http://127.0.0.1:9999');
  });

  it('treats any health HTTP response as a reachable local server', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 503 }));

    await expect(api.localServerReachable()).resolves.toBe(true);
  });

  it('treats failed health fetch as an unreachable local server', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));

    await expect(api.localServerReachable()).resolves.toBe(false);
  });

  it('rejects an unexpected response media type without exposing its body', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('<html>proxy failure</html>', { status: 502, headers: { 'content-type': 'text/html' } })));
    await expect(api.state()).rejects.toBeInstanceOf(ApiResponseError);
  });

  it('rejects oversized JSON responses before parsing them', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(`{"value":"${'x'.repeat(2 * 1024 * 1024)}"}`, { status: 200, headers: { 'content-type': 'application/json' } })));
    await expect(api.state()).rejects.toMatchObject({ name: 'ApiResponseError', message: 'The server response is too large.' });
  });

  it('propagates abort signals without joining signal-free GET de-duplication', async () => {
    const controller = new AbortController();
    vi.stubGlobal('fetch', vi.fn().mockImplementation((_path, options: RequestInit) => new Promise((_resolve, reject) => options.signal?.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError'))))));
    const pending = api.workspaces(controller.signal);
    controller.abort();
    await expect(pending).rejects.toMatchObject({ name: 'AbortError' });
  });

  it('requires event-stream responses for workspace progress', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('not an event stream', { status: 200, headers: { 'content-type': 'text/plain' } })));
    await expect(api.createWorkspaceStream({ name: 'One', path: '/repo', baselineBranch: 'main', sources: [] }, vi.fn())).rejects.toBeInstanceOf(ApiResponseError);
  });

  it('preserves typed format failures on workspace stream error responses', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('<html>bad gateway</html>', { status: 502, headers: { 'content-type': 'text/html' } })));
    await expect(api.createWorkspaceStream({ name: 'One', path: '/repo', baselineBranch: 'main', sources: [] }, vi.fn())).rejects.toBeInstanceOf(ApiResponseError);
  });

  it('parses bounded workspace progress events and returns the final workspace', async () => {
    const stream = new ReadableStream({ start(controller) { controller.enqueue(new TextEncoder().encode('event: log\ndata: {"chunk":"cloning"}\n\nevent: result\ndata: {"workspace":{"id":"w1","name":"One","path":"/repo","baselineBranch":"main","sources":[],"createdAt":""}}\n\n')); controller.close(); } });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(stream, { status: 200, headers: { 'content-type': 'text/event-stream' } })));
    const progress = vi.fn();
    await expect(api.createWorkspaceStream({ name: 'One', path: '/repo', baselineBranch: 'main', sources: [] }, progress)).resolves.toMatchObject({ workspace: { id: 'w1' } });
    expect(progress).toHaveBeenCalledWith('cloning');
  });

  it('passes cancellation through workspace progress streaming', async () => {
    const controller = new AbortController();
    vi.stubGlobal('fetch', vi.fn((_path, init: RequestInit) => new Promise((_resolve, reject) => init.signal?.addEventListener('abort', () => reject(new DOMException('Aborted', 'AbortError'))))));
    const pending = api.createWorkspaceStream({ name: 'One', path: '/repo', baselineBranch: 'main', sources: [] }, vi.fn(), controller.signal);
    controller.abort();
    await expect(pending).rejects.toMatchObject({ name: 'AbortError' });
  });

	it('rejects an oversized unfinished SSE event without invoking progress callbacks', async () => {
		const stream = new ReadableStream({ start(controller) { controller.enqueue(new TextEncoder().encode(`event: log\ndata: ${'x'.repeat(256 * 1024)} `)); controller.close(); } });
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(stream, { status: 200, headers: { 'content-type': 'text/event-stream' } })));
		const progress = vi.fn();
		await expect(api.createWorkspaceStream({ name: 'One', path: '/repo', baselineBranch: 'main', sources: [] }, progress)).rejects.toMatchObject({ name: 'ApiResponseError', message: 'Workspace progress response is too large.' });
		expect(progress).not.toHaveBeenCalled();
	});

	it('does not deliver a late SSE progress callback after cancellation', async () => {
		let streamController: ReadableStreamDefaultController<Uint8Array> | undefined;
		const stream = new ReadableStream<Uint8Array>({ start(controller) { streamController = controller; } });
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(stream, { status: 200, headers: { 'content-type': 'text/event-stream' } })));
		const abort = new AbortController();
		const progress = vi.fn();
		const pending = api.createWorkspaceStream({ name: 'One', path: '/repo', baselineBranch: 'main', sources: [] }, progress, abort.signal);
		abort.abort();
		streamController?.enqueue(new TextEncoder().encode('event: log\ndata: {"chunk":"late"}\n\n'));
		streamController?.close();
		await expect(pending).rejects.toMatchObject({ name: 'AbortError' });
		expect(progress).not.toHaveBeenCalled();
	});

  it('pins snapshot file requests to the expected reviewed commit', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, json: async () => [] })
      .mockResolvedValueOnce({ ok: true, json: async () => ({ id: 'README_md', path: 'README.md', content: '# Review' }) });
    vi.stubGlobal('fetch', fetchMock);

    await api.files('snapshot/item', 'commit/one');
    await api.file('snapshot/item', 'README md', 'commit/one');

    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/items/snapshot%2Fitem/files?expectedCommit=commit%2Fone', expect.any(Object));
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/items/snapshot%2Fitem/files/README%20md?expectedCommit=commit%2Fone', expect.any(Object));
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
		accessMode: undefined,
        baselineBranch: 'main',
        createdAt: '2026-06-20T00:00:00Z',
        registrationMode: 'local_path',
        remoteUrl: '',
        clonePathManaged: false,
		managedCloneRoot: '',
		managedCloneId: '',
		managedCloneVerified: false,
        sources: [],
        runtime: undefined
      }
    ]);
  });

  it('normalizes AI settings template args', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        defaultProvider: 'codex',
        providers: { codex: { enabled: true, executable: 'codex', args: null } },
        terminals: null
      })
    }));

    await expect(api.aiSettings()).resolves.toEqual({
      defaultProvider: 'codex',
      defaultTerminal: '',
      providers: { codex: { enabled: true, executable: 'codex', args: [] } },
      terminals: {}
    });
  });

	it('sends checkout-scoped Canvas, placement, and viewport contracts', async () => {
		const projection = { layout: { id: 'layout-1' }, nodes: [], connections: [], unplaced: [] };
		const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => projection });
		vi.stubGlobal('fetch', fetchMock);
		await api.resolveDefaultCanvas('workspace/one');
		await api.patchCanvasPlacements('layout/one', [{ nodeId: 'plan:1', entityRef: { kind: 'plan', workspaceId: 'workspace/one', itemId: '1', itemPath: 'plans/1', branchKey: 'feature/one' }, position: { x: 1, y: 2 }, collapsed: false, expectedRevision: 3 }]);
		await api.patchCanvasViewport('layout/one', 4, { x: 5, y: 6, zoom: 1.2 });
		expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/canvas/default', expect.objectContaining({ method: 'POST', body: JSON.stringify({ workspaceId: 'workspace/one' }) }));
		expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/canvas/layouts/layout%2Fone/placements', expect.objectContaining({ method: 'PATCH' }));
		expect(fetchMock).toHaveBeenNthCalledWith(3, '/api/canvas/layouts/layout%2Fone/viewport', expect.objectContaining({ method: 'PATCH', body: JSON.stringify({ expectedVersion: 4, viewport: { x: 5, y: 6, zoom: 1.2 } }) }));
	});

	it('preserves structured Canvas conflict metadata', async () => {
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 409, json: async () => ({ error: 'conflict', code: 'placement_conflict', nodeIds: ['plan:1'] }) }));
		await expect(api.patchCanvasPlacements('layout', [])).rejects.toMatchObject({ name: 'ApiError', code: 'placement_conflict', nodeIds: ['plan:1'], status: 409 });
	});

  it('scopes provider capability discovery to the selected workspace', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ provider: 'codex', skills: [], agents: [], supportsNativeSelection: false, supportsPromptFallback: true })
    });
    vi.stubGlobal('fetch', fetchMock);

    await api.aiProviderCapabilities('codex/local', { workspaceId: 'workspace/one' });

    expect(fetchMock).toHaveBeenCalledWith('/api/ai/providers/codex%2Flocal/capabilities?workspaceId=workspace%2Fone', expect.any(Object));
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
		await expect(api.importWorkspaces({ sourcePath: '/source/workspaces.yaml', sourceFingerprint: 'abc', candidateKeys: ['one'] })).resolves.toMatchObject([
			{ candidateKey: 'one', status: 'indexed', workspace: { registrationMode: 'existing_workspace', sources: [] }, scan: { warnings: [] }, message: '' }
		]);
		expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/workspaces/import-preview', expect.objectContaining({ method: 'POST', body: JSON.stringify({ sourcePath: '/source/workspaces.yaml' }) }));
		expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/workspaces/import', expect.objectContaining({ method: 'POST', body: JSON.stringify({ sourcePath: '/source/workspaces.yaml', sourceFingerprint: 'abc', candidateKeys: ['one'] }) }));
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
        json: async () => [{ id: 'event-1', time: '2026-06-20T00:00:00Z', ownerUserId: 'owner-1', actorUserId: 'actor-1', operation: 'scan', status: 'unknown', message: 'done' }]
      })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({ workspaceId: 'w1', checkedAt: '2026-06-20T00:00:00Z', summary: 'unknown' })
      });
    vi.stubGlobal('fetch', fetchMock);

    await expect(api.auditEvents({ workspaceId: 'w1', limit: 5 })).resolves.toEqual([
      { id: 'event-1', time: '2026-06-20T00:00:00Z', ownerUserId: 'owner-1', actorUserId: 'actor-1', operation: 'scan', status: 'success', message: 'done', paths: [], durationMs: 0 }
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
