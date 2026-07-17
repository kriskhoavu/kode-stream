import { RefreshCw, Save, Settings as SettingsIcon } from 'lucide-react';
import { statusLabels, statusOrder } from '../lib/api';
import type { ItemStatus } from '../lib/types';
import type { AppSettings } from '../features/settings/appSettings';
import { useAISettings } from '../features/ai-settings/useAISettings';
import type { AICapabilityKind, AILaunchTemplate, AISettings } from '../lib/types';
import { StorageSettings } from '../features/settings/StorageSettings';

export function SettingsPage({ settings, onChange }: { settings: AppSettings; onChange: (settings: AppSettings) => void }) {
  const visible = new Set(settings.visibleWorkstreamStatuses);
  const ai = useAISettings();

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
        <p>Configure how Kode Stream presents workspace and board metadata.</p>
      </header>

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
