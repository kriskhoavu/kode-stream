import { Bot } from 'lucide-react';
import './agentChip.css';

const toolLabels: Record<string, string> = { claude: 'Claude', codex: 'Codex', copilot: 'Copilot', opencode: 'OpenCode', terminal: 'Terminal', iterm2: 'iTerm2', wezterm: 'WezTerm' };
const providerMonograms: Record<string, string> = { claude: 'C', codex: 'Cx', copilot: 'Cp', opencode: 'Oc' };

/** Display name of an AI provider or external terminal; an unknown ID shows as itself. */
export function toolLabel(id: string): string {
	return toolLabels[id] ?? id;
}

export const providerLabel = toolLabel;

/**
 * A round provider mark: a short monogram for the known providers, a robot for
 * any other. `live` adds the running ring; `hollow` marks a conversation held
 * outside kode-stream, which this app does not run.
 */
export function AgentChip({ provider, live = false, hollow = false, size = 22 }: { provider: string; live?: boolean; hollow?: boolean; size?: number }) {
	const monogram = providerMonograms[provider];
	return <span className={`agent-chip provider-${monogram ? provider : 'other'}${live ? ' live' : ''}${hollow ? ' hollow' : ''}`} style={{ width: size, height: size }} aria-hidden="true">
		{monogram ?? <Bot size={Math.round(size * 0.6)} />}
	</span>;
}
