import { type DragEvent } from 'react';
import type { JiraConnection, KnowledgeSettings, WorkspaceConfig, WorkspaceInput, WorkspaceRuntimeConfig, SourceSurface, WorkspaceSource, SourceSettingsResult } from '../../lib/types';
import { defaultShownIn, sourceSurfaces, workspaceSources } from '../../lib/sources';
import { normalizeDroppedPath, parseSources } from './sourceSettings';
import { sourceSettingsEditorFromResult, type SourceSettingsEditorModel } from './sourceSettingsEditor';

export const DEFAULT_SOURCES = ['wiki', 'plans'];
export const emptyJiraConnection = (): JiraConnection => ({ deploymentType: 'cloud', baseUrl: '', projectKey: '', accountEmail: '', tokenEnvVar: 'JIRA_API_TOKEN' });
export const defaultRuntimeConfig = (): WorkspaceRuntimeConfig => ({
  type: 'docker-compose',
  configPath: '',
  rebuildPolicy: 'changed-only',
  commands: {
    up: '',
    down: '',
    rebuildChanged: '',
    verify: { smoke: '', critical: '', full: '' }
  },
  healthChecks: [],
  artifacts: { paths: [] },
  automation: {
    enabled: false,
    repositoryPath: '',
    runner: 'cypress',
    defaultEnvironment: 'local',
commandTemplate: 'CYPRESS_EPSAP_ENVIRONMENT={env} npx cypress run --spec {specs}',
    artifactPaths: ['cypress/reports', 'cypress/screenshots', 'cypress/videos']
  }
});
export type SettingsEditorState = SourceSettingsEditorModel & {
  repo: WorkspaceConfig;
};
export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : 'Unexpected error';
}

export function buildWorkspaceInput(input: {
  name: string;
  registrationMode: 'local_path' | 'remote_clone';
  path: string;
  remoteUrl: string;
  cloneRoot: string;
  baselineBranch: string;
  sources: string;
  jira?: JiraConnection | null;
}): WorkspaceInput {
  const payload = {
    name: input.name,
    baselineBranch: input.baselineBranch,
    sources: parseSources(input.sources),
    registrationMode: input.registrationMode
  } as WorkspaceInput;
  if (input.jira) payload.jira = normalizeJiraDraft(input.jira);
  if (input.registrationMode === 'remote_clone') {
    payload.remoteUrl = input.remoteUrl.trim();
    if (input.cloneRoot.trim()) {
      payload.cloneRoot = input.cloneRoot.trim();
    }
    return payload;
  }
  payload.path = input.path.trim();
  return payload;
}

export function normalizeJiraDraft(value: JiraConnection): JiraConnection {
  return { deploymentType: value.deploymentType, baseUrl: value.baseUrl.trim().replace(/\/$/, ''), projectKey: value.projectKey.trim().toUpperCase(), accountEmail: value.deploymentType === 'cloud' ? value.accountEmail?.trim() : undefined, tokenEnvVar: value.tokenEnvVar.trim() };
}

export function normalizeKnowledgeSettings(value: KnowledgeSettings): KnowledgeSettings {
	return { enabled: value.enabled !== false, enrichExecutable: value.enrichExecutable?.trim() ?? '', enrichArgs: [...(value.enrichArgs ?? [])] };
}

export function cloneRuntimeConfig(runtime: WorkspaceRuntimeConfig | undefined): WorkspaceRuntimeConfig | null {
  if (!runtime) return null;
  return {
    ...runtime,
    commands: { ...runtime.commands, verify: { ...runtime.commands.verify } },
    healthChecks: [...(runtime.healthChecks ?? [])],
    artifacts: { paths: [...(runtime.artifacts?.paths ?? [])] },
    automation: runtime.automation ? { ...runtime.automation, artifactPaths: [...(runtime.automation.artifactPaths ?? [])] } : undefined
  };
}

export function inferWorkspaceNameFromRemoteURL(remoteUrl: string): string {
  const value = remoteUrl.trim();
  if (!value) return '';
  const parsed = /[:/]([^/:?#]+?)(?:\.git)?$/.exec(value);
  return parsed?.[1] ?? '';
}

export function workspaceRemovalMessage(workspaces: WorkspaceConfig[]): string {
  if (workspaces.length === 0) return 'No workspaces selected.';
  if (workspaces.length === 1) {
    const [workspace] = workspaces;
		return `Remove ${workspace.name}? Cached items will be removed from the board${workspace.clonePathManaged && workspace.managedCloneVerified ? ', and the server-verified managed cloned repository folder will be deleted.' : '.'}`;
  }
	const managedCloneCount = workspaces.filter((workspace) => workspace.clonePathManaged && workspace.managedCloneVerified).length;
  return `Remove ${workspaces.length} selected workspaces? Cached items will be removed from the board${managedCloneCount > 0 ? `, and ${managedCloneCount} managed cloned repository folder${managedCloneCount === 1 ? '' : 's'} will be deleted.` : '.'}`;
}

export function pathFromDrop(event: DragEvent<HTMLElement>): string {
  const explicitPath = event.dataTransfer.getData('text/plain').trim();
  const uriList = event.dataTransfer.getData('text/uri-list').split('\n').find((line) => line.trim() && !line.startsWith('#'))?.trim();
  const filePath = (event.dataTransfer.files[0] as (File & { path?: string }) | undefined)?.path;
  return normalizeDroppedPath(filePath || uriList || explicitPath);
}

export function addSource(value: string, directory: string): string {
  return [...parseSources(value), directory].join(', ');
}

export function removeSource(value: string, directory: string): string {
  return parseSources(value).filter((item) => item !== directory).join(', ');
}

export function toggleSource(value: string, directory: string): string {
  const sources = parseSources(value);
  return sources.includes(directory) ? removeSource(value, directory) : addSource(value, directory);
}

export function settingsEditorFromResult(repo: WorkspaceConfig, directory: string, result: SourceSettingsResult): SettingsEditorState {
  return { repo, ...sourceSettingsEditorFromResult(directory, result) };
}

export function shownInByPath(repo: WorkspaceConfig): Record<string, SourceSurface[]> {
  return Object.fromEntries(workspaceSources(repo).map((source) => [source.path, source.shownIn]));
}

// A path kept from the saved workspace keeps its surfaces; a new one gets the defaults for its name.
export function sourcesFromDraft(paths: string[], shownIn: Record<string, SourceSurface[]>): WorkspaceSource[] {
  return paths.map((path) => ({ path, shownIn: shownIn[path]?.length ? shownIn[path] : defaultShownIn(path) }));
}

// Returns the sources with the surface toggled on the named path, or null when
// the toggle would leave the source shown nowhere.
export function toggleSourceSurface(sources: WorkspaceSource[], path: string, surface: SourceSurface): WorkspaceSource[] | null {
  const current = sources.find((source) => source.path === path);
  if (!current) return null;
  const shownIn = current.shownIn.includes(surface) ? current.shownIn.filter((item) => item !== surface) : sourceSurfaces.filter((item) => item === surface || current.shownIn.includes(item));
  if (shownIn.length === 0) return null;
  return sources.map((source) => (source.path === path ? { path, shownIn } : source));
}
