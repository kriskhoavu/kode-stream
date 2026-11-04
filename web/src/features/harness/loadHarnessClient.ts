/// <reference types="vite/client" />
import { readStringPreference, removePreference, writePreference } from '../../shared/preferences/store';
import { apiHarnessClient } from './harnessClient';
import type { HarnessClient } from './harnessClient';

const fixturePreference = 'harnessFixture';
let fixtureClient: Promise<HarnessClient> | undefined;

/**
 * Development only: `VITE_HARNESS_FIXTURE=1 npm run dev`, or open any harness
 * page with `?fixture=1` (remembered; `?fixture=0` forgets it). In a
 * production build `import.meta.env.DEV` is false, so the fixture module is
 * never bundled.
 */
export function wantsHarnessFixture(search = globalThis.location?.search ?? ''): boolean {
	if (!import.meta.env.DEV || import.meta.env.MODE === 'test') return false;
	const flag = new URLSearchParams(search).get('fixture');
	if (flag === '1') writePreference(fixturePreference, '1');
	if (flag === '0') removePreference(fixturePreference);
	return import.meta.env.VITE_HARNESS_FIXTURE === '1' || readStringPreference(fixturePreference) === '1';
}

export function loadHarnessClient(): Promise<HarnessClient> {
	if (import.meta.env.DEV && wantsHarnessFixture()) {
		// One fixture per page load, so decisions survive leaving and returning to the harness.
		fixtureClient ??= import('./fixture').then((module) => module.createFixtureClient());
		return fixtureClient;
	}
	return Promise.resolve(apiHarnessClient);
}
