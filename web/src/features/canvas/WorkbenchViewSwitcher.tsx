import { ListChecks, Workflow } from 'lucide-react';
import type { WorkbenchView } from '../../app/router';

export function WorkbenchViewSwitcher({ view, sessionCount = 0, onChange }: {
	view: WorkbenchView;
	sessionCount?: number;
	onChange: (view: WorkbenchView) => void;
}) {
	return <div className="segmented-control workbench-view-switcher" role="group" aria-label="Workbench view">
		<button type="button" className={view === 'canvas' ? 'active' : ''} aria-pressed={view === 'canvas'} onClick={() => onChange('canvas')}><Workflow size={15} /> Canvas</button>
		<button type="button" className={view === 'sessions' ? 'active' : ''} aria-pressed={view === 'sessions'} onClick={() => onChange('sessions')}><ListChecks size={15} /> Sessions{sessionCount ? ` (${sessionCount})` : ''}</button>
	</div>;
}
