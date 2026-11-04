import { Github, Ticket } from 'lucide-react';
import type { HarnessSourceKind } from '../../lib/types';
import { sourceLabels } from './vocabulary';

/** Where a task came from, as a word; the icon is decoration. */
export function SourceBadge({ kind, url }: { kind: HarnessSourceKind; url?: string }) {
	const Icon = kind === 'github' ? Github : Ticket;
	const body = <><Icon aria-hidden="true" size={11} /><span>{sourceLabels[kind]}</span></>;
	return url
		? <a className="hx-source" href={url} target="_blank" rel="noreferrer" title={`Open in ${sourceLabels[kind]}`}>{body}</a>
		: <span className="hx-source">{body}</span>;
}
