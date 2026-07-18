import type { AISessionLaunchInput } from '../../lib/types';

const storageKey = 'aiSession.lastLaunch';

export function readAISessionPreference(): AISessionLaunchInput | null {
  try {
    const value = JSON.parse(localStorage.getItem(storageKey) ?? 'null') as Partial<AISessionLaunchInput> | null;
    if (!value || typeof value.provider !== 'string' || !value.provider || typeof value.terminal !== 'string' || !value.terminal) return null;
    if (value.contextMode !== 'workspace_only' && value.contextMode !== 'card_context') return null;
		if (value.surface !== undefined && value.surface !== 'external' && value.surface !== 'embedded') return null;
    return {
			provider: value.provider,
			terminal: value.terminal,
			contextMode: value.contextMode,
			surface: value.surface ?? 'external',
			presetId: typeof value.presetId === 'string' ? value.presetId : undefined,
			promptDraft: typeof value.promptDraft === 'string' ? value.promptDraft : undefined,
			customPrompt: typeof value.customPrompt === 'string' ? value.customPrompt : undefined,
			includeJiraDescription: value.includeJiraDescription === true
		};
  } catch {
    return null;
  }
}

export function saveAISessionPreference(value: AISessionLaunchInput) {
  localStorage.setItem(storageKey, JSON.stringify(value));
}
