import { HarnessSettings } from '../harness/HarnessSettings';
import { useBoardHarness } from './useBoardHarness';

/** The harness under Settings: its defaults, or one workspace's own choices over them. */
export function HarnessSettingsSection({ workspaceId }: { workspaceId?: string }) {
	const harness = useBoardHarness();
	return (
		<div className="hx harness-settings-section">
			<HarnessSettings client={harness.client} status={harness.state.status} workspaceId={workspaceId} onPause={harness.pauseRole} onResume={harness.resumeRole} onSaved={() => void harness.refresh()} />
		</div>
	);
}
