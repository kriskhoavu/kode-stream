import type { PointerEvent as ReactPointerEvent } from 'react';
import { useEffect, useRef, useState } from 'react';
import { Columns2, Copy, Minus, PanelRight, Square, SquareTerminal, X } from 'lucide-react';
import type { Terminal } from '@xterm/xterm';
import type { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { api } from '../../lib/api';
import type { EmbeddedAISessionResult, EmbeddedAISessionState } from '../../lib/types';

type ServerFrame = { type: 'output' | 'state' | 'warning' | 'exit'; data?: string; encoding?: string; state?: EmbeddedAISessionState; exitCode?: number; message?: string };

export function EmbeddedTerminal({ initial, visible, mode, title, subtitle, onStartMove, onToggleDockMode, onToggleMinimize, onToggleMaximize, onClose, onStateChange }: { initial: EmbeddedAISessionResult; visible: boolean; mode: 'floating' | 'side_panel' | 'maximized'; title: string; subtitle: string; onStartMove: (x: number, y: number) => void; onToggleDockMode: () => void; onToggleMinimize: () => void; onToggleMaximize: () => void; onClose: () => void; onStateChange?: (state: EmbeddedAISessionState, exitCode?: number) => void }) {
	const [state, setState] = useState<EmbeddedAISessionState>(initial.session.state);
	const [connection, setConnection] = useState<'connecting' | 'connected' | 'reconnecting' | 'closed'>('connecting');
	const [exitCode, setExitCode] = useState<number | undefined>(initial.session.exitCode);
	const [message, setMessage] = useState('');
	const hostRef = useRef<HTMLDivElement | null>(null);
	const controlsRef = useRef<HTMLButtonElement | null>(null);
	const terminalRef = useRef<Terminal | null>(null);
	const fitRef = useRef<FitAddon | null>(null);
	const socketRef = useRef<WebSocket | null>(null);
	const active = state === 'starting' || state === 'running';

	useEffect(() => {
		if (!active) return;
		const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
		window.addEventListener('beforeunload', warn);
		return () => window.removeEventListener('beforeunload', warn);
	}, [active]);

	useEffect(() => {
		if (!hostRef.current) return;
		let disposed = false; let retry: number | undefined; let terminal: Terminal | undefined; let observer: ResizeObserver | undefined; let dataDisposable: { dispose(): void } | undefined; let resizeDisposable: { dispose(): void } | undefined; const deadline = Date.now() + 15_000;
		const send = (value: unknown) => { if (socketRef.current?.readyState === WebSocket.OPEN) socketRef.current.send(JSON.stringify(value)); };
		const connect = (activeTerminal: Terminal) => {
			if (disposed) return;
			const scheme = location.protocol === 'https:' ? 'wss:' : 'ws:';
			const socket = new WebSocket(`${scheme}//${location.host}/api/ai/sessions/${encodeURIComponent(initial.session.id)}/channel?token=${encodeURIComponent(initial.grant.token)}`);
			socketRef.current = socket;
			socket.onopen = () => { setConnection('connected'); send({ type: 'resize', columns: activeTerminal.cols, rows: activeTerminal.rows }); };
				socket.onmessage = (event) => {
				const frame = JSON.parse(String(event.data)) as ServerFrame;
				if (frame.type === 'output' && frame.data) activeTerminal.write(frame.encoding === 'base64' ? Uint8Array.from(atob(frame.data), (character) => character.charCodeAt(0)) : frame.data);
				if (frame.type === 'state' && frame.state) { setState(frame.state); setExitCode(frame.exitCode); onStateChange?.(frame.state, frame.exitCode); }
				if (frame.type === 'warning') setMessage(frame.message ?? 'Terminal warning');
			};
			socket.onclose = () => { if (disposed) return; if (Date.now() < deadline && (state === 'starting' || state === 'running')) { setConnection('reconnecting'); retry = window.setTimeout(() => connect(activeTerminal), 500); } else { setConnection('closed'); setMessage('The terminal connection could not be restored.'); } };
		};
		void Promise.all([import('@xterm/xterm'), import('@xterm/addon-fit')]).then(([terminalModule, fitModule]) => {
			if (disposed || !hostRef.current) return;
			terminal = new terminalModule.Terminal({ convertEol: true, cursorBlink: true, fontSize: 12, lineHeight: 1.15, scrollback: 5000, theme: { background: '#090d18' } });
			const fit = new fitModule.FitAddon(); fitRef.current = fit; terminal.loadAddon(fit); terminal.open(hostRef.current); fit.fit(); terminal.focus(); terminalRef.current = terminal;
			dataDisposable = terminal.onData((data) => send({ type: 'input', data: btoa(unescape(encodeURIComponent(data))) }));
			resizeDisposable = terminal.onResize(({ cols, rows }) => send({ type: 'resize', columns: cols, rows }));
			observer = new ResizeObserver(() => fit.fit()); observer.observe(hostRef.current); connect(terminal);
		}).catch(() => setMessage('The terminal emulator could not be loaded.'));
		return () => { disposed = true; if (retry) window.clearTimeout(retry); observer?.disconnect(); dataDisposable?.dispose(); resizeDisposable?.dispose(); socketRef.current?.close(); terminal?.dispose(); terminalRef.current = null; fitRef.current = null; };
	}, [initial]);

	useEffect(() => {
		const escape = (event: KeyboardEvent) => { if (event.key === 'Escape' && event.ctrlKey && event.shiftKey) { event.preventDefault(); controlsRef.current?.focus(); } };
		window.addEventListener('keydown', escape); return () => window.removeEventListener('keydown', escape);
	}, []);

	useEffect(() => {
		if (!visible) return;
		const frame = requestAnimationFrame(() => { fitRef.current?.fit(); terminalRef.current?.focus(); });
		return () => cancelAnimationFrame(frame);
	}, [mode, visible]);

	const close = () => { if (active && !window.confirm('Cancel the running AI session and close the terminal?')) return; if (active) void api.cancelEmbeddedAISession(initial.session.id); onClose(); };
	const startMove = (event: ReactPointerEvent<HTMLElement>) => {
		const target = event.target as HTMLElement;
		if (target.closest('button')) return;
		if (mode !== 'floating') return;
		event.preventDefault();
		onStartMove(event.clientX, event.clientY);
	};

	return <section className={`embedded-terminal-panel${visible ? ' active' : ''}`} aria-hidden={!visible} role={visible ? 'dialog' : undefined} aria-modal={visible ? 'true' : undefined} aria-labelledby={visible ? `embedded-terminal-title-${initial.session.id}` : undefined}>
		<header onPointerDown={startMove}><div className="embedded-terminal-heading"><span className="embedded-terminal-heading-icon"><SquareTerminal size={19} /></span><div><h2 id={`embedded-terminal-title-${initial.session.id}`}>{title}</h2><p>{subtitle} · <span role="status" aria-live="polite">{stateLabel(state, connection, exitCode)}</span></p></div></div><div className="embedded-terminal-window-actions"><button ref={controlsRef} className="icon-button" type="button" title="Minimize" aria-label="Minimize embedded terminal" onClick={onToggleMinimize}><Minus size={18} strokeWidth={1.8} /></button><button className="icon-button" type="button" title={mode === 'side_panel' ? 'Float window' : 'Dock to right side'} aria-label={mode === 'side_panel' ? 'Float embedded terminal window' : 'Dock embedded terminal to right side'} onClick={onToggleDockMode}>{mode === 'side_panel' ? <Columns2 size={16} strokeWidth={1.8} /> : <PanelRight size={16} strokeWidth={1.8} />}</button><button className="icon-button" type="button" title={mode === 'maximized' ? 'Restore' : 'Maximize'} aria-label={mode === 'maximized' ? 'Restore embedded terminal size' : 'Maximize embedded terminal'} onClick={onToggleMaximize}>{mode === 'maximized' ? <Copy size={16} strokeWidth={1.8} /> : <Square size={15} strokeWidth={1.8} />}</button><button className="icon-button embedded-terminal-close" type="button" title="Close" aria-label="Close embedded terminal" onClick={close}><X size={18} strokeWidth={1.8} /></button></div></header>
		<div ref={hostRef} className="embedded-terminal-canvas" aria-label="AI terminal output" />
		{message && <p className="error" role="alert">{message}</p>}
	</section>;
}

function stateLabel(state: EmbeddedAISessionState, connection: string, exitCode?: number) {
	if (connection === 'reconnecting' && (state === 'starting' || state === 'running')) return 'Reconnecting to session…';
	if (state === 'exited') return `Session exited${exitCode === undefined ? '' : ` with code ${exitCode}`}.`;
	return ({ starting: 'Starting session…', running: 'Session running', cancelled: 'Session cancelled', failed: 'Session failed' } as Record<EmbeddedAISessionState, string>)[state];
}
