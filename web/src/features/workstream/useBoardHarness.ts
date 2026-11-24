import { useEffect, useState } from 'react';
import type { HarnessClient } from '../harness/harnessClient';
import { apiHarnessClient } from '../harness/harnessClient';
import { loadHarnessClient, wantsHarnessFixture } from '../harness/loadHarnessClient';
import { useHarness } from '../harness/useHarness';
import type { HarnessController } from '../harness/useHarness';
import { useHarnessEvents } from '../harness/useHarnessEvents';

export function useBoardHarness(): HarnessController {
	const [client, setClient] = useState<HarnessClient>(() => apiHarnessClient);
	useEffect(() => {
		if (wantsHarnessFixture()) void loadHarnessClient().then(setClient);
	}, []);
	const harness = useHarness(client);
	const active = Boolean(harness.state.status?.enabled && harness.state.status.storageSupported);
	useHarnessEvents(harness, active);
	return harness;
}
