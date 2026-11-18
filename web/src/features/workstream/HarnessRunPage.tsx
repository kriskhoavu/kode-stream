import { useState } from 'react';
import { ArrowLeft } from 'lucide-react';
import type { HarnessRunTab as HarnessRunTabId } from '../../app/router';
import type { HarnessTask } from '../../lib/types';
import { ResetTaskButton, canResetTask } from '../harness/ResetTaskButton';
import { RunDrawer } from '../harness/RunDrawer';
import { RunView } from '../harness/RunView';
import { queueOrder } from './harnessJoin';
import { useBoardHarness } from './useBoardHarness';
import { useTaskActions } from './useTaskActions';

/**
 * A harness run shown whole, full width, for a task no plan card holds yet:
 * a Jira ticket's run has no item page to open it on, so it opens here.
 */
export function HarnessRunPage({ runId, task, onBack }: { runId: string; task: HarnessTask; onBack: () => void }) {
	const harness = useBoardHarness();
	const actions = useTaskActions(harness);
	const [runTab, setRunTab] = useState<{ tab: HarnessRunTabId; agentRunId?: string } | undefined>();
	return (
		<section className="workspace-page item-run-page hx" aria-label="Harness run, full width">
			<header className="workspace-header">
				<div className="workspace-header-main">
					<button className="ghost" type="button" onClick={onBack}><ArrowLeft size={16} /> Back to the board</button>
					{canResetTask(task) && <ResetTaskButton task={task} onReset={(reset) => harness.resetTask(reset).then(onBack)} />}
				</div>
				<div className="workspace-header-title">
					<h1>{task.title}</h1>
					<div className="workspace-item-path" aria-label="Item location"><span className="workspace-item-path-segment">Jira ticket</span><span className="workspace-item-path-separator">/</span><span className="workspace-item-path-segment">{task.source.key}</span></div>
				</div>
			</header>
			<RunView state={harness.state} runId={runId} onSelectRun={() => undefined} onRefresh={() => void harness.refresh()} loadRun={harness.loadRun} onApprove={harness.approve} onReject={harness.reject} onCancel={harness.cancelRun} onOpenTab={(tab, agentRunId) => setRunTab({ tab, agentRunId })} />
			{runTab && <RunDrawer state={harness.state} client={harness.client} taskId={task.id} tab={runTab.tab} agentRunId={runTab.agentRunId} queue={queueOrder(harness.state, task.workspaceId)} onTabChange={(tab) => setRunTab({ tab })} onClose={() => setRunTab(undefined)} onCancel={harness.cancelRun} loadRun={harness.loadRun} onApprove={harness.approve} onReject={harness.reject} onPin={actions.pin} onMove={actions.move} onStart={actions.start} onReset={(reset) => harness.resetTask(reset).then(onBack)} onSetQuestions={harness.setQuestions} askDefault={harness.state.status?.askQuestions ?? true} />}
		</section>
	);
}
