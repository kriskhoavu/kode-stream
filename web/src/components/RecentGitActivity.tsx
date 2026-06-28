import type { GitActivityEntry } from '../lib/types';

export function RecentGitActivity({ entries, loading, emptyLabel = 'No recent commits found.', pathLabel = '' }: {
  entries: GitActivityEntry[];
  loading: boolean;
  emptyLabel?: string;
  pathLabel?: string;
}) {
  if (loading) return <div className="recent-activity-empty">Loading activity...</div>;
  if (entries.length === 0) return <div className="recent-activity-empty">{emptyLabel}</div>;
  const grouped = groupByDate(entries);
  return <div className="recent-activity-list">
    {pathLabel && <p className="recent-activity-path">Tracking {pathLabel}</p>}
    {grouped.map((group) => <section key={group.date} className="recent-activity-day">
      <h5 className="recent-activity-date">{group.date}</h5>
      {group.entries.map((entry) => {
        const primaryPath = entry.paths[0];
        const moreCount = Math.max(entry.paths.length - 1, 0);
        return <article key={`${entry.commit}-${primaryPath?.path ?? 'commit'}`} className="recent-activity-entry">
          <p>{primaryPath ? `${primaryPath.status}: ${primaryPath.path}` : entry.message || 'Commit update'}</p>
          {moreCount > 0 && <small>+{moreCount} more changed path{moreCount > 1 ? 's' : ''}</small>}
          <small>{entry.author || 'Unknown author'} | {shortSha(entry.commit)}{entry.message ? ` | ${entry.message}` : ''}</small>
        </article>;
      })}
    </section>)}
  </div>;
}

function groupByDate(entries: GitActivityEntry[]): Array<{ date: string; entries: GitActivityEntry[] }> {
  const groups = new Map<string, GitActivityEntry[]>();
  for (const entry of entries) {
    const date = formatActivityDate(entry.committedAt);
    const bucket = groups.get(date);
    if (bucket) bucket.push(entry);
    else groups.set(date, [entry]);
  }
  return Array.from(groups.entries()).map(([date, dayEntries]) => ({ date, entries: dayEntries }));
}

function formatActivityDate(input: string): string {
  const value = new Date(input);
  if (Number.isNaN(value.getTime())) return 'Unknown date';
  return value.toISOString().slice(0, 10);
}

function shortSha(commit: string): string {
  return commit ? commit.slice(0, 7) : 'unknown';
}
