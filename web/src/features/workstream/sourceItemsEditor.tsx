import { useState } from 'react';
import type { Dispatch, SetStateAction } from 'react';
import type { SourceSettingsResult, SourceStructurePreview, SourceStructureProposal, WorkspaceConfig } from '../../lib/types';
import { previewPathSegments } from '../workspaces/sourceSettings';
import { normalizeSourceSettingsCard, sourceSettingsEditorFromResult, UNSORTED_SOURCE_SELECTION_ID, type SourceSettingsEditorModel } from '../workspaces/sourceSettingsEditor';

export const UNSORTED_SELECTION_ID = UNSORTED_SOURCE_SELECTION_ID;

export type SourceItemsEditorState = SourceSettingsEditorModel & {
  workspace: WorkspaceConfig;
};

export function SourceStructureProposalList({
  proposals,
  selectedProposalId,
  onSelect,
  onClear
}: {
  proposals: SourceStructureProposal[];
  selectedProposalId?: string;
  onSelect: (proposal: SourceStructureProposal) => void;
  onClear: () => void;
}) {
  if (proposals.length === 0) return null;
  return (
    <section className="source-proposals" aria-label="Source structure proposals">
      <div className="source-structure-section-heading">
        <strong>Suggested structures</strong>
        <span>Choose a structure, or keep the source unsorted.</span>
      </div>
      <div className="source-proposal-grid">
        <button className={selectedProposalId === UNSORTED_SELECTION_ID ? 'source-proposal-card active' : 'source-proposal-card'} type="button" onClick={onClear}>
          <strong>Unsorted</strong>
          <span>Keep this source as one unstructured item in the Unsorted lane.</span>
        </button>
        {proposals.map((proposal) => {
          const selected = selectedProposalId === proposal.id;
          return (
            <button className={selected ? 'source-proposal-card active' : 'source-proposal-card'} type="button" key={proposal.id} onClick={() => onSelect(proposal)}>
              <strong>{proposal.label}</strong>
              <span>{proposal.summary}</span>
            </button>
          );
        })}
      </div>
    </section>
  );
}

export function SourceStructurePreviewTable({ preview, onChangeField }: {
  preview: SourceStructurePreview[];
  onChangeField: (path: string, field: 'item' | 'title' | 'status', value: string) => void;
}) {
  const [mode, setMode] = useState<'table' | 'tree'>('table');
  return (
    <section className="source-preview" aria-label="Source structure preview">
      <div className="source-structure-section-heading">
        <strong>Item mapping</strong>
        <div className="source-preview-heading-actions">
          <span>{preview.length === 0 ? 'No matching card directories yet.' : `${preview.length} sample cards`}</span>
          {preview.length > 0 && (
            <button
              type="button"
              className="source-preview-mode-toggle"
              onClick={() => setMode((current) => current === 'table' ? 'tree' : 'table')}
            >
              {mode === 'table' ? 'Tree view' : 'Table view'}
            </button>
          )}
        </div>
      </div>
      {preview.length > 0 && mode === 'table' && (
        <div className="source-preview-table">
          <div className="source-preview-row heading">
            <span>Path</span>
            <span>Source</span>
            <span>Item</span>
            <span>Title</span>
            <span>Status</span>
          </div>
          {preview.map((row) => (
            <div className="source-preview-row" key={row.path}>
              <span title={row.path}>{row.path}</span>
              <span>{row.source ?? row.scope}</span>
              <span><input value={row.item ?? row.identifier ?? ''} onChange={(event) => onChangeField(row.path, 'item', event.target.value)} /></span>
              <span><input value={row.title ?? ''} onChange={(event) => onChangeField(row.path, 'title', event.target.value)} /></span>
              <span>
                <select value={row.status ?? 'draft'} onChange={(event) => onChangeField(row.path, 'status', event.target.value)}>
                  <option value="unsorted">Unsorted</option>
                  <option value="draft">Draft</option>
                  <option value="in_progress">In Progress</option>
                  <option value="review">Review</option>
                  <option value="done">Done</option>
                </select>
              </span>
            </div>
          ))}
        </div>
      )}
      {preview.length > 0 && mode === 'tree' && <SourcePreviewTree preview={preview} />}
    </section>
  );
}

