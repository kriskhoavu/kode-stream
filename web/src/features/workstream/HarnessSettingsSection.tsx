import { HarnessSettings } from '../harness/HarnessSettings';
import { useBoardHarness } from './useBoardHarness';

/** The harness under Settings: who does what, where work comes from, how it runs, pause a role, and what it spends. */
export function HarnessSettingsSection() {
	const harness = useBoardHarness();
	return (
		<div className="hx harness-settings-section">
			<HarnessSettings client={harness.client} status={harness.state.status} onPause={harness.pauseRole} onResume={harness.resumeRole} onSaved={() => void harness.refresh()} />
		</div>
	);
}
