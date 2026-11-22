import type { AISettings, AILaunchTemplate, AuditEvent, WorkstreamBranchLoadResult, GitActivityEntry, GitOperationResult, GitStatus, HealthCheck, RecentItem, SavedFilter, SearchResult, ItemDetail, ItemVerificationTests, ItemSummary, WorkspaceConfig, WorkspaceCreateResult, WorkspaceHealth, WorkspaceImportPreview, WorkspaceImportResult, WorkspaceDirectoryListing, WorkspacePathGitState, WorkspacePathMutationResult, WorkspaceContentSearchResponse, WorkspaceTreeEntry, SystemConfigPaths, StorageStatus, StorageSyncResult, HarnessRunDetail, HarnessSettings, HarnessTask } from '../../lib/types';
import { normalizeSources } from '../../lib/sources';

export function normalizeHarnessTask(task: HarnessTask): HarnessTask {
  return { ...task, scoreReasons: task.scoreReasons ?? [], dependsOn: task.dependsOn ?? [], source: { ...task.source, labels: task.source?.labels ?? [] } };
}

export function normalizeHarnessRunDetail(detail: HarnessRunDetail): HarnessRunDetail {
  return { ...detail, task: normalizeHarnessTask(detail.task), stages: detail.stages ?? [], agentRuns: detail.agentRuns ?? [], approvals: detail.approvals ?? [] };
}

export function knowledgeURL(workspaceId: string, root: string, resource: 'pages' | 'graph' | 'rescan'): string {
	return `/api/knowledge/wikis/${encodeURIComponent(workspaceId)}/${encodeURIComponent(root)}/${resource}`;
}

export function contentSearchQuery(params: { q: string; caseSensitive?: boolean }): URLSearchParams {
	const query = new URLSearchParams({ q: params.q });
	if (params.caseSensitive) query.set('caseSensitive', 'true');
	return query;
}

export function normalizeContentSearchResponse(response: WorkspaceContentSearchResponse): WorkspaceContentSearchResponse {
	return {
		...response,
		results: Array.isArray(response.results) ? response.results : [],
		truncated: Boolean(response.truncated),
		filesVisited: Number(response.filesVisited) || 0,
		bytesRead: Number(response.bytesRead) || 0,
		skippedFiles: Number(response.skippedFiles) || 0
	};
}

export function normalizeWorkspace(workspace: WorkspaceConfig): WorkspaceConfig {
  return {
    ...workspace,
		location: workspace.location === 'cloud_agent' || workspace.location === 'cloud_remote_snapshot' ? workspace.location : 'local_path',
		accessMode: workspace.accessMode === 'remote_snapshot' ? 'remote_snapshot' : workspace.accessMode === 'agent_backed' ? 'agent_backed' : undefined,
		registrationMode: workspace.registrationMode === 'remote_clone' || workspace.registrationMode === 'existing_workspace' ? workspace.registrationMode : 'local_path',
    remoteUrl: workspace.remoteUrl ?? '',
    clonePathManaged: Boolean(workspace.clonePathManaged),
    managedCloneRoot: workspace.managedCloneRoot ?? '',
    managedCloneId: workspace.managedCloneId ?? '',
    managedCloneVerified: Boolean(workspace.managedCloneVerified),
    sources: normalizeSources(workspace.sources),
    runtime: workspace.runtime ? {
      ...workspace.runtime,
      rebuildPolicy: workspace.runtime.rebuildPolicy ?? 'changed-only',
      healthChecks: Array.isArray(workspace.runtime.healthChecks) ? workspace.runtime.healthChecks : [],
      artifacts: { paths: Array.isArray(workspace.runtime.artifacts?.paths) ? workspace.runtime.artifacts?.paths : [] },
      automation: workspace.runtime.automation ? {
        ...workspace.runtime.automation,
        enabled: Boolean(workspace.runtime.automation.enabled),
        runner: workspace.runtime.automation.runner ?? 'cypress',
        defaultEnvironment: workspace.runtime.automation.defaultEnvironment ?? 'local',
        artifactPaths: Array.isArray(workspace.runtime.automation.artifactPaths) ? workspace.runtime.automation.artifactPaths : []
      } : undefined
    } : undefined
  };
}

