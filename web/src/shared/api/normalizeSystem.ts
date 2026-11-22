import type { AISettings, AILaunchTemplate, SystemConfigPaths, StorageStatus, StorageSyncResult, HarnessRunDetail, HarnessSettings, HarnessTask } from '../../lib/types';

export function normalizeHarnessTask(task: HarnessTask): HarnessTask {
  return { ...task, scoreReasons: task.scoreReasons ?? [], dependsOn: task.dependsOn ?? [], source: { ...task.source, labels: task.source?.labels ?? [] } };
}

export function normalizeHarnessRunDetail(detail: HarnessRunDetail): HarnessRunDetail {
  return { ...detail, task: normalizeHarnessTask(detail.task), stages: detail.stages ?? [], agentRuns: detail.agentRuns ?? [], approvals: detail.approvals ?? [] };
}

export function knowledgeURL(workspaceId: string, root: string, resource: 'pages' | 'graph' | 'rescan'): string {
	return `/api/knowledge/wikis/${encodeURIComponent(workspaceId)}/${encodeURIComponent(root)}/${resource}`;
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
