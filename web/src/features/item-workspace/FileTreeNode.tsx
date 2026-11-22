import { memo, useState } from 'react';
import type { CSSProperties } from 'react';
import { ChevronDown, File as FileIcon, FolderOpen } from 'lucide-react';
import type { FileNode } from '../../lib/types';
import { FileStateIcon } from '../file-tree/FileStateIcon';
import type { TreeFileState } from '../file-tree/FileStateIcon';
import { normalizePath } from './itemPaths';

export const TreeNode = memo(function TreeNode({ node, onOpen, activePath, depth, fileStateByPath, selectedDirectoryPath, onSelectDirectory, onSelectFile }: { node: FileNode; onOpen: (id: string) => void; activePath?: string; depth: number; fileStateByPath: Map<string, TreeFileState>; selectedDirectoryPath: string; onSelectDirectory: (path: string) => void; onSelectFile: (path: string) => void }) {
  const indent = { '--tree-indent': `${depth * 14}px` } as CSSProperties & Record<'--tree-indent', string>;
  const [expanded, setExpanded] = useState(true);

  if (node.type === 'directory') {
    return (
      <details open={expanded} className="tree-dir">
        <summary className={selectedDirectoryPath === node.path ? 'tree-row tree-row-dir active' : 'tree-row tree-row-dir'} style={indent} title={node.path} onClick={(event) => { event.preventDefault(); onSelectDirectory(node.path); }} onDoubleClick={(event) => { event.preventDefault(); setExpanded((value) => !value); }}>
          <ChevronDown className="tree-chevron" size={14} />
          <FolderOpen className="tree-icon" size={16} />
          <span className="tree-label">{node.name}</span>
        </summary>
        <div className="tree-children">
          {node.children?.map((child) => <TreeNode node={child} key={child.id} onOpen={onOpen} activePath={activePath} depth={depth + 1} fileStateByPath={fileStateByPath} selectedDirectoryPath={selectedDirectoryPath} onSelectDirectory={onSelectDirectory} onSelectFile={onSelectFile} />)}
        </div>
      </details>
    );
  }
  const state = fileStateByPath.get(normalizePath(node.path));
  return (
    <button className={activePath === node.path ? 'tree-row tree-file active' : 'tree-row tree-file'} style={indent} title={node.path} onClick={() => { onSelectFile(node.path); onOpen(node.id); }}>
      <span className="tree-spacer" />
      <FileIcon className="tree-icon" size={16} />
      <span className="tree-label">{node.name}</span>
      {state && <FileStateIcon state={state} />}
    </button>
  );
});
