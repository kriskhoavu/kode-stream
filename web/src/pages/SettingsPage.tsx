import { useEffect, useState } from 'react';
import { RefreshCw, Save, Settings as SettingsIcon } from 'lucide-react';
import { api, statusLabels, statusOrder } from '../shared/api';
import type { ItemStatus } from '../lib/types';
import type { AppSettings } from '../features/settings/appSettings';
import { useAISettings } from '../features/ai-settings/useAISettings';
import type { AICapabilityKind, AILaunchTemplate, AISettings, WorkspaceConfig } from '../lib/types';
import { SettingsFilePanel } from '../features/settings/SettingsFilePanel';
import { StorageSettings } from '../features/settings/StorageSettings';
import { backdropVariants } from '../components/HeaderBackdrop';
import { HarnessSettingsSection } from '../features/workstream/HarnessSettingsSection';
import '../features/harness/harness.css';
import type { BackdropVariant } from '../components/HeaderBackdrop';

/*
 * Settings is not a backdrop route, so choosing a variant shows no immediate
 * change here. The copy has to do the work a preview would.
 */
const backdropLabels: Record<BackdropVariant, { name: string; description: string }> = {
  lattice: { name: 'Lattice', description: 'A blueprint grid with a node network drawn along it.' },
  neon: { name: 'Neon', description: 'A scattered field of glowing tool icons.' },
  none: { name: 'None', description: 'No backdrop. The topbar keeps its own background and border.' }
};

