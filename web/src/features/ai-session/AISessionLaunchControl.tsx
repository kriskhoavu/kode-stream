import { useState } from 'react';
import { Bot, Settings2 } from 'lucide-react';
import { api } from '../../lib/api';
import type { AISessionLaunchInput, AISessionLaunchResult, EmbeddedAISessionResult } from '../../lib/types';
import { AISessionLaunchDialog } from './AISessionLaunchDialog';
import { readAISessionPreference, saveAISessionPreference } from './preferences';
import { openEmbeddedSession } from './terminalSessions';

export function AISessionLaunchControl({ itemId, disabled, onLaunched, onError, buttonLabel = 'Open AI session' }: { itemId: string; disabled?: boolean; onLaunched: (message: string) => void; onError: (error: unknown) => void; buttonLabel?: string }) {
  const [preference, setPreference] = useState<AISessionLaunchInput | null>(readAISessionPreference);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [launching, setLaunching] = useState(false);
  const savedChoice = preference ? preferenceLabel(preference) : '';

  const quickLaunch = async () => {
    if (!preference) {
      setDialogOpen(true);
      return;
    }
    setLaunching(true);
	try {
		if (preference.surface === 'embedded') {
			const result = await api.startEmbeddedAISession(itemId, { provider: preference.provider, contextMode: preference.contextMode, presetId: preference.presetId, promptDraft: preference.promptDraft, customPrompt: preference.customPrompt, selectedSkills: undefined, selectedAgents: undefined, columns: 80, rows: 24 });
			openEmbeddedSession(result); onLaunched(`${label(preference.provider)} opened in the embedded terminal.`);
		} else {
			const result = await api.launchAISession(itemId, { provider: preference.provider, terminal: preference.terminal, contextMode: preference.contextMode, presetId: preference.presetId, promptDraft: preference.promptDraft, customPrompt: preference.customPrompt, selectedSkills: undefined, selectedAgents: undefined });
			onLaunched(launchMessage(result));
		}
    } catch (caught) {
      onError(caught);
      setDialogOpen(true);
    } finally {
      setLaunching(false);
    }
  };

  const rememberLaunch = (result: AISessionLaunchResult | EmbeddedAISessionResult, input: AISessionLaunchInput) => {
		const next = { provider: input.provider, terminal: input.terminal, contextMode: input.contextMode, surface: input.surface ?? 'external', presetId: input.presetId, promptDraft: input.promptDraft, customPrompt: input.customPrompt };
    saveAISessionPreference(next);
    setPreference(next);
		if ('session' in result) { openEmbeddedSession(result); onLaunched(`${label(input.provider)} opened in the embedded terminal.`); }
		else onLaunched(launchMessage(result));
  };

  return <>
    <div className="ai-launch-split">
      <button className={`primary ai-launch-main${preference ? ' ai-launch-main-saved' : ''}`} type="button" disabled={disabled || launching} aria-label={preference ? `Open AI session using saved choice: ${savedChoice}` : 'Open AI session'} title={preference ? `Saved choice: ${savedChoice}` : 'Configure your first AI session'} onClick={() => void quickLaunch()}><Bot size={16} /> {launching ? 'Opening...' : buttonLabel} {preference && <span className="ai-launch-saved-indicator" aria-hidden="true" />}</button>
      <button className="primary ai-launch-settings" type="button" disabled={disabled || launching} aria-label="Configure AI session" title="Configure AI session" onClick={() => setDialogOpen(true)}><Settings2 size={16} /></button>
    </div>
    {dialogOpen && <AISessionLaunchDialog itemId={itemId} preference={preference} onClose={() => setDialogOpen(false)} onLaunched={rememberLaunch} />}
  </>;
}

function launchMessage(result: AISessionLaunchResult) {
  return `${label(result.provider)} opened in ${label(result.terminal)} with ${result.contextMode === 'card_context' ? 'card context' : 'workspace context'}.`;
}

function preferenceLabel(preference: AISessionLaunchInput) {
	const context = preference.contextMode === 'card_context' ? 'selected card' : 'workspace only';
	const surface = preference.surface === 'embedded' ? 'Embedded' : label(preference.terminal);
	const prompt = preference.presetId ? ` · ${preference.presetId}` : preference.promptDraft || preference.customPrompt ? ' · free prompt' : '';
	return `${label(preference.provider)} · ${surface} · ${context}${prompt}`;
}

function label(id: string) {
  return ({ claude: 'Claude', codex: 'Codex', copilot: 'Copilot', opencode: 'OpenCode', terminal: 'Terminal', iterm2: 'iTerm2', wezterm: 'WezTerm' } as Record<string, string>)[id] ?? id;
}
