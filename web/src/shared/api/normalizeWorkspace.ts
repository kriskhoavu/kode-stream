import type { WorkstreamBranchLoadResult, HealthCheck, WorkspaceConfig, WorkspaceCreateResult, WorkspaceHealth, WorkspaceImportPreview, WorkspaceImportResult, WorkspaceDirectoryListing, WorkspacePathGitState, WorkspacePathMutationResult, WorkspaceTreeEntry } from '../../lib/types';
import { normalizeSources } from '../../lib/sources';
import { normalizeItem } from './normalizeItem';

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

export function normalizeWorkspaceHealth(health: WorkspaceHealth): WorkspaceHealth {
  return {
    ...health,
    summary: normalizeHealthStatus(health.summary),
    checks: (Array.isArray(health.checks) ? health.checks : []).map(normalizeHealthCheck)
  };
}

export function normalizeHealthCheck(check: HealthCheck): HealthCheck {
  return { ...check, status: normalizeHealthStatus(check.status) };
}

export function normalizeHealthStatus(status: HealthCheck['status']): HealthCheck['status'] {
  return status === 'warning' || status === 'failed' ? status : 'ok';
}
