import { useEffect, useMemo, useRef, useState } from 'react';
import { Bot, Maximize2 } from 'lucide-react';
import type { EmbeddedAISessionResult, WorkspaceConfig } from '../../lib/types';
import { api } from '../../lib/api';
import { EmbeddedTerminal } from './EmbeddedTerminal';
import { embeddedSessionStartedEvent } from './terminalSessions';

type DockMode = 'floating' | 'side_panel' | 'maximized' | 'minimized';
type FloatingLayout = { x: number; y: number; width: number; height: number };

const minWidth = 520;
const minHeight = 340;
const minSidePanelWidth = 420;
const maxSidePanelInset = 280;
const defaultLayout = (): FloatingLayout => {
	const width = Math.min(980, Math.max(minWidth, Math.floor(window.innerWidth * 0.62)));
	const height = Math.min(720, Math.max(minHeight, Math.floor(window.innerHeight * 0.58)));
	return {
		width,
		height,
		x: Math.max(24, window.innerWidth - width - 32),
		y: Math.max(24, window.innerHeight - height - 72)
	};
};

export function EmbeddedTerminalDock({ workspaces }: { workspaces: WorkspaceConfig[] }) {
	const [sessions, setSessions] = useState<EmbeddedAISessionResult[]>([]);
	const [activeId, setActiveId] = useState('');
	const [mode, setMode] = useState<DockMode>('floating');
	const [lastExpandedMode, setLastExpandedMode] = useState<Exclude<DockMode, 'minimized'>>('floating');
	const [layout, setLayout] = useState<FloatingLayout>(() => defaultLayout());
	const [sidePanelWidth, setSidePanelWidth] = useState(() => defaultSidePanelWidth());
	const [autoTriggeredSessions, setAutoTriggeredSessions] = useState<Record<string, boolean>>({});
	const workspaceNames = useMemo(() => new Map(workspaces.map((workspace) => [workspace.id, workspace.name])), [workspaces]);
	const dragStateRef = useRef<{ pointerX: number; pointerY: number; layout: FloatingLayout; resizeCorner?: 'nw' | 'ne' | 'sw' | 'se'; sidePanelWidth?: number } | null>(null);
	const pointerCleanupRef = useRef<(() => void) | null>(null);

	useEffect(() => {
		const add = (event: Event) => {
			const result = (event as CustomEvent<EmbeddedAISessionResult>).detail;
			setSessions((current) => current.some(({ session }) => session.id === result.session.id) ? current : [...current, result]);
			setActiveId(result.session.id);
			setMode((current) => current === 'minimized' ? lastExpandedMode : current);
		};
		window.addEventListener(embeddedSessionStartedEvent, add);
		return () => window.removeEventListener(embeddedSessionStartedEvent, add);
	}, [lastExpandedMode]);

	useEffect(() => {
		if (mode === 'minimized') return;
		if (mode !== 'maximized') setLastExpandedMode(mode);
	}, [mode]);

	useEffect(() => {
		const onResizeViewport = () => {
			setLayout((current) => clampLayout(current));
			setSidePanelWidth((current) => clampSidePanelWidth(current));
		};
		window.addEventListener('resize', onResizeViewport);
		return () => window.removeEventListener('resize', onResizeViewport);
	}, []);

	useEffect(() => () => {
		pointerCleanupRef.current?.();
	}, []);

	if (sessions.length === 0) return null;
	const active = sessions.find(({ session }) => session.id === activeId) ?? sessions[0];
	const renderMode = resolveRenderMode(mode, sidePanelWidth);

	const select = (id: string) => { setActiveId(id); };
	const close = (id: string) => {
		setSessions((current) => {
			const remaining = current.filter(({ session }) => session.id !== id);
			setActiveId((currentActiveId) => currentActiveId === id ? remaining[0]?.session.id ?? '' : currentActiveId);
			return remaining;
		});
	};
	const restoreFromMinimized = () => setMode(lastExpandedMode);
	const toggleDockMode = () => setMode((current) => {
		if (current === 'side_panel') return 'floating';
		if (current === 'maximized') return 'side_panel';
		return 'side_panel';
	});
	const toggleMaximize = () => setMode((current) => current === 'maximized' ? lastExpandedMode : 'maximized');
	const beginMove = (pointerX: number, pointerY: number) => {
		if (mode !== 'floating') return;
		dragStateRef.current = { pointerX, pointerY, layout };
		pointerCleanupRef.current?.();
		pointerCleanupRef.current = attachPointerSession((event) => {
			const state = dragStateRef.current;
			if (!state) return;
			setLayout(clampLayout({
				...state.layout,
				x: state.layout.x + (event.clientX - state.pointerX),
				y: state.layout.y + (event.clientY - state.pointerY)
			}));
		}, () => {
			dragStateRef.current = null;
			pointerCleanupRef.current = null;
		});
	};
	const beginResize = (corner: 'nw' | 'ne' | 'sw' | 'se', pointerX: number, pointerY: number) => {
		if (mode !== 'floating') return;
		dragStateRef.current = { pointerX, pointerY, layout, resizeCorner: corner };
		pointerCleanupRef.current?.();
		pointerCleanupRef.current = attachPointerSession((event) => {
			const state = dragStateRef.current;
			if (!state) return;
			setLayout(clampLayout(resizeLayout(state.layout, state.resizeCorner ?? 'se', event.clientX - state.pointerX, event.clientY - state.pointerY)));
		}, () => {
			dragStateRef.current = null;
			pointerCleanupRef.current = null;
		});
	};
	const beginSidePanelResize = (pointerX: number) => {
		if (mode !== 'side_panel') return;
		dragStateRef.current = { pointerX, pointerY: 0, layout, sidePanelWidth };
		pointerCleanupRef.current?.();
		pointerCleanupRef.current = attachPointerSession((event) => {
			const state = dragStateRef.current;
			if (!state || state.sidePanelWidth === undefined) return;
			setSidePanelWidth(clampSidePanelWidth(state.sidePanelWidth - (event.clientX - state.pointerX)));
		}, () => {
			dragStateRef.current = null;
			pointerCleanupRef.current = null;
		});
	};

	return <>
		{mode === 'minimized' && <button className="embedded-terminal-minimized" type="button" aria-label={`Restore embedded terminal, ${sessions.length} open session${sessions.length === 1 ? '' : 's'}`} onClick={restoreFromMinimized}><span className="embedded-terminal-minimized-icon"><Bot size={17} /><i aria-hidden="true" /></span><span><strong>{sessionLabel(active)}</strong><small>{workspaceContext(active, workspaceNames)} · {sessions.length} open</small></span><Maximize2 size={17} /></button>}
		<div className={`embedded-terminal-backdrop terminal-mode-${mode}`} hidden={mode === 'minimized'}>
			<section className={`embedded-terminal-shell embedded-terminal-shell-${renderMode}`} aria-label="Embedded terminal dock" style={shellStyle(renderMode, layout, sidePanelWidth)}>
				<nav className="embedded-terminal-tabs" aria-label="Open embedded sessions">{sessions.map((result) => <button key={result.session.id} type="button" className={result.session.id === active.session.id ? 'active' : ''} onClick={() => select(result.session.id)}><Bot size={14} /> {sessionLabel(result)}<small>{workspaceName(result, workspaceNames)}</small></button>)}</nav>
				{sessions.map((result) => <EmbeddedTerminal key={result.session.id} initial={result} visible={result.session.id === active.session.id} mode={renderMode === 'side_panel' ? 'side_panel' : renderMode === 'maximized' ? 'maximized' : 'floating'} title={`${providerLabel(result.session.provider)} terminal`} subtitle={workspaceContext(result, workspaceNames)} onStartMove={beginMove} onToggleDockMode={toggleDockMode} onToggleMinimize={() => setMode('minimized')} onToggleMaximize={toggleMaximize} onClose={() => close(result.session.id)} onStateChange={(state) => {
					if (state !== 'exited' && state !== 'failed' && state !== 'cancelled') return;
					if (autoTriggeredSessions[result.session.id]) return;
					const workspace = workspaces.find((candidate) => candidate.id === result.session.workspaceId);
					if (!workspace?.runtime) return;
					setAutoTriggeredSessions((current) => ({ ...current, [result.session.id]: true }));
					void api.ingestVerificationCheckpoint(result.session.workspaceId, {
						eventType: 'session_completed',
						profile: 'smoke',
						provider: result.session.provider,
						sessionId: result.session.id,
						terminalMode: 'embedded'
					});
				}} />)}
				{renderMode === 'side_panel' && <button className="embedded-terminal-side-resize" type="button" aria-label="Resize right-side terminal panel" onPointerDown={(event) => { event.preventDefault(); beginSidePanelResize(event.clientX); }} />}
				{renderMode === 'floating' && <>
					<button className="embedded-terminal-resize embedded-terminal-resize-nw" type="button" aria-label="Resize terminal from top left corner" onPointerDown={(event) => { event.preventDefault(); beginResize('nw', event.clientX, event.clientY); }} />
					<button className="embedded-terminal-resize embedded-terminal-resize-ne" type="button" aria-label="Resize terminal from top right corner" onPointerDown={(event) => { event.preventDefault(); beginResize('ne', event.clientX, event.clientY); }} />
					<button className="embedded-terminal-resize embedded-terminal-resize-sw" type="button" aria-label="Resize terminal from bottom left corner" onPointerDown={(event) => { event.preventDefault(); beginResize('sw', event.clientX, event.clientY); }} />
					<button className="embedded-terminal-resize embedded-terminal-resize-se" type="button" aria-label="Resize terminal from bottom right corner" onPointerDown={(event) => { event.preventDefault(); beginResize('se', event.clientX, event.clientY); }} />
				</>}
			</section>
		</div>
	</>;
}

