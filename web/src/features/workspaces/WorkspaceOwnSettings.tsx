import { useState } from 'react';
import { Save } from 'lucide-react';
import type { AISettings, WorkspaceConfig } from '../../lib/types';
import { useAISettings } from '../ai-settings/useAISettings';
import { SettingsFilePanel } from '../settings/SettingsFilePanel';
import { HarnessSettingsSection } from '../workstream/HarnessSettingsSection';
import '../harness/harness.css';

/**
 * One workspace's own settings, on its page under Workspaces: the provider a
 * new session starts with, the harness's choices and models for it, and the
 * file that shares them with a team. App-wide settings stay under Settings.
 */
export function WorkspaceOwnSettings({ workspace, showHarness = true }: { workspace: WorkspaceConfig; /** The harness is absent on the extension surface. */ showHarness?: boolean }) {
	const ai = useAISettings();
	const [reload, setReload] = useState(0);
	return (
		<section className="settings-page workspace-own-settings">
			<section className="settings-section ai-settings-section">
				<header>
					<div>
						<span className="settings-group-label">Local AI tools</span>
						<h2>Provider in this workspace</h2>
						<p>The tool a new session in {workspace.name} starts with. Executables and terminals are set once for this machine, under Settings.</p>
					</div>
				</header>
				{ai.loading && !ai.settings && <p role="status">Loading the AI settings…</p>}
				{ai.settings && (
					<>
						<label className="settings-toggle-row">
							<span><strong>Provider here</strong><small>Default follows the provider chosen under Settings.</small></span>
							<select aria-label="Provider here" value={ai.settings.workspaceProviders?.[workspace.id] ?? ''} onChange={(event) => {
								const next = { ...(ai.settings?.workspaceProviders ?? {}) };
								if (event.target.value) next[workspace.id] = event.target.value;
								else delete next[workspace.id];
								ai.setSettings({ ...(ai.settings as AISettings), workspaceProviders: next });
							}}>
								<option value="">Default ({ai.settings.defaultProvider || 'none'})</option>
								{Object.entries(ai.settings.providers).filter(([, template]) => template.enabled).map(([id]) => <option key={id} value={id}>{id}</option>)}
							</select>
						</label>
						<div className="settings-actions">
							{ai.saved && <span role="status">Provider saved.</span>}
							<button className="primary" type="button" onClick={() => void ai.save()} disabled={ai.saving}>
								<Save size={15} /> {ai.saving ? 'Saving...' : 'Save provider'}
							</button>
						</div>
					</>
				)}
				{ai.error && <p className="settings-error" role="alert">{ai.error}</p>}
			</section>
			{showHarness && <HarnessSettingsSection key={`${workspace.id}:${reload}`} workspaceId={workspace.id} />}
			{showHarness && <SettingsFilePanel workspaceId={workspace.id} workspaceName={workspace.name} onImported={() => { setReload((value) => value + 1); void ai.refresh(); }} />}
		</section>
	);
}
