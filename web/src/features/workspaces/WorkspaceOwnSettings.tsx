import { Save } from 'lucide-react';
import type { AISettings, WorkspaceConfig } from '../../lib/types';
import { providerFor } from '../../lib/types';
import { useAISettings } from '../ai-settings/useAISettings';
import { HarnessSettingsSection } from '../workstream/HarnessSettingsSection';
import '../harness/harness.css';

/**
 * One workspace's own settings, on its page under Workspaces: the provider a
 * new session starts with, and the harness's choices and models for it. Its
 * sources sit with its Jira connection under Integrations, and the settings
 * file opens from the page's header. App-wide settings stay under Settings.
 */
export function WorkspaceOwnSettings({ workspace, showHarness = true }: { workspace: WorkspaceConfig; /** The harness is absent on the extension surface. */ showHarness?: boolean }) {
	const ai = useAISettings();
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
							<span><strong>Provider here</strong><small>A workspace starts with Claude until you choose another.</small></span>
							<select aria-label="Provider here" value={providerFor(ai.settings, workspace.id)} onChange={(event) => {
								ai.setSettings({ ...(ai.settings as AISettings), workspaceProviders: { ...(ai.settings?.workspaceProviders ?? {}), [workspace.id]: event.target.value } });
							}}>
								{Object.entries(ai.settings.providers).filter(([id, template]) => template.enabled || id === providerFor(ai.settings as AISettings, workspace.id)).map(([id]) => <option key={id} value={id}>{id}</option>)}
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
			{showHarness && <HarnessSettingsSection workspaceId={workspace.id} />}
		</section>
	);
}
