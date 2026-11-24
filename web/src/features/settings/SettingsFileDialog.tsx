import { useCallback, useEffect, useState } from 'react';
import { Download, Upload } from 'lucide-react';
import type { SettingsFileContent, SettingsFilePart, SettingsFileView } from '../../lib/types';
import { useModalDialog } from '../../components/overlay';
import { api } from '../../shared/api';

export const partLabels: Record<SettingsFilePart, string> = {
	provider: 'AI provider',
	pipeline: 'Harness switches',
	tests: 'Test command',
	scope: 'Paths the Builder may change',
	models: 'Models',
	procedure: 'Role skills',
	sources: 'Sources'
};

const onOff = (value?: boolean) => value === undefined ? 'default' : value ? 'on' : 'off';

export function partSummary(part: SettingsFilePart, content?: SettingsFileContent): string {
	const harness = content?.harness;
	switch (part) {
		case 'provider': return content?.ai?.provider ?? '—';
		case 'pipeline': return harness ? `run ${onOff(harness.enabled)} · start on its own ${onOff(harness.autoStart)} · review ${onOff(harness.review)} · stop and ask ${onOff(harness.askQuestions)}` : '—';
		case 'tests': return harness?.tests?.length ? harness.tests.join(' && ') : 'none';
		case 'scope': return harness?.scope?.length ? harness.scope.join(', ') : 'any path';
		case 'models': return Object.entries(harness?.models ?? {}).map(([role, model]) => `${role} ${[model?.engine, model?.model, model?.hardModel && `hard ${model.hardModel}`, model?.maxTurns && `${model.maxTurns} turns`].filter(Boolean).join(' ')}`).join(' · ') || 'none';
		case 'procedure': return Object.entries(harness?.procedure ?? {}).map(([role, step]) => `${role} ${step?.skill}${step?.track ? ` (${step.track})` : ''}`).join(' · ') || 'built-in instructions';
		case 'sources': return harness?.sources?.map((source) => `${source.kind}${source.enabled ? '' : ' (off)'}: ${source.query}`).join(' · ') || 'none';
	}
}

export type SettingsFileMode = 'export' | 'import';

// Kode Stream never reads the file on its own. Only Export and Import touch it.
export function SettingsFileDialog({ mode, workspaceId, workspaceName, onClose, onImported }: {
	mode: SettingsFileMode;
	workspaceId: string;
	workspaceName: string;
	onClose: () => void;
	onImported: () => void;
}) {
	const [view, setView] = useState<SettingsFileView>();
	const [error, setError] = useState('');
	const [notice, setNotice] = useState('');
	const [busy, setBusy] = useState(false);
	const [parts, setParts] = useState<SettingsFilePart[]>([]);
	const dialog = useModalDialog(onClose, !busy);

	const load = useCallback(async (signal?: AbortSignal) => {
		try {
			const next = await api.settingsFile(workspaceId, signal);
			if (signal?.aborted) return;
			setView(next);
			setError('');
		} catch (caught) {
			if (!signal?.aborted) setError(caught instanceof Error ? caught.message : 'The settings file could not be read.');
		}
	}, [workspaceId]);

	useEffect(() => {
		const controller = new AbortController();
		void load(controller.signal);
		return () => controller.abort();
	}, [load]);

	const run = async () => {
		const chosen = parts;
		setBusy(true);
		setError('');
		setNotice('');
		try {
			if (mode === 'export') {
				await api.exportSettingsFile(workspaceId, chosen);
				setNotice(`Wrote ${chosen.map((part) => partLabels[part]).join(', ')} into ${view?.path}. Commit it to share it.`);
			} else {
				await api.importSettingsFile(workspaceId, chosen);
				onImported();
				setNotice(`Imported ${chosen.map((part) => partLabels[part]).join(', ')}.`);
			}
			setParts([]);
			await load();
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'That did not work.');
		} finally {
			setBusy(false);
		}
	};
	const toggle = (part: SettingsFilePart) => setParts((current) => current.includes(part) ? current.filter((other) => other !== part) : [...current, part]);
	const shown = mode === 'export' ? view?.parts ?? [] : view?.has ?? [];
	const path = view?.path ?? '.kode-stream/settings.yaml';
	const fileState = !view ? '' : view.error
		? `The file is not valid, so nothing can be imported from it: ${view.error}`
		: view.exists ? `The file holds ${view.has.map((part) => partLabels[part]).join(', ') || 'nothing yet'}.` : mode === 'export' ? 'This workspace has no settings file yet. Export creates it.' : 'This workspace has no settings file to import from.';

	return (
		<div className="confirm-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onClose(); }}>
			<section ref={dialog.ref} className="confirm-dialog settings-file-dialog" role="dialog" aria-modal="true" aria-labelledby={dialog.titleId}>
				<header>
					<h2 id={dialog.titleId}>{mode === 'export' ? `Export ${workspaceName}'s settings` : `Import settings into ${workspaceName}`}</h2>
					<button className="icon-button" type="button" aria-label="Close dialog" disabled={busy} onClick={onClose}>×</button>
				</header>
				<p>{mode === 'export'
					? <>Writes the chosen parts into <code>{path}</code>, to share them through the repository. It holds no tool permission, executable or credential.</>
					: <>Lays the chosen parts of <code>{path}</code> over this workspace&apos;s settings. Kode Stream never reads the file on its own.</>}</p>
				{!view && !error && <p role="status">Reading the settings file…</p>}
				{view && <p className="settings-file-state">{fileState}</p>}
				{view && shown.length > 0 && (
					<fieldset className="settings-file-parts" disabled={busy || (mode === 'export' && Boolean(view.error))}>
						<legend>{mode === 'export' ? 'Parts to export' : 'Parts to import'}</legend>
						{shown.map((part) => (
							<label key={part} className="settings-file-part">
								<input type="checkbox" checked={parts.includes(part)} onChange={() => toggle(part)} />
								<span>
									<strong>{partLabels[part]}</strong>
									{mode === 'export'
										? <small>{partSummary(part, view.current)}</small>
										: <><small>File: {partSummary(part, view.file)}</small><small>Now: {partSummary(part, view.current)}</small></>}
								</span>
							</label>
						))}
					</fieldset>
				)}
				{error && <p className="confirm-dialog-error" role="alert">{error}</p>}
				{notice && <p role="status">{notice}</p>}
				<footer>
					<button className="ghost" type="button" disabled={busy} onClick={onClose}>{notice ? 'Done' : 'Cancel'}</button>
					{shown.length > 0 && <button data-autofocus className="primary" type="button" disabled={parts.length === 0 || busy || (mode === 'export' && Boolean(view?.error))} onClick={() => void run()}>
						{mode === 'export' ? <><Upload size={15} /> Export to the file</> : <><Download size={15} /> Import the chosen parts</>}
					</button>}
				</footer>
			</section>
		</div>
	);
}
