import { useEffect, useState } from 'react';
import type { RefObject } from 'react';
import { SlidersHorizontal, X } from 'lucide-react';
import { ModalLayer } from '../../components/ModalLayer';
import { useModalDialog } from '../../components/overlay';
import { useBodyScrollLock } from '../../components/useBodyScrollLock';
import { labels } from '../../lib/vocabulary';
import { sourcesShownIn } from '../../lib/sources';
import { api } from '../../shared/api';
import type { SourceStructureSettings, WorkspaceConfig } from '../../lib/types';
import { notifyReliabilityChanged } from '../reliability/hooks';
import { canonicalSourceSettingsCard } from '../workspaces/sourceSettingsEditor';
import { applySourceItemsProposal, clearSourceItemsProposal, sourceItemsEditorFromResult, SourceStructurePreviewTable, SourceStructureProposalList, UNSORTED_SELECTION_ID, updateSourceItemsPreviewField, type SourceItemsEditorState } from './sourceItemsEditor';
import { useWorkspaceGuard } from './useWorkspaceGuard';

/**
 * The Source Items dialog: how one of the workspace's sources splits into
 * board items. It loads, saves and resets the source's rules itself and
 * reports each change through `onSaved`.
 */
export function SourceItemsDialog({ workspace, initialDirectory, onClose, onSaved }: {
  workspace: WorkspaceConfig;
  initialDirectory: string;
  onClose: () => void;
  /** After a save or reset: what to say, and whether the dialog is done. */
  onSaved: (message: string, done: boolean) => Promise<void> | void;
}) {
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [directory, setDirectory] = useState('');
  const [error, setError] = useState('');
  const [editor, setEditor] = useState<SourceItemsEditorState | null>(null);
  const track = useWorkspaceGuard(workspace.id);
  const dialog = useModalDialog(onClose, !saving);
  useBodyScrollLock();

  const load = async (directory: string) => {
		const active = track(workspace.id);
    setLoading(true);
    setError('');
    setDirectory(directory);
    try {
      const result = await api.sourceStructure(workspace.id, directory);
			if (!active()) return;
      setEditor(sourceItemsEditorFromResult(workspace, directory, result));
    } catch (err) {
			if (!active()) return;
      setEditor(null);
      setError(err instanceof Error ? err.message : 'Source settings failed to load');
    } finally {
		if (active()) setLoading(false);
    }
  };

  // eslint-disable-next-line react-hooks/exhaustive-deps -- the first load only; a change of source reloads through its picker
  useEffect(() => { void load(initialDirectory); }, []);

  const save = async () => {
    if (!editor) return;
    setSaving(true);
    setError('');
    try {
      let message: string;
      if (editor.selectedProposalId === UNSORTED_SELECTION_ID) {
        if (editor.exists) {
          await api.resetSourceStructure(editor.workspace.id, editor.directory);
          message = 'Source structure reset';
        } else {
          const result = await api.scan(editor.workspace.id);
          message = `${result.itemCount} items indexed`;
        }
      } else {
        const settings: SourceStructureSettings = {
          version: 1,
          cards: [canonicalSourceSettingsCard(editor.card), ...editor.cards.slice(1).map(canonicalSourceSettingsCard)]
        };
        await api.saveSourceStructure(editor.workspace.id, editor.directory, settings);
        message = 'Source structure saved';
      }
      notifyReliabilityChanged();
      await onSaved(message, true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Source settings failed to save');
    } finally {
      setSaving(false);
    }
  };

  const reset = async () => {
    if (!editor) return;
    const confirmed = window.confirm(`Reset Source Items for ${editor.directory}? This removes the source's rules and scans the source again.`);
    if (!confirmed) return;
    setSaving(true);
    setError('');
    try {
      const result = await api.resetSourceStructure(editor.workspace.id, editor.directory);
      notifyReliabilityChanged();
      setEditor(sourceItemsEditorFromResult(editor.workspace, editor.directory, result));
      await onSaved('Source structure reset', false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Source settings failed to reset');
    } finally {
      setSaving(false);
    }
  };

  return (
    <ModalLayer><div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !saving) onClose(); }}>
      <div ref={dialog.ref as RefObject<HTMLDivElement>} className="modal-panel source-structure-modal" role="dialog" aria-modal="true" aria-label={labels.sourceStructure}>
        <header>
          <div>
            <h2>{labels.sourceStructure}</h2>
            <span>{workspace.name} / {directory || sourcesShownIn(workspace, 'workstream')[0]}</span>
          </div>
          <button data-autofocus className="icon-button" type="button" onClick={() => onClose()} disabled={saving} aria-label="Close source items">
            <X size={16} />
          </button>
        </header>
        {sourcesShownIn(workspace, 'workstream').length > 1 && (
          <label className="repo-field">
            Source
            <select value={directory} onChange={(event) => void load(event.target.value)} disabled={loading || saving}>
              {sourcesShownIn(workspace, 'workstream').map((directory) => <option key={directory} value={directory}>{directory}</option>)}
            </select>
          </label>
        )}
        <p className="modal-help">Define how this source should be split into board items.</p>
        {loading && <span className="reliability-muted">Loading source settings...</span>}
        {!loading && error && <span className="error">{error}</span>}
        {!loading && editor && (
          <>
            {!editor.exists && editor.mode === 'structured' && (
              <div className="metadata-callout source-structure-supported">
                <strong>Built-in structure detected</strong>
                <span>This source already follows a supported item layout. Saving here creates an optional override.</span>
              </div>
            )}
            {!editor.exists && editor.mode !== 'structured' && (
              <div className="metadata-callout">
                <strong>No settings file yet</strong>
                <span>Saving adds this source's rules to <code>.kode-stream/workspace-settings.yaml</code>.</span>
              </div>
            )}
            {editor.warnings.length > 0 && (
              <div className="plan-warnings">
                <h3>Warnings</h3>
                {editor.warnings.map((warning) => <p key={warning}>{warning}</p>)}
              </div>
            )}
            <SourceStructureProposalList
              proposals={editor.proposals}
              selectedProposalId={editor.selectedProposalId}
              onSelect={(proposal) => applySourceItemsProposal(setEditor, proposal)}
              onClear={() => clearSourceItemsProposal(setEditor)}
            />
            <SourceStructurePreviewTable
              preview={editor.preview}
              onChangeField={(path, field, value) => updateSourceItemsPreviewField(setEditor, path, field, value)}
            />
          </>
        )}
        <footer className="modal-actions">
          {editor?.exists && (
            <button className="secondary danger" type="button" onClick={() => void reset()} disabled={saving || loading}>
              Reset config
            </button>
          )}
          <button className="secondary" type="button" onClick={() => onClose()} disabled={saving}>Cancel</button>
          <button className="primary" type="button" onClick={() => void save()} disabled={saving || loading || !editor}>
            <SlidersHorizontal size={15} />
            {saving ? 'Saving...' : editor?.selectedProposalId === UNSORTED_SELECTION_ID ? 'Scan Unsorted' : 'Save and Scan'}
          </button>
        </footer>
      </div>
    </div></ModalLayer>
  );
}
