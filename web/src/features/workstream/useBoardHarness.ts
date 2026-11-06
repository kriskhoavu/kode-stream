import { useEffect, useState } from 'react';
import type { HarnessClient } from '../harness/harnessClient';
import { apiHarnessClient } from '../harness/harnessClient';
import { loadHarnessClient, wantsHarnessFixture } from '../harness/loadHarnessClient';
import { useHarness } from '../harness/useHarness';
import type { HarnessController } from '../harness/useHarness';
import { useHarnessEvents } from '../harness/useHarnessEvents';

/**
 * The harness as the Workstream board reads it: one controller for the page,
 * fed by the live event feed while the harness is on. The development fixture
 * is honoured the way the Harness page honours it.
 */
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
