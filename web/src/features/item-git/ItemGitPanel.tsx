import { useState } from 'react';
import type { GitChange } from '../../lib/types';
import { RecentGitActivity } from '../../components/RecentGitActivity';
import { readTogglePreference, writePreference } from '../../shared/preferences/store';
import type { ItemGit } from './useItemGit';

export function ItemGitPanel({ git, className, activityOpenPreference, branchPlaceholder, onOpenPath, unavailable }: {
	git: ItemGit;
	className: string;
	activityOpenPreference: string;
	branchPlaceholder: string;
	onOpenPath?: (change: GitChange) => void;
	unavailable: { className: string; hint: string };
}) {
	const [activityOpen, setActivityOpen] = useState(() => readTogglePreference(activityOpenPreference));
	const { status, busy } = git;
	if (!status) {
		return (
			<div className={unavailable.className}>
				<strong>Git status unavailable</strong>
				<span>{unavailable.hint}</span>
			</div>
		);
	}
	return (
		<section className={className}>
			<div className="git-summary">
				<span>{status.branch}</span>
				<span>{status.ahead} ahead</span>
				<span>{status.behind} behind</span>
			</div>
			<div className="workspace-actions">
				<button className="secondary" type="button" disabled={Boolean(busy)} onClick={() => void git.run('fetch')}>{busy === 'fetch' ? 'Fetching...' : 'Fetch'}</button>
				<button className="secondary" type="button" disabled={Boolean(busy)} onClick={() => void git.run('pull')}>{busy === 'pull' ? 'Pulling...' : 'Pull'}</button>
				<button className="secondary" type="button" disabled={Boolean(busy)} onClick={() => void git.run('push')}>{busy === 'push' ? 'Pushing...' : 'Push'}</button>
			</div>
			<div className="git-changes">
				{status.changes.length === 0 && <span>No local changes</span>}
				{status.changes.map((change) => {
					const key = `${change.status}-${change.path}`;
					const checkbox = <input type="checkbox" aria-label={`Select ${change.path} for commit`} checked={git.selectedPaths.includes(change.path)} onChange={() => git.togglePath(change.path)} />;
					return onOpenPath ? (
						<div className="git-change-row" key={key}>
							{checkbox}
							<span>{change.status}</span>
							<button className="git-change-path" type="button" title={change.path} onClick={() => onOpenPath(change)}>{change.path}</button>
						</div>
					) : (
						<label key={key}>
							{checkbox}
							<span>{change.status}</span>
							<strong>{change.path}</strong>
						</label>
					);
				})}
			</div>
			<textarea className="commit-message" value={git.message} onChange={(event) => git.setMessage(event.target.value)} placeholder="Commit message" />
			<button className="primary" type="button" disabled={Boolean(busy) || git.selectedPaths.length === 0 || !git.message.trim()} onClick={() => void git.commit()}>
				{busy === 'commit' ? 'Committing...' : 'Commit Selected'}
			</button>
			<div className="branch-create-row">
				<input value={git.branchName} onChange={(event) => git.setBranchName(event.target.value)} placeholder={branchPlaceholder} />
				<button className="secondary" type="button" disabled={Boolean(busy) || !git.branchName.trim()} onClick={() => void git.createBranch()}>
					{busy === 'branch' ? 'Creating...' : 'Create Branch'}
				</button>
			</div>
			<details className="recent-activity-panel" open={activityOpen} onToggle={(event) => {
				const open = event.currentTarget.open;
				setActivityOpen(open);
				writePreference(activityOpenPreference, open ? '1' : '0');
			}}>
				<summary>
					<span>Recent Activity</span>
					<small>{git.activity.length} events</small>
				</summary>
				<RecentGitActivity entries={git.activity} loading={git.activityLoading} emptyLabel="No activity found for this item." pathLabel={git.activityPath || 'workspace'} />
			</details>
		</section>
	);
}