export function normalizeItemVerificationTests(input: ItemVerificationTests): ItemVerificationTests {
  return {
    selection: {
      selectedSpecs: Array.isArray(input.selection?.selectedSpecs) ? input.selection.selectedSpecs : [],
      environment: input.selection?.environment ?? '',
      displayMode: input.selection?.displayMode === 'visible' ? 'visible' : 'silent',
      updatedAt: input.selection?.updatedAt
    },
    discoveredSpecs: Array.isArray(input.discoveredSpecs) ? input.discoveredSpecs : []
  };
}

export function normalizeSystemConfigPaths(input: SystemConfigPaths): SystemConfigPaths {
  return {
    dataDir: input.dataDir ?? '',
    defaultDataDir: input.defaultDataDir ?? '',
    cloneRootDir: input.cloneRootDir ?? '',
		registryFile: input.registryFile ?? '',
		dataDirEnvironmentLocked: Boolean(input.dataDirEnvironmentLocked),
    restartRequired: Boolean(input.restartRequired)
  };
}

export function normalizeHarnessSettings(input: HarnessSettings): HarnessSettings {
  return {
    ...input,
    sources: input.sources ?? [],
    workspaces: (input.workspaces ?? []).map((workspace) => ({ ...workspace, verificationCommands: workspace.verificationCommands ?? [], scope: workspace.scope ?? [], models: workspace.models ?? {}, procedure: workspace.procedure ?? {} }))
  };
}

export function normalizeAISettings(input: AISettings): AISettings {
  return {
    defaultProvider: input.defaultProvider ?? '',
    defaultTerminal: input.defaultTerminal ?? '',
    providers: normalizeAILaunchTemplates(input.providers),
    terminals: normalizeAILaunchTemplates(input.terminals),
    workspaceProviders: input.workspaceProviders ?? {}
  };
}

export function normalizeAILaunchTemplates(input: Record<string, AILaunchTemplate> | null | undefined): Record<string, AILaunchTemplate> {
  return Object.fromEntries(Object.entries(input ?? {}).map(([id, template]) => [id, {
    enabled: Boolean(template?.enabled),
    executable: template?.executable ?? '',
    args: Array.isArray(template?.args) ? template.args : []
  }]));
}

export function normalizeStorageStatus(input: StorageStatus): StorageStatus {
  return {
    ...input,
    mode: input.mode === 'cloud' ? 'cloud' : 'local',
    storageOption: input.storageOption === 'datadir' ? 'datadir' : 'database',
    storageDriver: input.storageDriver === 'file' || input.storageDriver === 'postgres' ? input.storageDriver : 'sqlite',
    environmentLocked: Boolean(input.environmentLocked),
    storageOptionEnv: input.storageOptionEnv ?? 'KODE_STREAM_STORAGE_OPTION',
    storageDriverEnv: input.storageDriverEnv ?? 'KODE_STREAM_STORAGE_DRIVER',
    dataDir: input.dataDir ?? '',
    databasePath: input.databasePath ?? '',
    databaseUrlConfigured: Boolean(input.databaseUrlConfigured),
    database: input.database ? {
      ...input.database,
      driver: input.database.driver === 'postgres' || input.database.driver === 'file' ? input.database.driver : 'sqlite',
      ok: Boolean(input.database.ok),
      migrationVersion: Number(input.database.migrationVersion) || 0
    } : undefined
  };
}