export type PreviewTreeNode = {
  name: string;
  path: string;
  row?: SourceStructurePreview;
  children: PreviewTreeNode[];
};

export function SourcePreviewTree({ preview }: { preview: SourceStructurePreview[] }) {
  return (
    <div className="source-preview-tree" role="tree" aria-label="Source item tree preview">
      {buildSourcePreviewTree(preview).map((node) => <SourcePreviewTreeNodeView key={node.path} node={node} />)}
    </div>
  );
}

export function SourcePreviewTreeNodeView({ node }: { node: PreviewTreeNode }) {
  return (
    <div className="source-preview-tree-node" role="treeitem" aria-label={node.path}>
      <span className="source-preview-tree-label">{node.name}</span>
      {node.row && (
        <small>
          item: {node.row.item ?? node.row.identifier} - title: {node.row.title} - status: {node.row.status}
        </small>
      )}
      {node.children.length > 0 && (
        <div className="source-preview-tree-children" role="group">
          {node.children.map((child) => <SourcePreviewTreeNodeView key={child.path} node={child} />)}
        </div>
      )}
    </div>
  );
}

export function buildSourcePreviewTree(preview: SourceStructurePreview[]): PreviewTreeNode[] {
  type MutableTreeNode = PreviewTreeNode & { childMap: Map<string, MutableTreeNode> };
  const roots = new Map<string, MutableTreeNode>();
  for (const row of preview) {
    const segments = row.path.split('/').filter(Boolean);
    let pathSoFar = '';
    let scope = roots;
    let currentNode: MutableTreeNode | null = null;
    for (const segment of segments) {
      pathSoFar = pathSoFar ? `${pathSoFar}/${segment}` : segment;
      if (!scope.has(segment)) {
        scope.set(segment, { name: segment, path: pathSoFar, children: [], childMap: new Map() });
      }
      currentNode = scope.get(segment) ?? null;
      scope = currentNode?.childMap ?? new Map();
    }
    if (currentNode) currentNode.row = row;
  }

  const toImmutable = (nodes: Map<string, MutableTreeNode>): PreviewTreeNode[] => Array.from(nodes.values())
    .sort((left, right) => left.name.localeCompare(right.name, undefined, { numeric: true, sensitivity: 'base' }))
    .map((node) => ({
      name: node.name,
      path: node.path,
      row: node.row,
      children: toImmutable(node.childMap)
    }));

  return toImmutable(roots);
}

export function sourceItemsEditorFromResult(workspace: WorkspaceConfig, directory: string, result: SourceSettingsResult): SourceItemsEditorState {
  return { workspace, ...sourceSettingsEditorFromResult(directory, result) };
}

export function applySourceItemsProposal(
  setSourceItemsEditor: Dispatch<SetStateAction<SourceItemsEditorState | null>>,
  proposal: SourceStructureProposal
) {
  setSourceItemsEditor((current) => {
    if (!current) return current;
    return {
      ...current,
      card: normalizeSourceSettingsCard(proposal.card, current.directory),
			cards: [normalizeSourceSettingsCard(proposal.card, current.directory), ...current.cards.slice(1)],
      selectedProposalId: proposal.id,
      preview: proposal.preview
    };
  });
}

export function clearSourceItemsProposal(
  setSourceItemsEditor: Dispatch<SetStateAction<SourceItemsEditorState | null>>
) {
  setSourceItemsEditor((current) => current ? {
    ...current,
    selectedProposalId: UNSORTED_SELECTION_ID,
    preview: current.unsortedPreview
  } : current);
}

