import type { HarnessRun, HarnessTask } from '../../lib/types';
import { hxStatuses } from './vocabulary';
import { runStatus } from './pipeline';

/** Choose which run the pipeline shows: running first, then the most recent. */
export function RunPicker({ runs, tasks, value, onChange }: { runs: HarnessRun[]; tasks: Record<string, HarnessTask>; value?: string; onChange: (runId: string) => void }) {
	const order = (run: HarnessRun) => (run.state === 'running' ? 0 : run.state === 'blocked' ? 1 : 2);
	const sorted = [...runs].sort((a, b) => order(a) - order(b) || Date.parse(b.startedAt) - Date.parse(a.startedAt));
	return (
		<label className="hx-run-picker">
			<span className="sr-only">Task</span>
			<select value={value ?? ''} onChange={(event) => onChange(event.target.value)}>
				{!value && <option value="">Choose a task</option>}
				{sorted.map((run) => {
					const task = tasks[run.taskId];
					return <option key={run.id} value={run.id}>{task ? `${task.source.key} · ${task.title}` : run.branch} · {hxStatuses[runStatus(run)].label}</option>;
				})}
			</select>
		</label>
	);
}
