import { useRef, useState, type KeyboardEvent } from 'react';
import { Save } from 'lucide-react';
import type { AISettings, WorkspaceConfig } from '../../lib/types';
import { providerFor } from '../../lib/types';
import { useAISettings } from '../ai-settings/useAISettings';
import { HarnessSettingsSection } from '../workstream/HarnessSettingsSection';
import '../harness/harness.css';

type OwnSettingsTab = 'tools' | 'models';
const tabs: { id: OwnSettingsTab; label: string }[] = [{ id: 'tools', label: 'Tools and harness' }, { id: 'models', label: 'Models and skills' }];

/**
 * One workspace's own settings, on its page under Workspaces: the provider a
 * new session starts with, and the harness's choices and models for it. Its
 * sources sit with its Jira connection under Integrations, and the settings
 * file opens from the page's header. App-wide settings stay under Settings.
 * Two tabs hold them: the provider and the harness's choices, then the
 * models and role skills. One harness form spans both, so an edit on one tab
 * survives a switch to the other and one Save keeps both.
 */
export function WorkspaceOwnSettings({ workspace, showHarness = true }: { workspace: WorkspaceConfig; /** The harness is absent on the extension surface. */ showHarness?: boolean }) {
	const ai = useAISettings();
	const [tab, setTab] = useState<OwnSettingsTab>('tools');
	const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);
	const onTabKey = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
		if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return;
		event.preventDefault();
		const next = (index + (event.key === 'ArrowRight' ? 1 : tabs.length - 1)) % tabs.length;
		setTab(tabs[next].id);
		tabRefs.current[next]?.focus();
	};
	return (
		<section className="settings-page workspace-own-settings">
			{showHarness && (
				<div className="hx-tabs" role="tablist" aria-label={`Settings for ${workspace.name}`}>
					{tabs.map((item, index) => (
						<button key={item.id} ref={(element) => { tabRefs.current[index] = element; }} type="button" role="tab" id={`own-tab-${item.id}`} aria-selected={tab === item.id} tabIndex={tab === item.id ? 0 : -1} className={tab === item.id ? 'on' : undefined} onClick={() => setTab(item.id)} onKeyDown={(event) => onTabKey(event, index)}>{item.label}</button>
					))}
				</div>
			)}
			{(tab === 'tools' || !showHarness) && <section className="settings-section ai-settings-section">
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
			</section>}
			{showHarness && <HarnessSettingsSection workspaceId={workspace.id} view={tab === 'tools' ? 'choices' : 'models'} />}
		</section>
	);
}
