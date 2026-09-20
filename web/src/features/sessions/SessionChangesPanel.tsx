import { useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { SafeSessionRecord, SessionChanges, SessionFile, SessionFileStatus } from '../../lib/types';
import { isSessionLive } from './sessionDisplay';
import { useSessionChanges } from './useSessionChanges';

const STATUS_ORDER: SessionFileStatus[] = ['A', 'M', 'D', 'R', 'C', '?'];
const STATUS_LABEL: Record<SessionFileStatus, string> = { A: 'Added', M: 'Modified', D: 'Deleted', R: 'Renamed', C: 'Copied', '?': 'Untracked' };

function pluralize(count: number, word: string): string {
	return `${count} ${word}${count === 1 ? '' : 's'}`;
}

function headerLine(changes: SessionChanges): string {
	return `+${changes.additions ?? 0} −${changes.deletions ?? 0} · ${pluralize((changes.files ?? []).length, 'file')} · HEAD moved ${pluralize(changes.headMoved ?? 0, 'commit')}`;
}

function availabilityMessage(changes: SessionChanges): string {
	if (changes.availability === 'unsupported') return "Changes are not available for this workspace's content provider.";
	switch (changes.reason) {
		case 'workspace_missing': return 'The workspace checkout is missing, so changes cannot be computed.';
		case 'launch_commit_unknown': return "The session's launch commit is unknown, so changes cannot be computed.";
		case 'launch_commit_missing': return "The session's launch commit is no longer in the checkout's history, so changes cannot be computed.";
		default: return 'Changes are unavailable for this session.';
	}
}

function groupFiles(files: SessionFile[]) {
	return STATUS_ORDER.map((status) => ({ status, label: STATUS_LABEL[status], files: files.filter((file) => file.status === status) })).filter((group) => group.files.length > 0);
}

/**
 * The session node's Changes tab: header stats, availability/reason text
 * when changes cannot be computed, a truncation note, and a file list
 * grouped by status whose selection loads a unified diff.
 */
export function SessionChangesPanel({ session }: { session: SafeSessionRecord }) {
	const live = isSessionLive(session);
	const { changes, loading, error } = useSessionChanges(session.id, live);
	const [selectedPath, setSelectedPath] = useState<string>();
	const [diff, setDiff] = useState<{ path: string; diff: string; truncated: boolean }>();
	const [diffLoading, setDiffLoading] = useState(false);
	const [diffError, setDiffError] = useState('');
	const diffTokenRef = useRef(0);
	const diffAbortRef = useRef<AbortController | null>(null);

	useEffect(() => {
		diffAbortRef.current?.abort();
		diffTokenRef.current += 1;
		setSelectedPath(undefined);
		setDiff(undefined);
		setDiffError('');
	}, [session.id]);
	useEffect(() => () => diffAbortRef.current?.abort(), []);

	const openFile = (path: string) => {
		setSelectedPath(path);
		setDiff(undefined);
		setDiffError('');
		setDiffLoading(true);
		diffAbortRef.current?.abort();
		const controller = new AbortController();
		diffAbortRef.current = controller;
		const token = ++diffTokenRef.current;
		api.sessionFileDiff(session.id, path, controller.signal).then((result) => {
			if (controller.signal.aborted || diffTokenRef.current !== token) return;
			setDiff(result);
		}).catch((caught) => {
			if (controller.signal.aborted || diffTokenRef.current !== token) return;
			setDiffError(caught instanceof Error ? caught.message : 'The diff could not be loaded.');
		}).finally(() => {
			if (controller.signal.aborted || diffTokenRef.current !== token) return;
			setDiffLoading(false);
		});
	};

	if (loading && !changes) return <p role="status">Loading changes…</p>;
	if (!changes) {
		if (error) return <p className="error" role="alert">{error}</p>;
		return null;
	}

	if (changes.availability !== 'available') {
		return <div className="session-changes-panel session-changes-unavailable">
			{error && <p className="error" role="alert">{error}</p>}
			<p>{availabilityMessage(changes)}</p>
		</div>;
	}

	const groups = groupFiles(changes.files ?? []);

	return <div className="session-changes-panel">
		{error && <p className="error" role="alert">{error}</p>}
		<p className="session-changes-header">{headerLine(changes)}</p>
		<p className="session-changes-note">Changes since launch include work by any session in this checkout.</p>
		{changes.truncated && <p className="session-changes-truncated">Only the first files are shown; more files changed than are listed here.</p>}
		<div className="session-changes-body">
			<ul className="session-changes-file-list" aria-label="Changed files">
				{groups.map((group) => <li key={group.status}><h3>{group.label}</h3><ul>{group.files.map((file) => <li key={file.path}><button type="button" aria-pressed={selectedPath === file.path} className={selectedPath === file.path ? 'active' : ''} onClick={() => openFile(file.path)}>{file.path}</button></li>)}</ul></li>)}
			</ul>
			<div className="session-changes-diff">
				{!selectedPath && <p className="session-changes-note">Select a file to view its diff.</p>}
				{selectedPath && diffLoading && <p role="status">Loading diff…</p>}
				{selectedPath && diffError && <p className="error" role="alert">{diffError}</p>}
				{selectedPath && diff && <>
					{diff.truncated && <p className="session-changes-truncated">This diff is truncated.</p>}
					<pre>{diff.diff}</pre>
				</>}
			</div>
		</div>
	</div>;
}
