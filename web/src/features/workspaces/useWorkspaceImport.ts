import { useEffect, useRef, useState } from 'react';
import type { WorkspaceImportPreview, WorkspaceImportResult } from '../../lib/types';
import { api } from '../../shared/api';
import { errorMessage } from './workspaceDraft';
import { defaultWorkspaceImportSelection, type WorkspaceImportState } from './workspaceImportReview';

/**
 * Importing workspaces from another app's settings file: pick the file,
 * preview the workspaces it holds, choose some and import them.
 */
export function useWorkspaceImport({ onChanged }: { onChanged: () => void | Promise<void> }) {
	const [importSourcePath, setImportSourcePath] = useState('');
	const [importState, setImportState] = useState<WorkspaceImportState>('selecting');
	const [importPreview, setImportPreview] = useState<WorkspaceImportPreview | null>(null);
	const [importSelection, setImportSelection] = useState<string[]>([]);
	const [importResults, setImportResults] = useState<WorkspaceImportResult[]>([]);
	const [importError, setImportError] = useState('');
	const [importConfirmOpen, setImportConfirmOpen] = useState(false);
	const importStatusRef = useRef<HTMLDivElement | null>(null);

	useEffect(() => {
		if (importState === 'reviewing' || importState === 'complete') importStatusRef.current?.focus();
	}, [importState]);

	const resetWorkspaceImport = () => {
		setImportSourcePath('');
		setImportState('selecting');
		setImportPreview(null);
		setImportSelection([]);
		setImportResults([]);
		setImportError('');
		setImportConfirmOpen(false);
	};

	const changeImportSourcePath = (value: string) => {
		setImportSourcePath(value);
		setImportState('selecting');
		setImportPreview(null);
		setImportSelection([]);
		setImportResults([]);
		setImportError('');
	};

	const toggleImportCandidate = (candidateKey: string) => {
		setImportSelection((current) => current.includes(candidateKey) ? current.filter((key) => key !== candidateKey) : [...current, candidateKey]);
	};

	const toggleAllImportCandidates = () => {
		if (!importPreview) return;
		const selectable = importPreview.candidates.filter((candidate) => candidate.status === 'valid').map((candidate) => candidate.candidateKey);
		setImportSelection((current) => selectable.every((key) => current.includes(key)) ? [] : selectable);
	};

	const importExistingWorkspaces = async () => {
		if (!importPreview || importSelection.length === 0) return;
		setImportConfirmOpen(false);
		setImportState('importing');
		setImportError('');
		try {
			const results = await api.importWorkspaces({ sourcePath: importPreview.sourcePath, sourceFingerprint: importPreview.sourceFingerprint, candidateKeys: importSelection });
			setImportResults(results);
			setImportState('complete');
			if (results.some((result) => result.workspace)) await onChanged();
		} catch (err) {
			setImportState('error');
			setImportError(errorMessage(err));
		}
	};

	const browseImportSource = async () => {
		setImportError('');
		try {
			const selection = await api.selectYAMLFile();
			if (!selection.path) return;
			changeImportSourcePath(selection.path);
		} catch (err) {
			setImportState('error');
			setImportError(errorMessage(err));
		}
	};

	const previewExistingWorkspaces = async () => {
		if (!importSourcePath.trim()) return;
		setImportState('previewing');
		setImportError('');
		try {
			const preview = await api.previewWorkspaceImport(importSourcePath.trim());
			setImportPreview(preview);
			setImportSourcePath(preview.sourcePath);
			setImportSelection(defaultWorkspaceImportSelection(preview));
			setImportState('reviewing');
		} catch (err) {
			setImportPreview(null);
			setImportSelection([]);
			setImportState('error');
			setImportError(errorMessage(err));
		}
	};

	return { importSourcePath, importState, importPreview, importSelection, importResults, importError, importConfirmOpen, setImportConfirmOpen, importStatusRef, resetWorkspaceImport, changeImportSourcePath, toggleImportCandidate, toggleAllImportCandidates, importExistingWorkspaces, browseImportSource, previewExistingWorkspaces };
}