export function normalizeStorageSyncResult(input: StorageSyncResult): StorageSyncResult {
  return {
    ...input,
    ok: Boolean(input.ok),
    direction: input.direction === 'database_to_datadir' ? 'database_to_datadir' : 'datadir_to_database',
    backupPath: input.backupPath ?? '',
    summary: input.summary ?? {},
    warnings: Array.isArray(input.warnings) ? input.warnings : [],
    skippedStores: Array.isArray(input.skippedStores) ? input.skippedStores : []
  };
}

export function normalizeWorkspaceImportPreview(input: WorkspaceImportPreview): WorkspaceImportPreview {
	return {
		...input,
		sourcePath: input.sourcePath ?? '',
		destinationPath: input.destinationPath ?? '',
		sourceFingerprint: input.sourceFingerprint ?? '',
		candidates: (Array.isArray(input.candidates) ? input.candidates : []).map((candidate) => ({
			...candidate,
			position: Number(candidate.position) || 0,
			workspace: {
				...candidate.workspace,
				registrationMode: 'existing_workspace',
				sources: normalizeSources(candidate.workspace?.sources)
			},
			issues: Array.isArray(candidate.issues) ? candidate.issues : [],
			selected: candidate.status === 'valid' && Boolean(candidate.selected)
		})),
		summary: {
			valid: Number(input.summary?.valid) || 0,
			invalid: Number(input.summary?.invalid) || 0,
			duplicate: Number(input.summary?.duplicate) || 0,
			alreadyRegistered: Number(input.summary?.alreadyRegistered) || 0
		}
	};
}

export function normalizeWorkspaceImportResult(result: WorkspaceImportResult): WorkspaceImportResult {
	return {
		...result,
		workspace: result.workspace ? normalizeWorkspace(result.workspace) : undefined,
		scan: result.scan ? { ...result.scan, warnings: Array.isArray(result.scan.warnings) ? result.scan.warnings : [] } : undefined,
		message: result.message ?? ''
	};
}

export function isWorkspaceCreateResult(input: WorkspaceConfig | WorkspaceCreateResult): input is WorkspaceCreateResult {
  return typeof (input as WorkspaceCreateResult).workspace === 'object' && (input as WorkspaceCreateResult).workspace !== null;
}

export function normalizeWorkspaceDirectoryListing(listing: WorkspaceDirectoryListing): WorkspaceDirectoryListing {
  return {
    ...listing,
    path: listing.path ?? '',
    hiddenCount: listing.hiddenCount ?? 0,
    entries: (Array.isArray(listing.entries) ? listing.entries : []).map(normalizeWorkspaceTreeEntry)
  };
}

export function normalizeWorkspaceTreeEntry(entry: WorkspaceTreeEntry): WorkspaceTreeEntry {
  return {
    ...entry,
    hasChildren: Boolean(entry.hasChildren),
    ignored: Boolean(entry.ignored),
    hidden: Boolean(entry.hidden),
    editable: Boolean(entry.editable)
  };
}

export function normalizeWorkspacePathGitState(state: WorkspacePathGitState): WorkspacePathGitState {
  return { ...state, staged: Boolean(state.staged), conflict: Boolean(state.conflict) };
}

export function normalizeWorkspacePathMutationResult(result: WorkspacePathMutationResult): WorkspacePathMutationResult {
  return { ...result, invalidatedPaths: Array.isArray(result.invalidatedPaths) ? result.invalidatedPaths : [], refreshed: Boolean(result.refreshed) };
}

export function normalizeItem(item: ItemSummary): ItemSummary {
  return {
    ...item,
    sourceMode: item.sourceMode === 'snapshot' ? 'snapshot' : 'working_tree',
    editable: Boolean(item.editable),
    tags: Array.isArray(item.tags) ? item.tags : []
  };
}

