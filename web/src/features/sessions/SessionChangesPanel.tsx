import { useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { SafeSessionRecord, SessionChanges, SessionFile, SessionFileStatus } from '../../lib/types';
import { isSessionLive } from './sessionDisplay';
import { useSessionChanges } from './useSessionChanges';
import type { ChangesLoader } from './useSessionChanges';

export interface ChangesSource {
	id: string;
	live: boolean;
	loadChanges: ChangesLoader;
	loadDiff: (id: string, path: string, signal: AbortSignal) => Promise<{ diff: string; truncated: boolean }>;
	note: string;
}

export function sessionChangesSource(session: SafeSessionRecord): ChangesSource {
	return {
		id: session.id,
		live: isSessionLive(session),
		loadChanges: (id, signal) => api.sessionChanges(id, signal),
		loadDiff: (id, path, signal) => api.sessionFileDiff(id, path, signal),
		note: 'Changes since launch include work by any session in this checkout.'
	};
}

const STATUS_ORDER: SessionFileStatus[] = ['A', 'M', 'D', 'R', 'C', '?'];
const STATUS_LABEL: Record<SessionFileStatus, string> = { A: 'Added', M: 'Modified', D: 'Deleted', R: 'Renamed', C: 'Copied', '?': 'Untracked' };

function pluralize(count: number, word: string): string {
	return `${count} ${word}${count === 1 ? '' : 's'}`;
}

function headerLine(changes: SessionChanges): string {
	const where = changes.from ? (changes.branch ? ` · on ${changes.branch}` : '') : ` · HEAD moved ${pluralize(changes.headMoved ?? 0, 'commit')}`;
	return `+${changes.additions ?? 0} −${changes.deletions ?? 0} · ${pluralize((changes.files ?? []).length, 'file')}${where}`;
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

function splitPath(path: string): { name: string; folder: string } {
	const index = path.lastIndexOf('/');
	return index < 0 ? { name: path, folder: '' } : { name: path.slice(index + 1), folder: path.slice(0, index) };
}

function diffLineClass(line: string): string {
	if (line.startsWith('+++') || line.startsWith('---') || line.startsWith('diff ') || line.startsWith('index ')) return 'meta';
	if (line.startsWith('@@')) return 'hunk';
	if (line.startsWith('+')) return 'added';
	if (line.startsWith('-')) return 'removed';
	return '';
}

function groupFiles(files: SessionFile[]) {
	return STATUS_ORDER.map((status) => ({ status, label: STATUS_LABEL[status], files: files.filter((file) => file.status === status) })).filter((group) => group.files.length > 0);
}

export function SessionChangesPanel({ session, source: given }: { session?: SafeSessionRecord; source?: ChangesSource }) {
	const source = given ?? sessionChangesSource(session!);
	const { changes, loading, error } = useSessionChanges(source.id, source.live, source.loadChanges);
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
	}, [source.id]);
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
		source.loadDiff(source.id, path, controller.signal).then((result) => {
			if (controller.signal.aborted || diffTokenRef.current !== token) return;
			setDiff({ path, diff: result.diff, truncated: result.truncated });
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
		<p className="session-changes-note">{source.note}</p>
		{changes.truncated && <p className="session-changes-truncated">Only the first files are shown; more files changed than are listed here.</p>}
		<div className="session-changes-body">
			<ul className="session-changes-file-list" aria-label="Changed files">
				{groups.map((group) => <li key={group.status}><h3>{group.label} <span>{group.files.length}</span></h3><ul>{group.files.map((file) => {
					const { name, folder } = splitPath(file.path);
					return <li key={file.path}><button type="button" aria-label={file.path} title={file.oldPath ? `${file.oldPath} → ${file.path}` : file.path} aria-pressed={selectedPath === file.path} className={selectedPath === file.path ? 'active' : ''} onClick={() => openFile(file.path)}>
						<span className={`session-changes-status status-${file.status === '?' ? 'untracked' : file.status}`} aria-hidden="true">{file.status === '?' ? 'U' : file.status}</span>
						<span className="session-changes-path"><span className="session-changes-name">{name}</span>{folder && <span className="session-changes-folder">{folder}</span>}</span>
					</button></li>;
				})}</ul></li>)}
			</ul>
			<div className="session-changes-diff">
				{!selectedPath && <p className="session-changes-note session-changes-diff-empty">Select a file to view its diff.</p>}
				{selectedPath && diffLoading && <p role="status">Loading diff…</p>}
				{selectedPath && diffError && <p className="error" role="alert">{diffError}</p>}
				{selectedPath && diff && <>
					{diff.truncated && <p className="session-changes-truncated">This diff is truncated.</p>}
					<pre className="session-changes-diff-text">{diff.diff.split('\n').map((line, index) => <span key={index} className={diffLineClass(line)}>{line}{'\n'}</span>)}</pre>
				</>}
			</div>
		</div>
	</div>;
}
