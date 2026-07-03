import { useEffect, useMemo, useState } from 'react';
import { Bot, Maximize2 } from 'lucide-react';
import type { EmbeddedAISessionResult, WorkspaceConfig } from '../../lib/types';
import { EmbeddedTerminal } from './EmbeddedTerminal';
import { embeddedSessionStartedEvent } from './terminalSessions';

type DockMode = 'normal' | 'maximized' | 'minimized';

export function EmbeddedTerminalDock({ workspaces }: { workspaces: WorkspaceConfig[] }) {
	const [sessions, setSessions] = useState<EmbeddedAISessionResult[]>([]);
	const [activeId, setActiveId] = useState('');
	const [mode, setMode] = useState<DockMode>('normal');
	const workspaceNames = useMemo(() => new Map(workspaces.map((workspace) => [workspace.id, workspace.name])), [workspaces]);

	useEffect(() => {
		const add = (event: Event) => {
			const result = (event as CustomEvent<EmbeddedAISessionResult>).detail;
			setSessions((current) => current.some(({ session }) => session.id === result.session.id) ? current : [...current, result]);
			setActiveId(result.session.id);
			setMode('normal');
		};
		window.addEventListener(embeddedSessionStartedEvent, add);
		return () => window.removeEventListener(embeddedSessionStartedEvent, add);
	}, []);

	if (sessions.length === 0) return null;
	const active = sessions.find(({ session }) => session.id === activeId) ?? sessions[0];
	const select = (id: string) => { setActiveId(id); };
	const close = (id: string) => {
		setSessions((current) => current.filter(({ session }) => session.id !== id));
		if (id === active.session.id) {
			const remaining = sessions.filter(({ session }) => session.id !== id);
			setActiveId(remaining[0]?.session.id ?? '');
		}
	};

	return <>
		{mode === 'minimized' && <button className="embedded-terminal-minimized" type="button" aria-label={`Restore embedded terminal, ${sessions.length} open session${sessions.length === 1 ? '' : 's'}`} onClick={() => setMode('normal')}><span className="embedded-terminal-minimized-icon"><Bot size={17} /><i aria-hidden="true" /></span><span><strong>{sessionLabel(active)}</strong><small>{workspaceContext(active, workspaceNames)} · {sessions.length} open</small></span><Maximize2 size={17} /></button>}
		<div className={`embedded-terminal-backdrop terminal-mode-${mode}`} hidden={mode === 'minimized'}>
			<section className="embedded-terminal-shell" aria-label="Embedded terminal dock">
				<nav className="embedded-terminal-tabs" aria-label="Open embedded sessions">{sessions.map((result) => <button key={result.session.id} type="button" className={result.session.id === active.session.id ? 'active' : ''} onClick={() => select(result.session.id)}><Bot size={14} /> {sessionLabel(result)}<small>{workspaceName(result, workspaceNames)}</small></button>)}</nav>
				{sessions.map((result) => <EmbeddedTerminal key={result.session.id} initial={result} visible={mode !== 'minimized' && result.session.id === active.session.id} mode={mode === 'maximized' ? 'maximized' : 'normal'} title={`${providerLabel(result.session.provider)} terminal`} subtitle={workspaceContext(result, workspaceNames)} onToggleMinimize={() => setMode('minimized')} onToggleMaximize={() => setMode((current) => current === 'maximized' ? 'normal' : 'maximized')} onClose={() => close(result.session.id)} />)}
			</section>
		</div>
	</>;
}

function sessionLabel(result: EmbeddedAISessionResult) {
	return `${providerLabel(result.session.provider)} · ${result.session.itemIdentifier ?? result.session.itemId}`;
}

function workspaceContext(result: EmbeddedAISessionResult, workspaceNames: Map<string, string>) {
	const workspace = workspaceName(result, workspaceNames);
	const item = result.session.itemIdentifier ?? result.session.itemTitle;
	return item ? `${workspace} · ${item}` : workspace;
}

function workspaceName(result: EmbeddedAISessionResult, workspaceNames: Map<string, string>) {
	return workspaceNames.get(result.session.workspaceId) ?? result.session.workspaceId;
}

function providerLabel(provider: string) {
	return ({ codex: 'Codex', claude: 'Claude', copilot: 'Copilot', opencode: 'OpenCode' } as Record<string, string>)[provider] ?? provider;
}