function shellStyle(mode: DockMode, layout: FloatingLayout, sidePanelWidth: number) {
	if (mode === 'floating') return { left: `${layout.x}px`, top: `${layout.y}px`, width: `${layout.width}px`, height: `${layout.height}px` };
	if (mode === 'side_panel') return { right: '0', top: '0', width: `${sidePanelWidth}px`, height: '100dvh' };
	return { inset: '0', width: '100vw', height: '100dvh' };
}

function resolveRenderMode(mode: DockMode, sidePanelWidth: number): DockMode {
	if (mode !== 'side_panel') return mode;
	return window.innerWidth-sidePanelWidth < maxSidePanelInset ? 'maximized' : 'side_panel';
}

function clampLayout(layout: FloatingLayout): FloatingLayout {
	const maxWidth = Math.max(minWidth, window.innerWidth - 32);
	const maxHeight = Math.max(minHeight, window.innerHeight - 32);
	const width = Math.min(maxWidth, Math.max(minWidth, layout.width));
	const height = Math.min(maxHeight, Math.max(minHeight, layout.height));
	return {
		width,
		height,
		x: Math.min(window.innerWidth - width - 16, Math.max(16, layout.x)),
		y: Math.min(window.innerHeight - height - 16, Math.max(16, layout.y))
	};
}

