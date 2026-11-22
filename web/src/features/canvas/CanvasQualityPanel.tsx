import { useEffect, useRef, useState } from 'react';
import { api } from '../../shared/api';
import type { AutomationDisplayMode, E2ERunbook, ItemVerificationTests, VerificationJob, VerificationTestSelection, VerifyProfile } from '../../lib/types';
import { E2EQualityPanel } from '../e2e-testing/E2EQualityPanel';

export function VerificationSummary({ job }: { job: VerificationJob }) {
	const freshness = job.freshness || 'inconclusive';
	const historicalPass = job.status === 'passed' && freshness !== 'fresh';
	return <dl className={`verification-summary result-${job.status} freshness-${freshness}`}><dt>Result</dt><dd>{historicalPass ? 'Passed (historical)' : job.status}</dd><dt>Freshness</dt><dd>{freshness}</dd><dt>Verified revision</dt><dd><code>{shortSHA(job.finishFingerprint?.commit || job.startFingerprint?.commit)}</code></dd><dt>Current revision</dt><dd><code>{shortSHA(job.currentFingerprint?.commit)}</code></dd></dl>;
}

export function CanvasQualityPanel({ itemId, workspaceId, capability, job, verifying, runbooks, onRun, onRunAutomation, onRerun, onRefresh }: { itemId: string; workspaceId?: string; capability?: { state: string; message?: string }; job?: VerificationJob; verifying: boolean; runbooks: { runbooks: E2ERunbook[]; diagnostic?: string }; onRun: (profile: VerifyProfile) => void; onRunAutomation: (input: { environment: string; displayMode: AutomationDisplayMode; selectedSpecs: string[] }) => void; onRerun: () => void; onRefresh: () => void }) {
	const canVerify = capability?.state === 'available';
	const [tests, setTests] = useState<ItemVerificationTests>();
	const [manualSpec, setManualSpec] = useState('');
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState('');
	const generationRef = useRef(0);
	useEffect(() => {
		const generation = ++generationRef.current;
		const controller = new AbortController();
		const load = api.itemVerificationTests;
		if (typeof load !== 'function') return;
		load(itemId, controller.signal).then((value) => !controller.signal.aborted && setTests(value)).catch((caught) => !controller.signal.aborted && setError(caught instanceof Error ? caught.message : 'Automation settings could not be loaded.'));
		return () => {
			controller.abort();
			if (generationRef.current === generation) generationRef.current += 1;
		};
	}, [itemId]);
	const selectedSpecs = tests?.selection.selectedSpecs ?? [];
	const environment = tests?.selection.environment || 'local';
	const displayMode: AutomationDisplayMode = tests?.selection.displayMode === 'visible' ? 'visible' : 'silent';
	const saveSelection = async (selection: VerificationTestSelection) => {
		const generation = generationRef.current;
		const isCurrent = () => generationRef.current === generation;
		setBusy(true); setError('');
		try {
			const next = await api.saveItemVerificationTests(itemId, { ...selection, expectedRevision: tests?.selection.revision });
			if (isCurrent()) setTests(next);
		}
		catch (caught) { if (isCurrent()) setError(caught instanceof Error ? caught.message : 'Automation settings could not be saved.'); }
		finally { if (isCurrent()) setBusy(false); }
	};
	const updateSpecs = (specs: string[]) => void saveSelection({ selectedSpecs: specs, environment, displayMode });
	const addManualSpec = () => { const spec = manualSpec.trim(); if (!spec || selectedSpecs.includes(spec)) return; updateSpecs([...selectedSpecs, spec]); setManualSpec(''); };
	return <section className="metadata-callout quality-panel canvas-quality-panel" aria-label="Quality"><div className="verification-header"><strong>Quality</strong>{job && <span className={`verification-trigger-badge ${job.status === 'passed' ? 'success' : job.status === 'failed' ? 'danger' : ''}`}>{job.profile ?? job.mode} · {job.status}</span>}</div><div className="verification-actions"><button className="secondary" type="button" disabled={!canVerify || verifying} title={!canVerify ? capability?.message : undefined} onClick={() => onRun('smoke')}>Run smoke verify</button><button className="secondary" type="button" disabled={!canVerify || verifying} title={!canVerify ? capability?.message : undefined} onClick={() => onRun('critical')}>Run critical verify</button><button className="secondary" type="button" disabled={!job || verifying} onClick={onRerun}>Re-run latest</button></div>{!workspaceId && <span className="verification-note">No workspace selected.</span>}{capability && !canVerify && <span className="verification-note">{capability.message || 'Verification is unavailable for this workspace.'}</span>}{workspaceId && <div className="automation-test-panel"><div className="quality-section-heading"><strong>Automation</strong><span>Environment and card-linked specs</span></div><label className="repo-field">Automation environment<input value={environment} onChange={(event) => setTests((current) => ({ selection: { selectedSpecs, environment: event.target.value, displayMode }, discoveredSpecs: current?.discoveredSpecs ?? [] }))} onBlur={() => void saveSelection({ selectedSpecs, environment, displayMode })} placeholder="local" /></label><div className="automation-mode-field"><span>Run mode</span><div className="segmented-control automation-mode-toggle" role="tablist" aria-label="Automation run mode"><button type="button" className={displayMode === 'silent' ? 'active' : ''} aria-selected={displayMode === 'silent'} onClick={() => void saveSelection({ selectedSpecs, environment, displayMode: 'silent' })}>Silent</button><button type="button" className={displayMode === 'visible' ? 'active' : ''} aria-selected={displayMode === 'visible'} onClick={() => void saveSelection({ selectedSpecs, environment, displayMode: 'visible' })}>Visible browser</button></div></div><div className="quality-section-heading compact"><strong>Selected specs</strong></div><div className="automation-spec-list" aria-label="Selected automation specs">{selectedSpecs.length ? selectedSpecs.map((spec) => <span className="automation-spec-chip" key={spec}>{spec}<button type="button" aria-label={`Remove ${spec}`} onClick={() => updateSpecs(selectedSpecs.filter((candidate) => candidate !== spec))}>×</button></span>) : <span className="verification-note">No selected specs.</span>}</div><div className="path-input-row manual-spec-row"><input aria-label="Manual automation spec" value={manualSpec} onChange={(event) => setManualSpec(event.target.value)} placeholder="Repo-relative spec path" /><button className="secondary" type="button" onClick={addManualSpec} disabled={!manualSpec.trim() || busy}>Add spec</button></div><div className="automation-action-row"><button className="secondary automation-run-action" type="button" onClick={() => onRunAutomation({ environment, displayMode, selectedSpecs })} disabled={!canVerify || verifying || busy || selectedSpecs.length === 0}>Run automation tests</button></div><div className="automation-suggestions"><div className="quality-subsection-heading"><strong>Suggested specs</strong><span>From automation-test in plan.yaml</span></div>{tests?.discoveredSpecs?.length ? <div className="automation-discovered-specs" aria-label="Suggested automation specs">{tests.discoveredSpecs.map((spec) => { const selected = selectedSpecs.includes(spec.path); return <article className={selected ? 'automation-suggestion-card selected' : 'automation-suggestion-card'} key={`${spec.path}-${spec.sourcePath ?? ''}`}><div><strong>{spec.path}</strong><span>{spec.runner}{spec.sourcePath ? ` · ${spec.sourcePath}` : ''}</span></div><button className="secondary" type="button" disabled={selected || busy} onClick={() => updateSpecs([...selectedSpecs, spec.path])}>{selected ? 'Selected' : 'Select'}</button></article>; })}</div> : <span className="verification-note">No suggested specs from plan.yaml.</span>}</div></div>}{verifying && <span className="verification-note" role="status">Starting verification...</span>}{job && <><VerificationSummary job={job} />{job.steps?.length ? <div className="verification-steps">{job.steps.map((step) => <span className={`verification-step ${step.status === 'ok' ? 'ok' : 'failed'}`} key={`${step.step}-${step.at}`}>{step.step}: {step.status}</span>)}</div> : null}</>}{workspaceId && <E2EQualityPanel variant="nested" workspaceId={workspaceId} runbooks={runbooks.runbooks} diagnostic={runbooks.diagnostic} onRefresh={onRefresh} />}{error && <p className="error" role="alert">{error}</p>}</section>;
}

export function shortSHA(value?: string) { return value ? value.slice(0, 8) : 'Unavailable'; }
