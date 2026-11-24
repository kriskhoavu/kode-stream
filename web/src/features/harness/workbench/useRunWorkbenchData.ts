import { useEffect, useState } from 'react';
import type { HarnessRunFileDiff, HarnessRunPlan, SessionChanges } from '../../../lib/types';
import type { HarnessClient } from '../harnessClient';
import { plainError } from '../useHarness';
import { type Loaded } from './workbenchTypes';

export function useRunPlan(client: HarnessClient, runId: string, version: string): Loaded<HarnessRunPlan> {
	const [state, setState] = useState<Loaded<HarnessRunPlan>>({ error: '' });
	useEffect(() => {
		if (!client.runPlan) return;
		const controller = new AbortController();
		client.runPlan(runId, controller.signal)
			.then((value) => { if (!controller.signal.aborted) setState({ value, error: '' }); })
			.catch((caught) => { if (!controller.signal.aborted) setState({ error: plainError(caught, 'The plan could not be read.') }); });
		return () => controller.abort();
	}, [client, runId, version]);
	return state;
}

export function useRunChanges(client: HarnessClient, runId: string, version: string): Loaded<SessionChanges> {
	const [state, setState] = useState<Loaded<SessionChanges>>({ error: '' });
	useEffect(() => {
		const controller = new AbortController();
		client.runChanges(runId, controller.signal)
			.then((value) => { if (!controller.signal.aborted) setState({ value, error: '' }); })
			.catch((caught) => { if (!controller.signal.aborted) setState({ error: plainError(caught, 'The changes could not be read.') }); });
		return () => controller.abort();
	}, [client, runId, version]);
	return state;
}

export function useFileDiff(client: HarnessClient, runId: string, path: string | undefined, version: string): Loaded<HarnessRunFileDiff> {
	const [state, setState] = useState<Loaded<HarnessRunFileDiff>>({ error: '' });
	useEffect(() => {
		setState({ error: '' });
		if (!path || !client.runFileDiff) return;
		const controller = new AbortController();
		client.runFileDiff(runId, path, controller.signal)
			.then((value) => { if (!controller.signal.aborted) setState({ value, error: '' }); })
			.catch((caught) => { if (!controller.signal.aborted) setState({ error: plainError(caught, 'The diff could not be read.') }); });
		return () => controller.abort();
	}, [client, runId, path, version]);
	return state;
}

