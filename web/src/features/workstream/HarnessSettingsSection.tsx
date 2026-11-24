import { HarnessSettings, type HarnessSettingsView } from '../harness/HarnessSettings';
import { useBoardHarness } from './useBoardHarness';

export function HarnessSettingsSection({ workspaceId, part, view }: { workspaceId?: string; part?: 'workspace' | 'sources'; view?: HarnessSettingsView }) {
	const harness = useBoardHarness();
	return (
		<div className="hx harness-settings-section">
			<HarnessSettings client={harness.client} status={harness.state.status} workspaceId={workspaceId} part={part} view={view} onPause={harness.pauseRole} onResume={harness.resumeRole} onSaved={() => void harness.refresh()} />
		</div>
	);
}