export function updateSourceItemsPreviewField(
  setSourceItemsEditor: Dispatch<SetStateAction<SourceItemsEditorState | null>>,
  path: string,
  field: 'item' | 'title' | 'status',
  value: string
) {
  setSourceItemsEditor((current) => {
    if (!current) return current;
    const normalized = value.trim();
    const nextCard = { ...current.card, fields: { ...current.card.fields } };
    let nextPreview: SourceStructurePreview[] = current.preview.map((row) => ({
      ...row,
      item: row.path === path && field === 'item' ? value : row.item,
      identifier: row.path === path && field === 'item' ? value : row.identifier,
      title: row.path === path && field === 'title' ? value : row.title,
      status: row.path === path && field === 'status' ? value as SourceStructurePreview['status'] : row.status
    }));
    if (field === 'item') {
      nextCard.fields.item = normalized;
      const suggestedTemplate = suggestTemplateFromValue(current.directory, current.card.pathPattern, path, normalized, true);
      if (suggestedTemplate) {
        nextCard.fields.item = suggestedTemplate;
        nextPreview = current.preview.map((row): SourceStructurePreview => {
          const captures = pathPatternCaptures(current.directory, current.card.pathPattern, row.path);
          const rendered = captures ? renderTemplateWithCaptures(suggestedTemplate, captures) : '';
          const resolved = rendered || (row.path === path ? normalized : row.item ?? row.identifier);
          return { ...row, item: resolved, identifier: resolved };
        });
      }
    }
    if (field === 'title') {
      nextCard.fields.title = value;
      const suggestedTemplate = suggestTemplateFromValue(current.directory, current.card.pathPattern, path, normalized, false);
      if (suggestedTemplate) {
        nextCard.fields.title = suggestedTemplate;
        nextPreview = current.preview.map((row): SourceStructurePreview => {
          const captures = pathPatternCaptures(current.directory, current.card.pathPattern, row.path);
          const rendered = captures ? renderTemplateWithCaptures(suggestedTemplate, captures) : '';
          const resolved = rendered || (row.path === path ? value : row.title);
          return { ...row, item: row.item, identifier: row.identifier, title: resolved };
        });
      }
    }
    if (field === 'status') {
      nextCard.fields.status = value;
    }

    return {
      ...current,
      selectedProposalId: undefined,
      card: nextCard,
      preview: nextPreview
    };
  });
}

export function suggestTemplateFromValue(directory: string, pathPattern: string, rowPath: string, value: string, allowMultiSegment: boolean): string | null {
  if (!value) return null;
  const captures = pathPatternCaptures(directory, pathPattern, rowPath);
  if (!captures) return null;
  const segments = value.split('/').map((segment) => segment.trim()).filter(Boolean);
  if (segments.length === 0) return null;
  if (!allowMultiSegment && segments.length > 1) return null;

  const used = new Set<string>();
  const templateSegments: string[] = [];
  for (const segment of segments) {
    const options = Object.entries(captures)
      .filter(([name, value]) => value === segment && !used.has(name));
    if (options.length !== 1) return null;
    used.add(options[0][0]);
    templateSegments.push(`{${options[0][0]}}`);
  }
  return templateSegments.join('/');
}

export function pathPatternCaptures(directory: string, pathPattern: string, rowPath: string): Record<string, string> | null {
  const patternSegments = pathPattern.split('/').map((segment) => segment.trim()).filter(Boolean);
  const rowSegments = previewPathSegments(rowPath, directory);
  if (patternSegments.length === 0 || patternSegments.length !== rowSegments.length) return null;

  const captures: Record<string, string> = {};
  for (let index = 0; index < patternSegments.length; index += 1) {
    const patternSegment = patternSegments[index];
    const rowSegment = rowSegments[index];
    const variable = patternSegment.match(/^\{([A-Za-z][A-Za-z0-9_]*)\}$/)?.[1];
    if (variable) {
      captures[variable] = rowSegment;
      continue;
    }
    if (patternSegment !== rowSegment) return null;
  }
  return captures;
}

export function renderTemplateWithCaptures(template: string, captures: Record<string, string>): string {
  return Object.entries(captures).reduce((result, [name, value]) => result.replaceAll(`{${name}}`, value), template).trim();
}
