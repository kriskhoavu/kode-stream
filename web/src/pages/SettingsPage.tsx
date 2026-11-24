import { useState } from 'react';
import { Bot, Database, Palette, RefreshCw, Save, Settings as SettingsIcon, TerminalSquare } from 'lucide-react';
import { statusLabels, statusOrder } from '../shared/api';
import type { ItemStatus } from '../lib/types';
import type { AppSettings } from '../features/settings/appSettings';
import { useAISettings } from '../features/ai-settings/useAISettings';
import type { AICapabilityKind, AILaunchTemplate, AISettings } from '../lib/types';
import { StorageSettings } from '../features/settings/StorageSettings';
import { backdropVariants } from '../components/HeaderBackdrop';
import { HarnessSettingsSection } from '../features/workstream/HarnessSettingsSection';
import '../features/harness/harness.css';
import '../features/settings/settings.css';
import type { BackdropVariant } from '../components/HeaderBackdrop';

// Settings shows no backdrop, so each description stands in for a preview.
const backdropLabels: Record<BackdropVariant, { name: string; description: string }> = {
  lattice: { name: 'Lattice', description: 'A blueprint grid with a node network drawn along it.' },
  neon: { name: 'Neon', description: 'A scattered field of glowing tool icons.' },
  none: { name: 'None', description: 'No backdrop. The topbar keeps its own background and border.' }
};

type SettingsArea = 'appearance' | 'ai' | 'harness' | 'storage';

const areas: { id: SettingsArea; label: string; hint: string; icon: typeof Palette }[] = [
  { id: 'appearance', label: 'Appearance', hint: 'Board columns and header backdrop', icon: Palette },
  { id: 'ai', label: 'AI tools', hint: 'Providers and terminals', icon: TerminalSquare },
  { id: 'harness', label: 'Harness', hint: 'Defaults and role pauses', icon: Bot },
  { id: 'storage', label: 'Storage', hint: 'Where app state lives', icon: Database }
];

export function SettingsPage({ settings, onChange, showHarness = true }: { settings: AppSettings; onChange: (settings: AppSettings) => void; showHarness?: boolean }) {
  const visible = new Set(settings.visibleWorkstreamStatuses);
  const ai = useAISettings();
  const shownAreas = areas.filter((area) => showHarness || area.id !== 'harness');
  const [area, setArea] = useState<SettingsArea>('appearance');
  const current = shownAreas.find((candidate) => candidate.id === area) ?? shownAreas[0];
  const toggleStatus = (status: ItemStatus) => {
    const next = visible.has(status)
      ? settings.visibleWorkstreamStatuses.filter((item) => item !== status)
      : statusOrder.filter((item) => item === status || visible.has(item));
    onChange({ ...settings, visibleWorkstreamStatuses: next.length > 0 ? next : [status] });
  };

  return (
    <section className="settings-page settings-layout">
      <header className="settings-title">
        <h1><SettingsIcon size={22} /> Settings</h1>
        <p>How Kode Stream looks and runs on this machine, and the harness defaults. A workspace's own provider, harness choices, models and sources are on its page under Workspaces.</p>
      </header>

      <div className="settings-shell">
        <nav className="settings-nav" aria-label="Settings areas">
          {shownAreas.map(({ id, label, hint, icon: Icon }) => (
            <button key={id} type="button" className={id === current.id ? 'active' : undefined} aria-current={id === current.id ? 'page' : undefined} onClick={() => setArea(id)}>
              <Icon size={16} aria-hidden="true" />
              <span><strong>{label}</strong><small>{hint}</small></span>
            </button>
          ))}
        </nav>
        <label className="settings-nav-select">
          <span>Settings area</span>
          <select value={current.id} onChange={(event) => setArea(event.target.value as SettingsArea)}>
            {shownAreas.map(({ id, label }) => <option key={id} value={id}>{label}</option>)}
          </select>
        </label>

        <div className={`settings-area settings-area-${current.id}`}>
          {current.id === 'appearance' && <>
            <section className="settings-section">
              <header>
                <div>
                  <span className="settings-group-label">Board View</span>
                  <h2>Status columns</h2>
                  <p>Show or hide the statuses that appear as board columns. Saved as you change them.</p>
                </div>
                <span className="settings-count">{settings.visibleWorkstreamStatuses.length} visible</span>
              </header>
              <div className="settings-toggle-list settings-toggle-grid">
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
              <div className="settings-toggle-list settings-toggle-grid" role="radiogroup" aria-label="Header backdrop">
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
          </>}

          {current.id === 'ai' && <section className="settings-section ai-settings-section">
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
                <div className="ai-tool-groups">
                  <ToolSettingsGroup kind="provider" templates={ai.settings.providers} settings={ai.settings} capabilities={ai.capabilities} onChange={ai.setSettings} />
                  <ToolSettingsGroup kind="terminal" templates={ai.settings.terminals} settings={ai.settings} capabilities={ai.capabilities} onChange={ai.setSettings} />
                </div>
                <div className="settings-actions">
                  {ai.saved && <span role="status">AI settings saved.</span>}
                  <button className="primary" type="button" onClick={() => void ai.save()} disabled={ai.saving}>
                    <Save size={15} /> {ai.saving ? 'Saving...' : 'Save AI settings'}
                  </button>
                </div>
              </>
            )}
          </section>}

          {current.id === 'harness' && <HarnessSettingsSection />}

          {current.id === 'storage' && <StorageSettings />}
        </div>
      </div>
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
            <details className="ai-tool-command">
              <summary><span>Command</span><code>{[template.executable, ...args].filter(Boolean).join(' ') || 'Not set'}</code></summary>
              <label>Executable<input value={template.executable} onChange={(event) => update(id, { ...template, executable: event.target.value })} /></label>
              <label>Arguments, one per line<textarea rows={2} value={args.join('\n')} onChange={(event) => update(id, { ...template, args: event.target.value.split('\n') })} /></label>
            </details>
          </div>
        );
      })}
    </fieldset>
  );
}

function toolLabel(id: string) {
  return ({ claude: 'Claude', codex: 'Codex', copilot: 'Copilot', opencode: 'OpenCode', terminal: 'Terminal', iterm2: 'iTerm2', wezterm: 'WezTerm' } as Record<string, string>)[id] ?? id;
}
