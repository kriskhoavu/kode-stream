import { useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { SafeSessionRecord } from '../../lib/types';
import { formatSessionDate, isSessionLive, SessionConfirmDialog } from '../sessions/sessionDisplay';

/** Session node Terminal tab: the durable session summary and Stop; the live terminal itself stays in the Canvas node, never duplicated here. */
export function SessionTerminalSummary({ record, onReload }: { record: SafeSessionRecord; onReload: () => Promise<unknown> | void }) {
	const [confirming, setConfirming] = useState(false);
	const [stopping, setStopping] = useState(false);
	const [error, setError] = useState('');
	const generationRef = useRef(0);
	useEffect(() => {
		const generation = ++generationRef.current;
		return () => { if (generationRef.current === generation) generationRef.current += 1; };
	}, [record.id]);
	const live = isSessionLive(record);
	const stop = async () => {
		const generation = generationRef.current;
		const isCurrent = () => generationRef.current === generation;
		setStopping(true);
		setError('');
		try {
			await api.cancelEmbeddedAISession(record.id);
			await Promise.resolve(onReload());
			if (!isCurrent()) return;
			setConfirming(false);
		} catch (caught) {
			if (isCurrent()) setError(caught instanceof Error ? caught.message : 'The session could not be stopped.');
		} finally {
			if (isCurrent()) setStopping(false);
		}
	};
	return <div className="canvas-item-tab-content canvas-session-summary">
		<dl>
			<dt>Provider</dt><dd>{record.provider}</dd>
			<dt>Branch</dt><dd>{record.requestedBranch}</dd>
			<dt>State</dt><dd className={`canvas-session-state state-${record.state}`}>{record.state}</dd>
			<dt>Started</dt><dd>{formatSessionDate(record.startedAt)}</dd>
			{record.exitCode !== undefined && <><dt>Exit code</dt><dd>{record.exitCode}</dd></>}
		</dl>
		{live && <button className="danger-confirm" type="button" onClick={() => setConfirming(true)}>{`Stop ${record.provider} session`}</button>}
		{error && !confirming && <p className="error" role="alert">{error}</p>}
		{confirming && <SessionConfirmDialog title="Stop session?" message={`Stop the ${record.provider} session on ${record.requestedBranch}? Its durable session metadata will remain available.`} confirmLabel="Stop session" busyLabel="Stopping…" busy={stopping} error={error} onCancel={() => setConfirming(false)} onConfirm={() => void stop()} />}
	</div>;
}
