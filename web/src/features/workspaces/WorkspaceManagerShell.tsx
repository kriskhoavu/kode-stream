import { Search, SquareCheckBig } from 'lucide-react';
import type { WorkspaceConfig } from '../../lib/types';

export function filterWorkspaces(workspaces: WorkspaceConfig[], query: string): WorkspaceConfig[] {
  const normalized = query.trim().toLocaleLowerCase();
  if (!normalized) return workspaces;
  return workspaces.filter((workspace) => [workspace.name, workspace.path, workspace.remoteUrl ?? '']
    .some((value) => value.toLocaleLowerCase().includes(normalized)));
}

export function WorkspaceList({
  workspaces,
  selectedWorkspaceId,
  selectedWorkspaceIds,
  query,
  bulkMode,
  onQueryChange,
  onSelect,
  onToggleBulkMode,
  onToggleSelection
}: {
  workspaces: WorkspaceConfig[];
  selectedWorkspaceId: string;
  selectedWorkspaceIds: string[];
  query: string;
  bulkMode: boolean;
  onQueryChange: (query: string) => void;
  onSelect: (workspaceId: string) => void;
  onToggleBulkMode: () => void;
  onToggleSelection: (workspaceId: string) => void;
}) {
  const visibleWorkspaces = filterWorkspaces(workspaces, query);

  return <aside className="workspace-manager-list" aria-label="Registered workspaces">
    <div className="workspace-manager-list-header">
      <label className="workspace-manager-search">
        <Search size={15} aria-hidden="true" />
        <input
          aria-label="Search workspaces"
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder="Search workspaces"
        />
      </label>
      <button className={bulkMode ? 'secondary active' : 'secondary'} type="button" onClick={onToggleBulkMode} aria-pressed={bulkMode}>
        <SquareCheckBig size={15} />
        {bulkMode ? 'Done' : 'Select'}
      </button>
    </div>
    <div className="workspace-manager-list-items">
      {visibleWorkspaces.map((workspace) => {
        const active = workspace.id === selectedWorkspaceId;
        return <div className={active ? 'workspace-manager-list-item active' : 'workspace-manager-list-item'} key={workspace.id}>
          {bulkMode && <label className="workspace-manager-checkbox" aria-label={`Select ${workspace.name}`}>
            <input
              type="checkbox"
              checked={selectedWorkspaceIds.includes(workspace.id)}
              onChange={() => onToggleSelection(workspace.id)}
            />
          </label>}
          <button type="button" onClick={() => onSelect(workspace.id)} aria-current={active ? 'true' : undefined}>
            <span className="workspace-manager-item-title">
              <strong>{workspace.name}</strong>
              <span className={workspace.lastScannedAt ? 'workspace-manager-state ok' : 'workspace-manager-state'}>
                {workspace.lastScannedAt ? 'Scanned' : 'Not scanned'}
              </span>
            </span>
            <span className="workspace-manager-item-path" title={workspace.path}>{workspace.path}</span>
            <span className="workspace-manager-item-meta">
              {workspace.registrationMode === 'remote_clone' ? 'Remote' : workspace.registrationMode === 'existing_workspace' ? 'Imported' : 'Local'} · {workspace.sources.length} source{workspace.sources.length === 1 ? '' : 's'}
            </span>
          </button>
        </div>;
      })}
      {visibleWorkspaces.length === 0 && <div className="workspace-manager-list-empty">
        {workspaces.length === 0 ? 'No workspaces registered.' : 'No workspaces match your search.'}
      </div>}
    </div>
  </aside>;
}