export function SettingsPage({ settings, onChange, showHarness = true }: { settings: AppSettings; onChange: (settings: AppSettings) => void; /** The harness has no page of its own; its settings live here, except on the extension surface. */ showHarness?: boolean }) {
  const visible = new Set(settings.visibleWorkstreamStatuses);
  const ai = useAISettings();
  const [workspaces, setWorkspaces] = useState<WorkspaceConfig[]>([]);
  /** '' is the defaults every workspace starts from; a workspace id is that workspace's own choices. */
  const [scope, setScope] = useState('');
  const [reload, setReload] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    api.workspaces(controller.signal).then(setWorkspaces).catch(() => setWorkspaces([]));
    return () => controller.abort();
  }, []);
  const workspace = workspaces.find((candidate) => candidate.id === scope);

  const toggleStatus = (status: ItemStatus) => {
    const next = visible.has(status)
      ? settings.visibleWorkstreamStatuses.filter((item) => item !== status)
      : statusOrder.filter((item) => item === status || visible.has(item));
    onChange({ ...settings, visibleWorkstreamStatuses: next.length > 0 ? next : [status] });
  };

  return (
    <section className="settings-page">
      <header className="settings-title">
        <h1><SettingsIcon size={22} /> Settings</h1>
        <p>{workspace ? `${workspace.name}'s own choices. A value it does not set follows the defaults.` : 'The defaults every workspace starts from, and how Kode Stream looks on this machine.'}</p>
        {workspaces.length > 0 && (
          <label className="settings-scope">
            <span>Settings for</span>
            <select aria-label="Settings for" value={scope} onChange={(event) => setScope(event.target.value)}>
              <option value="">Defaults, for every workspace</option>
              {workspaces.map((candidate) => <option key={candidate.id} value={candidate.id}>{candidate.name}</option>)}
            </select>
          </label>
        )}
      </header>

      {workspace && (
        <>
          <section className="settings-section ai-settings-section">
            <header>
              <div>
                <span className="settings-group-label">Local AI tools</span>
                <h2>Provider in this workspace</h2>
                <p>The tool a new session in {workspace.name} starts with. Executables and terminals stay the same on this machine for every workspace.</p>
              </div>
            </header>
            {ai.settings && (
              <>
                <label className="settings-toggle-row">
                  <span><strong>Default provider here</strong><small>Blank follows the defaults.</small></span>
                  <select aria-label="Default provider here" value={ai.settings.workspaceProviders?.[workspace.id] ?? ''} onChange={(event) => {
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
                  {ai.saved && <span role="status">AI settings saved.</span>}
                  {ai.error && <p className="settings-error" role="alert">{ai.error}</p>}
                  <button className="primary" type="button" onClick={() => void ai.save()} disabled={ai.saving}>
                    <Save size={15} /> {ai.saving ? 'Saving...' : 'Save provider'}
                  </button>
                </div>
              </>
            )}
          </section>
          {showHarness && <HarnessSettingsSection key={`${workspace.id}:${reload}`} workspaceId={workspace.id} />}
          {showHarness && <SettingsFilePanel workspaceId={workspace.id} workspaceName={workspace.name} onImported={() => { setReload((value) => value + 1); void ai.refresh(); }} />}
        </>
      )}

      {!workspace && <>

      <section className="settings-section">
        <header>
          <div>
            <span className="settings-group-label">Board View</span>
            <h2>Status columns</h2>
            <p>Show or hide the statuses that appear as board columns.</p>
          </div>
          <span className="settings-count">{settings.visibleWorkstreamStatuses.length} visible</span>
        </header>
        <div className="settings-toggle-list">
          {statusOrder.map((status) => (
            <label className="settings-toggle-row" key={status}>
              <span>
                <strong>{statusLabels[status]}</strong>
                <small>{status}</small>
              </span>
              <input
                type="checkbox"
                checked={visible.has(status)}
                onChange={() => toggleStatus(status)}
                aria-label={`Show ${statusLabels[status]} status`}
              />
            </label>
          ))}
        </div>
      </section>

      <section className="settings-section">
        <header>
          <div>
            <span className="settings-group-label">Appearance</span>
            <h2>Header backdrop</h2>
            <p>The decoration behind the topbar and filter rows on Workstream, Agent Space and Knowledge.</p>
          </div>
        </header>
        <div className="settings-toggle-list" role="radiogroup" aria-label="Header backdrop">
          {backdropVariants.map((variant) => (
            <label className="settings-toggle-row" key={variant}>
              <span>
                <strong>{backdropLabels[variant].name}</strong>
                <small>{backdropLabels[variant].description}</small>
              </span>
              <input
                type="radio"
                name="header-backdrop"
                value={variant}
                checked={settings.headerBackdrop === variant}
                onChange={() => onChange({ ...settings, headerBackdrop: variant })}
                aria-label={backdropLabels[variant].name}
              />
            </label>
          ))}
        </div>
      </section>

      <StorageSettings />

      <section className="settings-section ai-settings-section">
        <header>
          <div>
            <span className="settings-group-label">Local AI tools</span>
            <h2>Providers and terminals</h2>
            <p>Use detected presets or override executable paths and argument templates for this machine.</p>
          </div>
          <button className="ghost" type="button" onClick={() => void ai.refresh()} disabled={ai.loading || ai.saving}>
            <RefreshCw size={15} /> Detect again
          </button>
        </header>
        {ai.loading && <p role="status">Detecting local AI tools...</p>}
        {ai.error && <p className="settings-error" role="alert">{ai.error}</p>}
        {ai.settings && (
          <>
            <ToolSettingsGroup kind="provider" templates={ai.settings.providers} settings={ai.settings} capabilities={ai.capabilities} onChange={ai.setSettings} />
            <ToolSettingsGroup kind="terminal" templates={ai.settings.terminals} settings={ai.settings} capabilities={ai.capabilities} onChange={ai.setSettings} />
            <div className="settings-actions">
              {ai.saved && <span role="status">AI settings saved.</span>}
              <button className="primary" type="button" onClick={() => void ai.save()} disabled={ai.saving}>
                <Save size={15} /> {ai.saving ? 'Saving...' : 'Save AI settings'}
              </button>
            </div>
          </>
        )}
      </section>
      {showHarness && <HarnessSettingsSection />}
      </>}
    </section>
  );
}

function ToolSettingsGroup({ kind, templates, settings, capabilities, onChange }: {
  kind: AICapabilityKind;
  templates: Record<string, AILaunchTemplate>;
  settings: AISettings;
  capabilities: ReturnType<typeof useAISettings>['capabilities'];
  onChange: (settings: AISettings) => void;
}) {
  const defaultKey = kind === 'provider' ? 'defaultProvider' : 'defaultTerminal';
  const collectionKey = kind === 'provider' ? 'providers' : 'terminals';
  const update = (id: string, template: AILaunchTemplate) => onChange({ ...settings, [collectionKey]: { ...templates, [id]: template } });
  const toggleEnabled = (id: string, enabled: boolean) => {
    const nextTemplates = { ...templates, [id]: { ...templates[id], enabled } };
    const nextDefault = settings[defaultKey] === id && !enabled
      ? Object.keys(nextTemplates).find((candidate) => nextTemplates[candidate].enabled) ?? ''
      : settings[defaultKey];
    onChange({ ...settings, [collectionKey]: nextTemplates, [defaultKey]: nextDefault });
  };
  return (
    <fieldset className="ai-tool-group">
      <legend>{kind === 'provider' ? 'AI providers' : 'Terminal applications'}</legend>
      {Object.entries(templates).sort(([left], [right]) => left.localeCompare(right)).map(([id, template]) => {
        const capability = capabilities.find((item) => item.kind === kind && item.id === id);
        const args = Array.isArray(template.args) ? template.args : [];
        return (
          <div className="ai-tool-row" key={id}>
            <div className="ai-tool-heading">
              <label><input type="radio" name={`default-${kind}`} checked={settings[defaultKey] === id} disabled={!template.enabled} onChange={() => onChange({ ...settings, [defaultKey]: id })} /> Default</label>
              <strong>{toolLabel(id)}</strong>
              <span className={capability?.detected ? 'tool-state detected' : 'tool-state missing'}>{capability?.detected ? 'Detected' : capability?.reason ?? 'Not detected'}</span>
              <label><input type="checkbox" checked={template.enabled} onChange={(event) => toggleEnabled(id, event.target.checked)} /> Enabled</label>
            </div>
            <label>Executable<input value={template.executable} onChange={(event) => update(id, { ...template, executable: event.target.value })} /></label>
            <label>Arguments, one per line<textarea rows={2} value={args.join('\n')} onChange={(event) => update(id, { ...template, args: event.target.value.split('\n') })} /></label>
          </div>
        );
      })}
    </fieldset>
  );
}

function toolLabel(id: string) {
  return ({ claude: 'Claude', codex: 'Codex', copilot: 'Copilot', opencode: 'OpenCode', terminal: 'Terminal', iterm2: 'iTerm2', wezterm: 'WezTerm' } as Record<string, string>)[id] ?? id;
}
