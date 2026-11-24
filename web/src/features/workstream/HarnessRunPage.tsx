import { useState } from 'react';
import { ArrowLeft } from 'lucide-react';
import type { HarnessRunTab as HarnessRunTabId } from '../../app/router';
import type { HarnessTask } from '../../lib/types';
import { RemoveTaskButton } from '../harness/RemoveTaskButton';
import { ResetTaskButton, canResetTask } from '../harness/ResetTaskButton';
import { RunDrawer } from '../harness/RunDrawer';
import { RunView } from '../harness/RunView';
import { RunPagePath, RunWorkbench } from '../harness/RunWorkbench';
import { sourceLabels } from '../harness/vocabulary';
import { queueOrder } from './harnessJoin';
import { useBoardHarness } from './useBoardHarness';
import { useTaskActions } from './useTaskActions';

export function HarnessRunPage({ runId, task, onBack }: { runId: string; task: HarnessTask; onBack: () => void }) {
	const harness = useBoardHarness();
	const actions = useTaskActions(harness);
	const [runTab, setRunTab] = useState<{ tab: HarnessRunTabId; agentRunId?: string } | undefined>();
	return (
		<section className="workspace-page item-run-page hx" aria-label="Harness run, full width">
			<header className="workspace-header">
				<div className="workspace-header-main">
					<button className="ghost" type="button" onClick={onBack}><ArrowLeft size={16} /> Back to the board</button>
</div>
				<div className="workspace-header-title">
					<h1>{task.title}</h1>
					<RunPagePath run={harness.state.runs[runId]} lead={task.source.url
						? <a className="workspace-item-path-segment hx-link" href={task.source.url} target="_blank" rel="noreferrer" title={task.source.key}>{sourceLabels[task.source.kind]} ticket ↗</a>
						: <span className="workspace-item-path-segment">{sourceLabels[task.source.kind]} ticket {task.source.key}</span>} />
				</div>
				<div className="workspace-header-actions">
					{canResetTask(task) && <ResetTaskButton task={task} onReset={(reset) => harness.resetTask(reset).then(onBack)} />}
					<RemoveTaskButton task={task} onRemove={(removed) => harness.removeTask(removed).then(onBack)} />
				</div>
			</header>
			<RunView state={harness.state} runId={runId} onSelectRun={() => undefined} onRefresh={() => void harness.refresh()} loadRun={harness.loadRun} onApprove={harness.approve} onReject={harness.reject} onCancel={harness.cancelRun} onOpenTab={(tab, agentRunId) => setRunTab({ tab, agentRunId })} workbench={(detail, request) => <RunWorkbench client={harness.client} detail={detail} request={request} />} />
			{runTab && <RunDrawer state={harness.state} client={harness.client} taskId={task.id} tab={runTab.tab} agentRunId={runTab.agentRunId} queue={queueOrder(harness.state, task.workspaceId)} onTabChange={(tab) => setRunTab({ tab })} onClose={() => setRunTab(undefined)} onCancel={harness.cancelRun} loadRun={harness.loadRun} onApprove={harness.approve} onReject={harness.reject} onPin={actions.pin} onMove={actions.move} onStart={actions.start} onReset={(reset) => harness.resetTask(reset).then(onBack)} onRemove={(removed) => harness.removeTask(removed).then(onBack)} onSetQuestions={harness.setQuestions} askDefault={harness.state.status?.askQuestions ?? true} />}
		</section>
	);
}