export function normalizeWorkstreamBranchLoadResult(result: WorkstreamBranchLoadResult): WorkstreamBranchLoadResult {
  if (Array.isArray(result)) {
    const items = result.map(normalizeItem);
    const branch = items[0]?.branch ?? '';
    return {
      workspaceId: items[0]?.workspaceId ?? '',
      branch,
      selectedBranch: branch,
      branchRef: '',
      commit: '',
      currentCheckoutBranch: branch,
      sourceMode: 'working_tree',
      mode: 'working_tree',
      editable: true,
      scannedAt: '',
      itemCount: items.length,
      warnings: [],
      items
    };
  }
  const sourceMode = result.sourceMode === 'snapshot' || result.mode === 'snapshot' ? 'snapshot' : 'working_tree';
  const items = (Array.isArray(result.items) ? result.items : []).map(normalizeItem);
  return {
    ...result,
    branch: result.branch || result.selectedBranch,
    selectedBranch: result.selectedBranch || result.branch,
    currentCheckoutBranch: result.currentCheckoutBranch ?? '',
    sourceMode,
    mode: sourceMode,
    editable: Boolean(result.editable),
    itemCount: Number(result.itemCount) || items.length,
    warnings: Array.isArray(result.warnings) ? result.warnings : [],
    items
  };
}

export function normalizeItemDetail(item: ItemDetail): ItemDetail {
  return {
    ...normalizeItem(item),
    documents: Array.isArray(item.documents) ? item.documents : [],
    metadata: item.metadata ?? {},
    counts: item.counts ?? { files: 0 }
  };
}

export function normalizeGitStatus(status: GitStatus): GitStatus {
  return {
    ...status,
    ahead: status.ahead ?? 0,
    behind: status.behind ?? 0,
    dirty: Boolean(status.dirty),
    conflicted: Boolean(status.conflicted),
    changes: Array.isArray(status.changes) ? status.changes : []
  };
}

export function normalizeGitActivityEntry(entry: GitActivityEntry): GitActivityEntry {
  return {
    ...entry,
    committedAt: entry.committedAt ?? '',
    author: entry.author ?? '',
    message: entry.message ?? '',
    paths: Array.isArray(entry.paths) ? entry.paths : []
  };
}

export function normalizeGitResult(result: GitOperationResult): GitOperationResult {
  return {
    ...result,
    ok: Boolean(result.ok),
    status: normalizeGitStatus(result.status ?? { workspaceId: '', branch: '', ahead: 0, behind: 0, dirty: false, conflicted: false, changes: [] })
  };
}

export function normalizeAuditEvent(event: AuditEvent): AuditEvent {
  return {
    ...event,
    status: normalizeAuditStatus(event.status),
    paths: Array.isArray(event.paths) ? event.paths : [],
    durationMs: event.durationMs ?? 0
  };
}

export function normalizeWorkspaceHealth(health: WorkspaceHealth): WorkspaceHealth {
  return {
    ...health,
    summary: normalizeHealthStatus(health.summary),
    checks: (Array.isArray(health.checks) ? health.checks : []).map(normalizeHealthCheck)
  };
}

export function normalizeSearchResult(result: SearchResult): SearchResult {
  return {
    ...result,
    type: ['workspace', 'branch', 'savedFilter'].includes(result.type) ? result.type : 'item',
    subtitle: result.subtitle ?? '',
    context: result.context ?? '',
    score: result.score ?? 0
  };
}

export function normalizeSavedFilter(filter: SavedFilter): SavedFilter {
  return { ...filter, filters: filter.filters ?? {} };
}

export function normalizeRecentItem(item: RecentItem): RecentItem {
  return { ...item, subtitle: item.subtitle ?? '', route: item.route || `/items/${encodeURIComponent(item.itemId)}` };
}

export function normalizeHealthCheck(check: HealthCheck): HealthCheck {
  return { ...check, status: normalizeHealthStatus(check.status) };
}

export function normalizeAuditStatus(status: AuditEvent['status']): AuditEvent['status'] {
  return status === 'blocked' || status === 'failed' ? status : 'success';
}

export function normalizeHealthStatus(status: HealthCheck['status']): HealthCheck['status'] {
  return status === 'warning' || status === 'failed' ? status : 'ok';
}
