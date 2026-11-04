import { useEffect, useMemo, useRef, useState } from 'react';
import type { HarnessLocation, HarnessView } from '../app/router';
import type { HarnessClient } from '../features/harness/harnessClient';
import { loadHarnessClient, wantsHarnessFixture } from '../features/harness/loadHarnessClient';
import { apiHarnessClient } from '../features/harness/harnessClient';
import { useHarness } from '../features/harness/useHarness';
import { queueTasks, waitingApprovals } from '../features/harness/harnessState';
import { InboxView } from '../features/harness/InboxView';
import { ApprovalsView } from '../features/harness/ApprovalsView';
import { QueueList } from '../features/harness/QueueList';
import { plural } from '../features/harness/vocabulary';
import '../features/harness/harness.css';

const sections: { view: HarnessView; label: string }[] = [
	{ view: 'inbox', label: 'Inbox' },
	{ view: 'queue', label: 'Queue' },
	{ view: 'approvals', label: 'Approvals' },
	{ view: 'settings', label: 'Settings' }
];

export function HarnessPage({ location, onLocationChange, onWaitingCountChange }: {
	location: HarnessLocation;
	onLocationChange: (location: HarnessLocation) => void;
	onWaitingCountChange?: (count: number) => void;
}) {
	const [client, setClient] = useState<HarnessClient | undefined>(() => wantsHarnessFixture() ? undefined : apiHarnessClient);
	useEffect(() => {
		if (!client) void loadHarnessClient().then(setClient);
	}, [client]);
	if (!client) return <section className="hx empty-state">Loading the harness…</section>;
	return <HarnessScreens client={client} location={location} onLocationChange={onLocationChange} onWaitingCountChange={onWaitingCountChange} />;
}

function HarnessScreens({ client, location, onLocationChange, onWaitingCountChange }: {
	client: HarnessClient;
	location: HarnessLocation;
	onLocationChange: (location: HarnessLocation) => void;
	onWaitingCountChange?: (count: number) => void;
}) {
	const harness = useHarness(client);
	const { state } = harness;
	const waiting = waitingApprovals(state);
	const headingRef = useRef<HTMLHeadingElement | null>(null);
	const newestApprovalId = useNewArrival(state.loaded, waiting.map((approval) => approval.id));
	const openRun = (runId: string) => onLocationChange({ view: 'run', runId });

	useEffect(() => {
		onWaitingCountChange?.(waiting.length);
	}, [onWaitingCountChange, waiting.length]);

	useEffect(() => {
		// The shell focuses #app-main on every route; the heading is the better landing point inside the harness.
		const frame = window.requestAnimationFrame(() => headingRef.current?.focus());
		return () => window.cancelAnimationFrame(frame);
	}, [location.view]);

	const politeText = state.announcement && !state.announcement.assertive
		? state.announcement.text
		: state.loaded ? `${plural(waiting.length, 'item')} waiting on you` : '';
	const activeSection = location.view === 'run' ? undefined : location.view;

	return (
		<section className="hx hx-page">
			<div className="page-title hx-title">
				<h1 ref={headingRef} tabIndex={-1}>Harness</h1>
				<nav className="hx-seg" aria-label="Harness sections">
					{sections.map((section) => (
						<button key={section.view} type="button" className={activeSection === section.view ? 'on' : undefined} aria-current={activeSection === section.view ? 'page' : undefined} onClick={() => onLocationChange({ view: section.view })}>
							{section.label}
							{section.view === 'approvals' && waiting.length > 0 && <span className="hx-seg-count" aria-hidden="true">{waiting.length}</span>}
						</button>
					))}
				</nav>
				{client.fixture && <span className="hx-chip hx-demo-chip" title="Showing demo data from the development fixture">Demo data</span>}
				<span className="hx-title-status hx-muted" role="status" aria-live="polite">{politeText}</span>
			</div>
			{location.view === 'inbox' && state.announcement?.assertive && <span className="sr-only" role="alert" aria-live="assertive" key={state.announcement.id}>{state.announcement.text}</span>}

			<HarnessBody harness={harness} location={location} newestApprovalId={newestApprovalId} onOpenRun={openRun} onLocationChange={onLocationChange} />
		</section>
	);
}

function HarnessBody({ harness, location, newestApprovalId, onOpenRun, onLocationChange }: {
	harness: ReturnType<typeof useHarness>;
	location: HarnessLocation;
	newestApprovalId?: string;
	onOpenRun: (runId: string) => void;
	onLocationChange: (location: HarnessLocation) => void;
}) {
	const { state } = harness;
	const tasks = useMemo(() => queueTasks(state), [state]);
	if (!state.loaded && !state.error) return <section className="hx-card hx-state" aria-busy="true">Loading the harness…</section>;
	if (!state.loaded && state.error) {
		return (
			<section className="hx-card hx-state" role="alert">
				<p>{state.error}</p>
				<button type="button" className="primary" onClick={() => void harness.refresh()}>Try again</button>
			</section>
		);
	}
	if (state.status && !state.status.storageSupported) {
		return <section className="hx-card hx-state" role="alert"><p>The harness needs SQLite storage to keep its queue. Switch Storage to SQLite in Settings, then restart Kode Stream.</p></section>;
	}
	if (state.status && !state.status.enabled && location.view !== 'settings') {
		return (
			<section className="hx-card hx-state">
				<p>The harness is off. Nothing is looked at, planned or built until it is turned on.</p>
				<button type="button" className="primary" onClick={() => onLocationChange({ view: 'settings' })}>Open harness settings</button>
			</section>
		);
	}
	return (
		<>
			{state.error && <p className="hx-banner" role="alert">{state.error}</p>}
			{location.view === 'inbox' && <InboxView state={state} newestApprovalId={newestApprovalId} onApprove={harness.approve} onReject={harness.reject} onOpenRun={onOpenRun} />}
			{location.view === 'approvals' && <ApprovalsView state={state} onApprove={harness.approve} onReject={harness.reject} onOpenRun={onOpenRun} />}
			{location.view === 'queue' && <QueueList tasks={tasks} allTasks={state.tasks} onPin={harness.pin} onOpen={(task) => task.runId && onOpenRun(task.runId)} />}
			{location.view === 'run' && <section className="hx-card hx-state"><p>Loading progress for this run…</p></section>}
			{location.view === 'settings' && <section className="hx-card hx-state"><p>Harness settings are not available yet.</p></section>}
		</>
	);
}

/** The id of an approval that appeared after the first snapshot, so the Inbox can draw attention to it once. */
function useNewArrival(loaded: boolean, ids: string[]): string | undefined {
	const known = useRef<Set<string> | undefined>(undefined);
	const [newest, setNewest] = useState<string | undefined>();
	// `key` stands for `ids`, so a new array with the same ids does not re-run the effect.
	const key = ids.join(',');
	useEffect(() => {
		if (!loaded) return;
		if (!known.current) {
			known.current = new Set(ids);
			return;
		}
		const fresh = ids.find((id) => !known.current?.has(id));
		ids.forEach((id) => known.current?.add(id));
		if (fresh) setNewest(fresh);
	}, [loaded, key]);
	return newest;
}
