import { useCallback, useEffect, useState } from 'react';
import { Download, Upload } from 'lucide-react';
import type { SettingsFileContent, SettingsFilePart, SettingsFileView } from '../../lib/types';
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

/** One line on what a part of a file holds. */
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

/**
 * A workspace's settings shared through its repository. Kode Stream never
 * reads the file on its own: Export writes the chosen parts of this
 * workspace's settings into it, and Import lays the chosen parts of it over
 * them.
 */
export function SettingsFilePanel({ workspaceId, workspaceName, onImported }: { workspaceId: string; workspaceName: string; onImported: () => void }) {
	const [view, setView] = useState<SettingsFileView>();
	const [error, setError] = useState('');
	const [notice, setNotice] = useState('');
	const [busy, setBusy] = useState(false);
	const [exportParts, setExportParts] = useState<SettingsFilePart[]>([]);
	const [importParts, setImportParts] = useState<SettingsFilePart[]>([]);

	const load = useCallback(async (signal?: AbortSignal) => {
		try {
			const next = await api.settingsFile(workspaceId, signal);
			if (signal?.aborted) return;
			setView(next);
			setImportParts([]);
			setError('');
		} catch (caught) {
			if (!signal?.aborted) setError(caught instanceof Error ? caught.message : 'The settings file could not be read.');
		}
	}, [workspaceId]);

	useEffect(() => {
		const controller = new AbortController();
		setView(undefined);
		setExportParts([]);
		setNotice('');
		void load(controller.signal);
		return () => controller.abort();
	}, [load]);

	const act = async (work: () => Promise<string>) => {
		setBusy(true);
		setError('');
		setNotice('');
		try {
			setNotice(await work());
			await load();
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'That did not work.');
		} finally {
			setBusy(false);
		}
	};
	const toggle = (list: SettingsFilePart[], part: SettingsFilePart) => list.includes(part) ? list.filter((other) => other !== part) : [...list, part];

	return (
		<section className="settings-section settings-file-section">
			<header>
				<div>
					<span className="settings-group-label">Share</span>
					<h2>Settings file</h2>
					<p>{view?.path ?? '.kode-stream/settings.yaml'} shares chosen parts of {workspaceName}&apos;s settings through its repository. Kode Stream never reads it on its own. It holds no tool permission, executable or credential.</p>
				</div>
			</header>
			{!view && !error && <p role="status">Reading the settings file…</p>}
			{error && <p className="settings-error" role="alert">{error}</p>}
			{view && <>
				<p className="settings-file-state">{view.error ? `The file is not valid, so nothing can be imported from it: ${view.error}` : view.exists ? `The file holds ${view.has.map((part) => partLabels[part]).join(', ') || 'nothing yet'}.` : 'This workspace has no settings file yet. Export creates it.'}</p>
				<div className="settings-file-columns">
					<fieldset className="settings-file-parts" disabled={busy}>
						<legend>Export from this workspace</legend>
						{view.parts.map((part) => (
							<label key={part} className="settings-file-part">
								<input type="checkbox" checked={exportParts.includes(part)} onChange={() => setExportParts((current) => toggle(current, part))} />
								<span><strong>{partLabels[part]}</strong><small>{partSummary(part, view.current)}</small></span>
							</label>
						))}
						<button type="button" className="secondary" disabled={exportParts.length === 0 || busy || Boolean(view.error)} onClick={() => void act(async () => { await api.exportSettingsFile(workspaceId, exportParts); setExportParts([]); return `Wrote ${exportParts.map((part) => partLabels[part]).join(', ')} into ${view.path}. Commit it to share it.`; })}>
							<Upload size={15} /> Export to the file
						</button>
					</fieldset>
					<fieldset className="settings-file-parts" disabled={busy || !view.file}>
						<legend>Import into this workspace</legend>
						{view.has.length === 0 && <p className="hx-muted">{view.exists ? 'Nothing to import.' : 'No file to import from.'}</p>}
						{view.has.map((part) => (
							<label key={part} className="settings-file-part">
								<input type="checkbox" checked={importParts.includes(part)} onChange={() => setImportParts((current) => toggle(current, part))} />
								<span><strong>{partLabels[part]}</strong><small>File: {partSummary(part, view.file)}</small><small>Now: {partSummary(part, view.current)}</small></span>
							</label>
						))}
						{view.has.length > 0 && <button type="button" className="secondary" disabled={importParts.length === 0 || busy} onClick={() => void act(async () => { await api.importSettingsFile(workspaceId, importParts); onImported(); return `Imported ${importParts.map((part) => partLabels[part]).join(', ')}.`; })}>
							<Download size={15} /> Import the chosen parts
						</button>}
					</fieldset>
				</div>
				<span role="status" className="hx-muted">{notice}</span>
			</>}
		</section>
	);
}
