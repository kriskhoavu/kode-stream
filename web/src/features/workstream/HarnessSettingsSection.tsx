import { HarnessSettings } from '../harness/HarnessSettings';
import { useBoardHarness } from './useBoardHarness';

/** The harness's settings: its defaults, one workspace's own choices over them, or that workspace's sources. */
export function HarnessSettingsSection({ workspaceId, part }: { workspaceId?: string; part?: 'workspace' | 'sources' }) {
	const harness = useBoardHarness();
	return (
		<div className="hx harness-settings-section">
			<HarnessSettings client={harness.client} status={harness.state.status} workspaceId={workspaceId} part={part} onPause={harness.pauseRole} onResume={harness.resumeRole} onSaved={() => void harness.refresh()} />
		</div>
	);
}
