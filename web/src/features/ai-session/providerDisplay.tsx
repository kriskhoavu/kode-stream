import { Bot, TerminalSquare } from 'lucide-react';
import './agentChip.css';

const toolLabels: Record<string, string> = { claude: 'Claude', codex: 'Codex', copilot: 'Copilot', opencode: 'OpenCode', terminal: 'Terminal', iterm2: 'iTerm2', wezterm: 'WezTerm' };

/** Display name of an AI provider or external terminal; an unknown ID shows as itself. */
export function toolLabel(id: string): string {
	return toolLabels[id] ?? id;
}

export const providerLabel = toolLabel;

/**
 * A round agent mark: a robot for a session kode-stream runs, green with a
 * ring while it is live, and a terminal glyph on a dashed outline for a
 * conversation held in the user's own terminal. The provider is named in the
 * surrounding label or tooltip, not in the mark.
 */
export function AgentChip({ live = false, hollow = false, size = 22 }: { live?: boolean; hollow?: boolean; size?: number }) {
	const Icon = hollow ? TerminalSquare : Bot;
	return <span className={`agent-chip${live ? ' live' : ''}${hollow ? ' hollow' : ''}`} style={{ width: size, height: size }} aria-hidden="true">
		<Icon size={Math.round(size * 0.62)} strokeWidth={2} />
	</span>;
}
