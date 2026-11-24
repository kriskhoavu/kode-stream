import { useState } from 'react';
import { ArrowLeft } from 'lucide-react';
import type { ItemDetail } from '../../lib/types';
import type { HarnessRunTab } from '../../app/router';
import { RunView } from '../harness/RunView';
import { RunDrawer } from '../harness/RunDrawer';
import { RunPagePath, RunWorkbench } from '../harness/RunWorkbench';
import { queueOrder, type CardWork } from '../workstream/harnessJoin';
import type { HarnessController } from '../harness/useHarness';

export function ItemRunView({ plan, work, harness, onBack }: {
	plan: ItemDetail;
	work: CardWork & { run: NonNullable<CardWork['run']> };
	harness: HarnessController;
	onBack: () => void;
}) {
	const [runTab, setRunTab] = useState<{ tab: HarnessRunTab; agentRunId?: string } | undefined>();
	const run = work.run;
	return (
		<section className="workspace-page item-run-page hx" aria-label="Harness run, full width">
			<header className="workspace-header">
				<div className="workspace-header-main">
					<button className="ghost" type="button" onClick={onBack}><ArrowLeft size={16} /> Back to the card</button>
				</div>
				<div className="workspace-header-title">
					<h1>{plan.title}</h1>
					<RunPagePath run={harness.state.runs[run.id] ?? run} lead={<><span className="workspace-item-path-segment">{plan.scope}</span><span className="workspace-item-path-separator">/</span><span className="workspace-item-path-segment">{plan.identifier}</span></>} />
				</div>
				<div className="workspace-header-actions" />
			</header>
			<RunView state={harness.state} runId={run.id} onSelectRun={() => undefined} onRefresh={() => void harness.refresh()} loadRun={harness.loadRun} onApprove={harness.approve} onReject={harness.reject} onCancel={harness.cancelRun} onOpenTab={(tab, agentRunId) => setRunTab({ tab, agentRunId })} workbench={(detail, request) => <RunWorkbench client={harness.client} detail={detail} request={request} />} />
			{runTab && <RunDrawer state={harness.state} client={harness.client} taskId={work.task.id} tab={runTab.tab} agentRunId={runTab.agentRunId} queue={queueOrder(harness.state, work.task.workspaceId)} onTabChange={(tab) => setRunTab({ tab })} onClose={() => setRunTab(undefined)} onCancel={harness.cancelRun} loadRun={harness.loadRun} onApprove={harness.approve} onReject={harness.reject} onPin={harness.pin} onMove={() => undefined} onSetQuestions={harness.setQuestions} askDefault={harness.state.status?.askQuestions ?? true} />}
		</section>
	);
}