function defaultSidePanelWidth() {
	return clampSidePanelWidth(Math.max(minSidePanelWidth, Math.floor(window.innerWidth * 0.36)));
}

function clampSidePanelWidth(width: number) {
	return Math.min(Math.max(minSidePanelWidth, width), Math.max(minSidePanelWidth, window.innerWidth - maxSidePanelInset));
}

function resizeLayout(layout: FloatingLayout, corner: 'nw' | 'ne' | 'sw' | 'se', deltaX: number, deltaY: number): FloatingLayout {
	if (corner === 'se') return { ...layout, width: layout.width + deltaX, height: layout.height + deltaY };
	if (corner === 'sw') return { x: layout.x + deltaX, y: layout.y, width: layout.width - deltaX, height: layout.height + deltaY };
	if (corner === 'ne') return { x: layout.x, y: layout.y + deltaY, width: layout.width + deltaX, height: layout.height - deltaY };
	return { x: layout.x + deltaX, y: layout.y + deltaY, width: layout.width - deltaX, height: layout.height - deltaY };
}

function attachPointerSession(onMove: (event: PointerEvent) => void, onEnd: () => void) {
	const move = (event: PointerEvent) => onMove(event);
	const up = () => {
		window.removeEventListener('pointermove', move);
		window.removeEventListener('pointerup', up);
		onEnd();
	};
	window.addEventListener('pointermove', move);
	window.addEventListener('pointerup', up, { once: true });
	return () => {
		window.removeEventListener('pointermove', move);
		window.removeEventListener('pointerup', up);
		onEnd();
	};
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
