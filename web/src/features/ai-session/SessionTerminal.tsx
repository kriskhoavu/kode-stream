import { useEffect, useRef, useState } from 'react';
import { Square } from 'lucide-react';
import { api } from '../../shared/api';
import type { EmbeddedAISessionResult, EmbeddedAISessionState, SafeSessionRecord } from '../../lib/types';
import { EmbeddedTerminal } from './EmbeddedTerminal';
import { notifyEmbeddedSessionEnded } from './terminalSessions';

// Canvas nodes use this too, hence the `nodrag nopan nowheel` classes and the pointer guards.
export function SessionTerminal({ record, visible, onClose = () => {}, onReload, classPrefix = 'canvas-session-terminal', ariaLabel }: { record: SafeSessionRecord; visible: boolean; onClose?: () => void; onReload: () => Promise<unknown> | void; classPrefix?: string; ariaLabel?: string }) {
	const [result, setResult] = useState<EmbeddedAISessionResult>();
	const [attaching, setAttaching] = useState(false);
	const [error, setError] = useState('');
	const [retry, setRetry] = useState(0);
	const attempted = useRef(-1);
	const currentRecord = useRef(record);
	const mounted = useRef(true);
	currentRecord.current = record;
	const active = record.state === 'starting' || record.state === 'running';

	useEffect(() => {
		mounted.current = true;
		return () => { mounted.current = false; };
	}, []);

	// A new record attaches from scratch instead of keeping the old terminal state.
	useEffect(() => {
		setResult(undefined);
		setError('');
		attempted.current = -1;
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [record.id]);

	useEffect(() => {
		if (!visible || !record.live || result || attempted.current === retry) return;
		attempted.current = retry;
		let current = true;
		let settled = false;
		setAttaching(true);
		setError('');
		void Promise.all([api.embeddedAISession(record.id), api.embeddedAISessionGrant(record.id)]).then(([session, grant]) => {
			if (current) setResult({ session, grant, record: currentRecord.current });
		}).catch(() => { if (current) setError('The live process could not be reattached.'); }).finally(() => {
			settled = true;
			if (current) setAttaching(false);
		});
		return () => {
			current = false;
			if (!settled && attempted.current === retry) attempted.current = -1;
		};
	}, [record.id, record.live, result, retry, visible]);

	const cancel = async () => {
		if (!window.confirm('Cancel this terminal process? Its durable session metadata will remain available.')) return;
		setError('');
		try {
			notifyEmbeddedSessionEnded(record.id);
			const session = await api.cancelEmbeddedAISession(record.id);
			if (!mounted.current || currentRecord.current.id !== record.id) return;
			setResult((current) => current ? { ...current, session } : current);
			await Promise.resolve(onReload());
		} catch (caught) {
			if (mounted.current && currentRecord.current.id === record.id) setError(caught instanceof Error ? caught.message : 'Could not cancel the terminal process.');
		}
	};
	const stateChanged = (state: EmbeddedAISessionState, exitCode?: number) => {
		setResult((current) => current ? { ...current, session: { ...current.session, state, exitCode } } : current);
		if (state !== 'starting' && state !== 'running') void onReload();
	};

	return <section className={`${classPrefix} nodrag nopan nowheel`} hidden={!visible} aria-label={ariaLabel ?? `${record.provider} Canvas terminal`} onClick={(event) => event.stopPropagation()} onPointerDown={(event) => event.stopPropagation()}>
		{result && <EmbeddedTerminal initial={result} visible={visible} mode="side_panel" title={`${record.provider} terminal`} subtitle={record.requestedBranch} cancelOnClose={false} compact onStartMove={() => {}} onToggleDockMode={() => {}} onToggleMinimize={() => {}} onToggleMaximize={() => {}} onClose={onClose} onStateChange={stateChanged} />}
		{attaching && <p role="status">Connecting to terminal…</p>}
		{!record.live && <div className={`${classPrefix}-lifecycle`}><strong>Terminal session {record.state}</strong><span>Branch {record.requestedBranch}</span>{record.exitCode !== undefined && <span>Exit code {record.exitCode}</span>}{record.state === 'interrupted' && <p>The application restarted without this process. Reconnect is unavailable; launch a new session.</p>}</div>}
		{error && <div className={`${classPrefix}-error`} role="alert"><span>{error}</span>{record.live && <button type="button" onClick={() => { setError(''); setRetry((value) => value + 1); }}>Retry connection</button>}</div>}
		{active && <div className={`${classPrefix}-actions`}><button className="danger-confirm" type="button" onClick={() => void cancel()}><Square size={13} /> Cancel process</button></div>}
	</section>;
}
