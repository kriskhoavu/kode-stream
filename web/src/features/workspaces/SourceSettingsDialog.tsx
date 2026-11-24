import type { Dispatch, RefObject, SetStateAction } from 'react';
import { SlidersHorizontal, X } from 'lucide-react';
import { api } from '../../shared/api';
import type { SourceStructureSettings } from '../../lib/types';
import { labels } from '../../lib/vocabulary';
import { useModalDialog } from '../../components/overlay';
import { ModalLayer } from '../../components/ModalLayer';
import { notifyReliabilityChanged } from '../reliability/hooks';
import { canonicalSourceSettingsCard, UNSORTED_SOURCE_SELECTION_ID as UNSORTED_SELECTION_ID } from './sourceSettingsEditor';
import { applySourceItemsProposal, clearSourceItemsProposal, SourceStructurePreviewTable, SourceStructureProposalList, updateSourceItemsPreviewField } from './sourceStructureEditor';
import { type WorkspaceNotice, scanNotice, sourceSettingsNotice } from './workspaceNotice';
import { errorMessage, settingsEditorFromResult, type SettingsEditorState } from './workspaceDraft';

// `busy` belongs to the page, so the page stays locked while the dialog saves.
export function SourceSettingsDialog({ editor, setEditor, busy, setBusy, onNotice, onChanged }: {
	editor: SettingsEditorState;
	setEditor: Dispatch<SetStateAction<SettingsEditorState | null>>;
	busy: boolean;
	setBusy: (pending: boolean) => void;
	onNotice: (notice: WorkspaceNotice | null) => void;
	onChanged: () => void | Promise<void>;
}) {
	const dialog = useModalDialog(() => setEditor(null), !busy, true);

	const saveSourceSettings = async () => {
		setBusy(true);
		onNotice(null);
		try {
			if (editor.selectedProposalId === UNSORTED_SELECTION_ID) {
				if (editor.exists) {
					const result = await api.resetSourceStructure(editor.repo.id, editor.directory);
					onNotice(sourceSettingsNotice('Source structure reset', editor.repo, result.scan));
				} else {
					const result = await api.scan(editor.repo.id);
					onNotice(scanNotice(editor.repo, result, 'Source kept unsorted'));
				}
				notifyReliabilityChanged();
				setEditor(null);
				await onChanged();
				return;
			}
			const settings: SourceStructureSettings = {
				version: 1,
				cards: [canonicalSourceSettingsCard(editor.card), ...editor.cards.slice(1).map(canonicalSourceSettingsCard)]
			};
			const result = await api.saveSourceStructure(editor.repo.id, editor.directory, settings);
			notifyReliabilityChanged();
			setEditor(null);
			onNotice(sourceSettingsNotice('Source structure saved', editor.repo, result.scan));
			await onChanged();
		} catch (err) {
			onNotice({ tone: 'error', title: 'Settings failed to save', details: [errorMessage(err)] });
		} finally {
			setBusy(false);
		}
	};

	const resetSourceSettings = async () => {
			const confirmed = window.confirm(`Reset Source Items for ${editor.directory}? This removes the source's rules and scans the source again.`);
		if (!confirmed) return;
		setBusy(true);
		onNotice(null);
		try {
			const { repo, directory } = editor;
			const result = await api.resetSourceStructure(repo.id, directory);
			notifyReliabilityChanged();
			setEditor(settingsEditorFromResult(repo, directory, result));
			onNotice(sourceSettingsNotice('Source structure reset', repo, result.scan));
			await onChanged();
		} catch (err) {
			onNotice({ tone: 'error', title: 'Settings reset failed', details: [errorMessage(err)] });
		} finally {
			setBusy(false);
		}
	};

	return (
		<ModalLayer><section className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) setEditor(null); }}>
		  <div ref={dialog.ref as RefObject<HTMLDivElement>} className="modal-panel source-structure-modal" role="dialog" aria-modal="true" aria-label={labels.sourceStructure}>
			<header>
			  <div>
				<h2>{labels.sourceStructure}</h2>
				<span>{editor.repo.name} / {editor.directory}</span>
			  </div>
			  <button data-autofocus className="icon-button" type="button" onClick={() => setEditor(null)} disabled={busy} aria-label="Close source items">
				<X size={16} />
			  </button>
			</header>
			<p className="modal-help">
			  Define how this source should be split into board items.
			</p>
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
			<footer className="modal-actions">
			  {editor.exists && (
				<button className="secondary danger" type="button" onClick={() => void resetSourceSettings()} disabled={busy}>
				  Reset config
				</button>
			  )}
			  <button className="secondary" type="button" onClick={() => setEditor(null)} disabled={busy}>Cancel</button>
			  <button className="primary" type="button" onClick={() => void saveSourceSettings()} disabled={busy}>
				<SlidersHorizontal size={15} />
				{busy ? 'Saving...' : editor.selectedProposalId === UNSORTED_SELECTION_ID ? 'Scan Unsorted' : 'Save and Scan'}
			  </button>
			</footer>
		  </div>
		</section></ModalLayer>
	);
}
