import { Bot, TerminalSquare } from 'lucide-react';
import './agentChip.css';

const toolLabels: Record<string, string> = { claude: 'Claude', codex: 'Codex', copilot: 'Copilot', opencode: 'OpenCode', terminal: 'Terminal', iterm2: 'iTerm2', wezterm: 'WezTerm' };

export function toolLabel(id: string): string {
	return toolLabels[id] ?? id;
}

export const providerLabel = toolLabel;

export function AgentChip({ live = false, hollow = false, size = 22 }: { live?: boolean; hollow?: boolean; size?: number }) {
	const Icon = hollow ? TerminalSquare : Bot;
	return <span className={`agent-chip${live ? ' live' : ''}${hollow ? ' hollow' : ''}`} style={{ width: size, height: size }} aria-hidden="true">
		<Icon size={Math.round(size * 0.62)} strokeWidth={2} />
	</span>;
}
