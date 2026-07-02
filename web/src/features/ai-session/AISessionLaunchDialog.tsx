import { useEffect, useRef, useState } from 'react';
import { Bot, X } from 'lucide-react';
import { api } from '../../lib/api';
import type { AICapability, AISessionEligibility, AISettings, AISessionLaunchInput, AISessionLaunchResult } from '../../lib/types';

export function AISessionLaunchDialog({ itemId, preference, onClose, onLaunched }: { itemId: string; preference?: AISessionLaunchInput | null; onClose: () => void; onLaunched: (result: AISessionLaunchResult) => void }) {
  const [settings, setSettings] = useState<AISettings | null>(null);
  const [capabilities, setCapabilities] = useState<AICapability[]>([]);
  const [eligibility, setEligibility] = useState<AISessionEligibility | null>(null);
  const [provider, setProvider] = useState('');
  const [terminal, setTerminal] = useState('');
  const [contextMode, setContextMode] = useState<AISessionLaunchInput['contextMode']>('card_context');
  const [loading, setLoading] = useState(true);
  const [launching, setLaunching] = useState(false);
  const [error, setError] = useState('');
  const closeRef = useRef<HTMLButtonElement | null>(null);
  const dialogRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    let active = true;
    Promise.all([api.aiSettings(), api.aiCapabilities(), api.aiSessionEligibility(itemId)]).then(([nextSettings, nextCapabilities, nextEligibility]) => {
      if (!active) return;
      setSettings(nextSettings);
      setCapabilities(nextCapabilities);
      setEligibility(nextEligibility);
      setProvider(preference?.provider ?? nextSettings.defaultProvider);
      setTerminal(preference?.terminal ?? nextSettings.defaultTerminal);
      setContextMode(preference?.contextMode ?? 'card_context');
    }).catch((caught) => active && setError(caught instanceof Error ? caught.message : 'AI session options are unavailable.')).finally(() => active && setLoading(false));
    return () => { active = false; };
  }, [itemId, preference]);

  useEffect(() => {
    closeRef.current?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !launching) onClose();
      if (event.key !== 'Tab' || !dialogRef.current) return;
      const controls = Array.from(dialogRef.current.querySelectorAll<HTMLElement>('button:not([disabled]), select:not([disabled]), input:not([disabled])'));
      if (controls.length === 0) return;
      const first = controls[0];
      const last = controls[controls.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [launching, onClose]);

  const launch = async () => {
    setLaunching(true);
    setError('');
    try {
      const result = await api.launchAISession(itemId, { provider, terminal, contextMode });
      onLaunched(result);
      onClose();
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : 'AI session launch failed.');
    } finally {
      setLaunching(false);
    }
  };

  const providers = toolOptions(settings?.providers, capabilities, 'provider');
  const terminals = toolOptions(settings?.terminals, capabilities, 'terminal');
  const canLaunch = !loading && !launching && (contextMode === 'workspace_only' || eligibility?.cardContextAvailable) && providers.some((item) => item.id === provider) && terminals.some((item) => item.id === terminal);

  return (
    <div className="modal-backdrop ai-launch-backdrop" role="presentation">
      <section ref={dialogRef} className="modal-panel ai-launch-dialog" role="dialog" aria-modal="true" aria-labelledby="ai-launch-title">
        <header><div><h2 id="ai-launch-title"><Bot size={19} /> Open AI session</h2><span>Start an interactive CLI with this workspace and item context.</span></div><button ref={closeRef} className="icon-button" type="button" aria-label="Close AI session dialog" disabled={launching} onClick={onClose}><X size={18} /></button></header>
        {loading && <p role="status">Loading available tools...</p>}
        {error && <p className="error" role="alert">{error}</p>}
        {settings && eligibility && <div className="ai-launch-fields">
          <label>AI provider<select value={provider} onChange={(event) => setProvider(event.target.value)}>{providers.map((item) => <option key={item.id} value={item.id}>{label(item.id)}</option>)}</select></label>
          <label>Terminal<select value={terminal} onChange={(event) => setTerminal(event.target.value)}>{terminals.map((item) => <option key={item.id} value={item.id}>{label(item.id)}</option>)}</select></label>
          <fieldset><legend>Session context</legend><label><input type="radio" name="ai-context" checked={contextMode === 'workspace_only'} onChange={() => setContextMode('workspace_only')} /> Workspace only — start with a free prompt</label><label><input type="radio" name="ai-context" checked={contextMode === 'card_context'} disabled={!eligibility.cardContextAvailable} aria-describedby="card-context-readiness" onChange={() => setContextMode('card_context')} /> Selected card — provide its path and related documents</label></fieldset>
          {contextMode === 'workspace_only' && <p className="eligibility-ready">No card context will be injected. The AI opens at the workspace root so you can manually reference any relevant file or directory.</p>}
          {contextMode === 'card_context' && <p id="card-context-readiness" className={eligibility.cardContextAvailable ? 'eligibility-ready' : 'eligibility-blocked'}>{eligibility.cardContextAvailable ? 'The selected card path will be provided as context. The AI will read relevant documents from that path and wait for your request.' : `Card context unavailable: ${eligibility.missing.join(', ') || 'the card is not available in the working tree'}.`}</p>}
          {!eligibility.editable && contextMode !== 'workspace_only' && <p className="error">Card context requires an editable working-tree item.</p>}
          {(providers.length === 0 || terminals.length === 0) && <p className="error">Enable and detect at least one AI provider and terminal in Settings.</p>}
        </div>}
        <footer className="modal-actions"><button className="ghost" type="button" disabled={launching} onClick={onClose}>Cancel</button><button className="primary" type="button" disabled={!canLaunch} onClick={() => void launch()}>{launching ? 'Opening...' : 'Open session'}</button></footer>
      </section>
    </div>
  );
}

function toolOptions(templates: Record<string, { enabled: boolean }> | undefined, capabilities: AICapability[], kind: 'provider' | 'terminal') {
  return Object.keys(templates ?? {}).filter((id) => templates?.[id].enabled && capabilities.some((item) => item.kind === kind && item.id === id && item.detected)).map((id) => ({ id }));
}

function label(id: string) {
  return ({ claude: 'Claude', codex: 'Codex', copilot: 'Copilot', opencode: 'OpenCode', terminal: 'Terminal', iterm2: 'iTerm2', wezterm: 'WezTerm' } as Record<string, string>)[id] ?? id;
}
