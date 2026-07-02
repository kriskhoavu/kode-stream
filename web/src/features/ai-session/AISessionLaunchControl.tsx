import { useState } from 'react';
import { Bot, Settings2 } from 'lucide-react';
import { api } from '../../lib/api';
import type { AISessionLaunchInput, AISessionLaunchResult } from '../../lib/types';
import { AISessionLaunchDialog } from './AISessionLaunchDialog';
import { readAISessionPreference, saveAISessionPreference } from './preferences';

export function AISessionLaunchControl({ itemId, disabled, onLaunched, onError }: { itemId: string; disabled?: boolean; onLaunched: (message: string) => void; onError: (error: unknown) => void }) {
  const [preference, setPreference] = useState<AISessionLaunchInput | null>(readAISessionPreference);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [launching, setLaunching] = useState(false);

  const quickLaunch = async () => {
    if (!preference) {
      setDialogOpen(true);
      return;
    }
    setLaunching(true);
    try {
      const result = await api.launchAISession(itemId, preference);
      onLaunched(launchMessage(result));
    } catch (caught) {
      onError(caught);
      setDialogOpen(true);
    } finally {
      setLaunching(false);
    }
  };

  const rememberLaunch = (result: AISessionLaunchResult) => {
    const next = { provider: result.provider, terminal: result.terminal, contextMode: result.contextMode };
    saveAISessionPreference(next);
    setPreference(next);
    onLaunched(launchMessage(result));
  };

  return <>
    <div className="ai-launch-split">
      <button className="primary ai-launch-main" type="button" disabled={disabled || launching} onClick={() => void quickLaunch()}><Bot size={16} /> {launching ? 'Opening...' : 'Open AI session'}</button>
      <button className="primary ai-launch-settings" type="button" disabled={disabled || launching} aria-label="Configure AI session" title="Configure AI session" onClick={() => setDialogOpen(true)}><Settings2 size={16} /></button>
    </div>
    {dialogOpen && <AISessionLaunchDialog itemId={itemId} preference={preference} onClose={() => setDialogOpen(false)} onLaunched={rememberLaunch} />}
  </>;
}

function launchMessage(result: AISessionLaunchResult) {
  return `${label(result.provider)} opened in ${label(result.terminal)} with ${result.contextMode === 'card_context' ? 'card context' : 'workspace context'}.`;
}

function label(id: string) {
  return ({ claude: 'Claude', codex: 'Codex', copilot: 'Copilot', opencode: 'OpenCode', terminal: 'Terminal', iterm2: 'iTerm2', wezterm: 'WezTerm' } as Record<string, string>)[id] ?? id;
}
