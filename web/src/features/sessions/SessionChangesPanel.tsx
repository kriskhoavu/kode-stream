import { useEffect, useState } from 'react';
import type { SessionChanges, SessionFile } from '../../lib/types';
import { api } from '../../shared/api';

const availabilityMessages: Record<SessionChanges['availability'], string> = {
	available: '',
	unavailable: 'Changes are not available for this session.',
	unsupported: 'Changes are not supported for this workspace.'
};

const reasonMessages: Record<NonNullable<SessionChanges['reason']>, string> = {
	launch_commit_unknown: 'The session did not record a launch commit.',
	launch_commit_missing: 'The launch commit no longer exists in this checkout.',
	workspace_missing: 'The workspace checkout is missing.',
	content_provider_unsupported: 'This workspace does not use a local checkout.'
};

/**
 * The changes panel for one session: what its checkout has done since the
 * session's launch commit. This duplicates the Workbench's own Changes tab
 * built in Phase F2 (both render a SessionChanges value); the two are meant
 * to be unified onto one shared component in Phase I1 once F2 lands.
 */
export function SessionChangesPanel({ sessionId, changes }: { sessionId: string; changes: SessionChanges }) {
	const [openPath, setOpenPath] = useState<string>();
	const [diff, setDiff] = useState('');
	const [diffTruncated, setDiffTruncated] = useState(false);
	const [diffError, setDiffError] = useState('');
	const [diffLoading, setDiffLoading] = useState(false);

	useEffect(() => {
		setOpenPath(undefined);
		setDiff('');
		setDiffError('');
	}, [sessionId]);

	useEffect(() => {
		if (!openPath) return;
		let cancelled = false;
		setDiffLoading(true);
		setDiffError('');
		setDiffTruncated(false);
		api.sessionFileDiff(sessionId, openPath).then((result) => {
			if (cancelled) return;
			setDiff(result.diff);
			setDiffTruncated(result.truncated);
		}).catch((caught: unknown) => {
			if (cancelled) return;
			setDiff('');
			setDiffTruncated(false);
			setDiffError(caught instanceof Error ? caught.message : 'Could not load the diff.');
		}).finally(() => {
			if (!cancelled) setDiffLoading(false);
		});
		return () => { cancelled = true; };
	}, [sessionId, openPath]);

	if (changes.availability !== 'available') {
		return (
			<div className="session-changes-panel">
				<p className="session-changes-note">{availabilityMessages[changes.availability]} {changes.reason ? reasonMessages[changes.reason] : ''}</p>
			</div>
		);
	}

	return (
		<div className="session-changes-panel">
			<p className="session-changes-summary">
				{changes.files.length} file{changes.files.length === 1 ? '' : 's'} changed since <code>{changes.launchCommit.slice(0, 12)}</code>
				{changes.headMoved > 0 ? `, head has moved ${changes.headMoved} commit${changes.headMoved === 1 ? '' : 's'}` : ''}
				{' · '}+{changes.additions} / -{changes.deletions}
			</p>
			{changes.truncated && <p className="session-changes-note">The file list was truncated.</p>}
			{changes.files.length === 0 && <p className="session-changes-note">No changes since the session launched.</p>}
			<ul className="session-changes-files">
				{changes.files.map((file) => (
					<li key={file.path}>
						<button type="button" className={openPath === file.path ? 'session-changes-file active' : 'session-changes-file'} onClick={() => setOpenPath(file.path === openPath ? undefined : file.path)} aria-expanded={openPath === file.path}>
							<span className={`session-file-status session-file-status-${file.status}`}>{file.status}</span>
							<span>{fileLabel(file)}</span>
						</button>
						{openPath === file.path && (
							<div className="session-changes-diff">
								{diffLoading && <p className="session-changes-note">Loading diff…</p>}
								{diffError && <p className="session-changes-note" role="alert">{diffError}</p>}
								{!diffLoading && !diffError && <pre>{diff || 'No textual diff available.'}</pre>}
								{diffTruncated && <p className="session-changes-note">The diff was truncated.</p>}
							</div>
						)}
					</li>
				))}
			</ul>
		</div>
	);
}

function fileLabel(file: SessionFile): string {
	return file.oldPath ? `${file.oldPath} → ${file.path}` : file.path;
}
