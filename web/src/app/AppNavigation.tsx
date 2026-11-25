import type { ReactNode } from 'react';
import { Bot, BookOpen, Boxes, FolderGit2, KanbanSquare as WorkstreamIcon, PanelLeftClose, PanelLeftOpen, Settings } from 'lucide-react';
import type { WorkspaceConfig } from '../lib/types';
import { sourcePaths } from '../lib/sources';
import type { Route } from './router';

export function LeftNav({ route, navigate, workspaces, activeWorkspaceId, lastSync, collapsed, onToggle, onSelectWorkspace }: {
  route: Route;
  navigate: (route: Route) => void;
  workspaces: WorkspaceConfig[];
  activeWorkspaceId?: string;
  lastSync: string;
  collapsed: boolean;
  onToggle: () => void;
  onSelectWorkspace: (workspace: WorkspaceConfig) => void;
}) {
  return (
    <aside className="left-nav">
      <div className="left-nav-brand-row">
        <button className="brand" onClick={() => navigate({ name: 'workstream' })} aria-label="Kode Stream home">
          <Boxes size={20} />
          <span>Kode Stream</span>
        </button>
        <button className="left-nav-toggle" type="button" onClick={onToggle} aria-label={collapsed ? 'Expand navigation' : 'Collapse navigation'} title={collapsed ? 'Expand navigation' : 'Collapse navigation'}>{collapsed ? <PanelLeftOpen size={17} /> : <PanelLeftClose size={17} />}</button>
      </div>
      <div className="nav-section">
        <span className="nav-section-label">Workspace</span>
        <NavButton active={route.name === 'workstream'} onClick={() => navigate({ name: 'workstream' })} icon={<WorkstreamIcon size={18} />} label="Workstream" />
        <NavButton active={route.name === 'canvas'} onClick={() => navigate({ name: 'canvas', location: { workspaceId: activeWorkspaceId } })} icon={<Bot size={18} />} label="Agent Space" />
        <NavButton active={route.name === 'knowledge'} onClick={() => navigate({ name: 'knowledge' })} icon={<BookOpen size={18} />} label="Knowledge" />
      </div>
      <div className="workspace-list">
        <span className="workspace-list-label">Workspaces</span>
        {workspaces.map((repo) => (
          <button
            className={repo.id === activeWorkspaceId ? 'workspace-button active' : 'workspace-button'}
            key={repo.id}
            onClick={() => onSelectWorkspace(repo)}
            title={repo.path}
          >
            <FolderGit2 size={16} />
            <span>{repo.name}</span>
          </button>
        ))}
        {workspaces.length === 0 && <span className="workspace-empty">No workspaces registered</span>}
      </div>
      <button className="add-repository-button" type="button" onClick={() => navigate({ name: 'workspaces' })} aria-label="Manage workspaces" title="Manage workspaces">
        <Settings size={16} />
        <span>Manage Workspaces</span>
      </button>
      <div className="repo-status">
        <span className="repo-status-label">Last scan</span>
        <span>{lastSync}</span>
      </div>
    </aside>
  );
}

export function WorkspaceMenu({ workspaces, activeWorkspaceId, onSelect, onManage }: {
  workspaces: WorkspaceConfig[];
  activeWorkspaceId?: string;
  onSelect: (workspace: WorkspaceConfig) => void;
  onManage: () => void;
}) {
  return (
    <div className="workspace-menu" role="menu">
        <div className="workspace-menu-header">
          <strong>Workspaces</strong>
          <span>{workspaces.length} workspace{workspaces.length === 1 ? '' : 's'}</span>
        </div>
        <div className="workspace-menu-list">
          {workspaces.map((repo) => (
            <button
              className={repo.id === activeWorkspaceId ? 'workspace-menu-item active' : 'workspace-menu-item'}
              key={repo.id}
              type="button"
              onClick={() => onSelect(repo)}
              role="menuitem"
              title={repo.path}
            >
              <FolderGit2 size={16} />
              <span>
                <strong>{repo.name}</strong>
                <small>{repo.location === 'cloud_agent' ? 'Cloud Agent' : 'Local'} · {repo.baselineBranch} · {sourcePaths(repo).join(', ') || 'plans'}</small>
              </span>
            </button>
          ))}
          {workspaces.length === 0 && <span className="workspace-menu-empty">No workspaces registered</span>}
        </div>
        <button className="workspace-menu-add" type="button" onClick={onManage}>
          <Settings size={15} />
          Manage Workspaces
        </button>
      </div>
  );
}

function NavButton({ active, icon, label, onClick }: { active: boolean; icon: ReactNode; label: string; onClick: () => void }) {
  return (
    <button className={active ? 'nav-button active' : 'nav-button'} onClick={onClick}>
      {icon}
      <span>{label}</span>
    </button>
  );
}
