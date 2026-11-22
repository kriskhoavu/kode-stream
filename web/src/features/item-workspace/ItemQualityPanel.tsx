import type { RefObject } from 'react';
import { useModalDialog } from '../../components/overlay';
import { ModalLayer } from '../../components/ModalLayer';
import type { ItemQuality } from './useItemQuality';
import { FolderOpen, X } from 'lucide-react';
import type { ItemDetail, WorkspaceConfig } from '../../lib/types';
import { E2EQualityPanel } from '../e2e-testing/E2EQualityPanel';
import { verificationTriggerDescription, verificationTriggerLabel, verificationTriggerTone, formatArtifactKind, formatAt, formatBytes } from './itemFormat';
import { parentPath } from './itemPaths';

/** The Quality tab: verification, automation specs, E2E coverage, and the dialogs they open. */
export function ItemQualityPanel({ quality, plan, workspaceConfig }: { quality: ItemQuality; plan: ItemDetail | null; workspaceConfig: WorkspaceConfig | null }) {
  const { verificationJob, verificationBusy, verificationError, verificationTests, e2eRunbooks, verificationTestsBusy, manualSpec, setManualSpec, specPickerOpen, setSpecPickerOpen, specPickerPath, setSpecPickerPath, specPickerEntries, specPickerSelected, specPickerLoading, specPickerError, artifactPreview, setArtifactPreview, loadE2ERunbooks, runVerification, selectedSpecs, automationEnvironment, automationDisplayMode, automationRunBusy, automationWorkspace, addSelectedSpec, removeSelectedSpec, saveAutomationEnvironment, saveAutomationDisplayMode, addManualSpec, openSpecPicker, togglePickerSpec, addPickedSpecs, runAutomationVerification, rerunVerification, openArtifactPath, previewArtifact } = quality;
  const artifactDialog = useModalDialog(() => setArtifactPreview(null), true, Boolean(artifactPreview));
  const specPickerDialog = useModalDialog(() => setSpecPickerOpen(false), true, specPickerOpen);
  return (
    <>
    <section className="metadata-callout quality-panel" aria-label="Quality">
      <div className="verification-header">
        <strong>Quality</strong>
        {verificationJob && <span className={`verification-trigger-badge ${verificationTriggerTone(verificationJob)}`}>{verificationTriggerLabel(verificationJob)}</span>}
      </div>
      <div className="verification-actions">
        <button className="secondary" type="button" onClick={() => void runVerification('smoke')} disabled={verificationBusy || !plan?.workspaceId || !workspaceConfig?.runtime}>Run smoke verify</button>
        <button className="secondary" type="button" onClick={() => void runVerification('critical')} disabled={verificationBusy || !plan?.workspaceId || !workspaceConfig?.runtime}>Run critical verify</button>
        <button className="secondary" type="button" onClick={() => void rerunVerification()} disabled={verificationBusy || !verificationJob}>Re-run latest</button>
      </div>
      {!plan?.workspaceId && <span className="verification-note">No workspace selected.</span>}
      {plan?.workspaceId && !workspaceConfig?.runtime && <span className="verification-note">Runtime not configured for this workspace.</span>}
      {workspaceConfig?.runtime && !workspaceConfig.runtime.automation?.enabled && <span className="verification-note">Automation tests are not configured for this workspace.</span>}
      {workspaceConfig?.runtime?.automation?.enabled && (
        <div className="automation-test-panel">
          <div className="quality-section-heading">
            <strong>Automation</strong>
            <span>Environment and card-linked specs</span>
          </div>
          <label className="repo-field">Automation environment<input value={automationEnvironment} onChange={(event) => void saveAutomationEnvironment(event.target.value)} placeholder="local" /></label>
          <div className="automation-mode-field">
            <span>Run mode</span>
            <div className="segmented-control automation-mode-toggle" role="tablist" aria-label="Automation run mode">
              <button type="button" className={automationDisplayMode === 'silent' ? 'active' : ''} aria-selected={automationDisplayMode === 'silent'} onClick={() => void saveAutomationDisplayMode('silent')}>Silent</button>
              <button type="button" className={automationDisplayMode === 'visible' ? 'active' : ''} aria-selected={automationDisplayMode === 'visible'} onClick={() => void saveAutomationDisplayMode('visible')}>Visible browser</button>
            </div>
          </div>
          <div className="quality-section-heading compact">
            <strong>Selected specs</strong>
            {!automationWorkspace && <span>Register the automation repository as a workspace to browse specs.</span>}
          </div>
          <div className="automation-spec-list" aria-label="Selected automation specs">
            {selectedSpecs.length ? selectedSpecs.map((spec) => <span className="automation-spec-chip" key={spec}>{spec}<button type="button" aria-label={`Remove ${spec}`} onClick={() => void removeSelectedSpec(spec)}>×</button></span>) : <span className="verification-note">No selected specs.</span>}
          </div>
          <div className="path-input-row manual-spec-row">
            <input aria-label="Manual automation spec" value={manualSpec} onChange={(event) => setManualSpec(event.target.value)} placeholder="Repo-relative spec path" />
            <button className="secondary" type="button" onClick={() => void addManualSpec()} disabled={!manualSpec.trim()}>Add spec</button>
            <button className="secondary browse-specs-action" type="button" onClick={openSpecPicker} disabled={!automationWorkspace}>Browse</button>
          </div>
          <div className="automation-action-row">
            <button className={automationRunBusy ? 'secondary automation-run-action progress-button active' : 'secondary automation-run-action'} type="button" onClick={() => void runAutomationVerification()} disabled={automationRunBusy || verificationTestsBusy || !plan?.workspaceId || !workspaceConfig?.runtime?.automation?.enabled || selectedSpecs.length === 0}>
              <span>{automationRunBusy ? (automationDisplayMode === 'visible' ? 'Starting browser...' : 'Running automation...') : 'Run automation tests'}</span>
              {automationRunBusy && <span className="button-progress-bar" aria-hidden="true" />}
            </button>
          </div>
          <div className="automation-suggestions">
            <div className="quality-subsection-heading">
              <strong>Suggested specs</strong>
              <span>From automation-test in plan.yaml</span>
            </div>
            {verificationTests?.discoveredSpecs?.length ? (
              <div className="automation-discovered-specs" aria-label="Suggested automation specs">
                {verificationTests.discoveredSpecs.map((spec) => {
                  const selected = selectedSpecs.includes(spec.path);
                  return (
                    <article className={selected ? 'automation-suggestion-card selected' : 'automation-suggestion-card'} key={`${spec.path}-${spec.sourcePath ?? ''}`}>
                      <div>
                        <strong>{spec.path}</strong>
                        <span>{spec.runner}{spec.sourcePath ? ` · ${spec.sourcePath}` : ''}</span>
                      </div>
                      <button className="secondary" type="button" disabled={selected} onClick={() => void addSelectedSpec(spec.path)}>
                        {selected ? 'Selected' : 'Select'}
                      </button>
                    </article>
                  );
                })}
              </div>
            ) : <span className="verification-note">No suggested specs from plan.yaml.</span>}
          </div>
        </div>
      )}
		{plan?.workspaceId && <E2EQualityPanel workspaceId={plan.workspaceId} runbooks={e2eRunbooks.runbooks} diagnostic={e2eRunbooks.diagnostic} onRefresh={loadE2ERunbooks} />}
      {verificationBusy && <span className="verification-note">Starting verification...</span>}
      {verificationError && <span className="error" role="alert">{verificationError}</span>}
      {verificationJob && <span className="verification-status">{verificationJob.mode === 'automation' ? 'automation' : verificationJob.profile} · {verificationJob.status}{verificationJob.failureType ? ` (${verificationJob.failureType})` : ''}</span>}
      {verificationJob && <span className="verification-note">{verificationTriggerDescription(verificationJob)}</span>}
      {verificationJob?.steps?.length ? (
        <div className="verification-steps">
          {verificationJob.steps.map((step) => (
            <span className={`verification-step ${step.status === 'ok' ? 'ok' : 'failed'}`} key={`${step.step}-${step.at}`}>{step.step}: {step.status}</span>
          ))}
        </div>
      ) : null}
      {verificationJob?.artifacts?.length ? (
        <div className="verification-artifacts">
          {verificationJob.artifacts.map((artifact) => (
            <article className="verification-artifact-card" key={`${artifact.kind}-${artifact.path}`}>
              <div className="verification-artifact-meta">
                <strong>{formatArtifactKind(artifact.kind, artifact.path)}</strong>
                <span>{artifact.path}</span>
                <small>{formatBytes(artifact.sizeBytes)} · {formatAt(artifact.createdAt)}</small>
              </div>
              <div className="verification-artifact-actions">
                <button className="secondary" type="button" onClick={() => void previewArtifact(formatArtifactKind(artifact.kind, artifact.path), artifact)}>Preview</button>
                <button className="secondary" type="button" onClick={() => void openArtifactPath(artifact)}>Open</button>
              </div>
            </article>
          ))}
        </div>
      ) : null}
    </section>
      {artifactPreview && (
        <ModalLayer><section className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setArtifactPreview(null); }}>
          <div ref={artifactDialog.ref as RefObject<HTMLDivElement>} className="modal-panel artifact-preview-modal" role="dialog" aria-modal="true" aria-label="Artifact preview">
            <header>
              <div>
                <h2>{artifactPreview.title}</h2>
                <span>{artifactPreview.path}</span>
              </div>
            </header>
            {artifactPreview.loading && <p>Loading preview...</p>}
            {!artifactPreview.loading && artifactPreview.error && <p className="error">{artifactPreview.error}</p>}
            {!artifactPreview.loading && !artifactPreview.error && <pre className="artifact-preview-content">{artifactPreview.content || 'No text content available.'}</pre>}
            <div className="modal-actions">
              <button data-autofocus className="secondary" type="button" onClick={() => void openArtifactPath({ root: 'workspace', path: artifactPreview.path })}>Open externally</button>
              <button className="primary" type="button" onClick={() => setArtifactPreview(null)}>Close</button>
            </div>
          </div>
        </section></ModalLayer>
      )}
      {specPickerOpen && (
        <ModalLayer><section className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setSpecPickerOpen(false); }}>
          <div ref={specPickerDialog.ref as RefObject<HTMLDivElement>} className="modal-panel spec-picker-modal" role="dialog" aria-modal="true" aria-label="Browse automation specs">
            <header>
              <div>
                <h2>Browse automation specs</h2>
                <span>{automationWorkspace?.name ?? 'Automation repository'} / {specPickerPath || 'root'}</span>
              </div>
              <button data-autofocus className="icon-button" type="button" aria-label="Close spec browser" onClick={() => setSpecPickerOpen(false)}><X size={16} /></button>
            </header>
            {specPickerPath && (
              <button className="secondary spec-picker-up" type="button" onClick={() => setSpecPickerPath(parentPath(specPickerPath))}>Up one folder</button>
            )}
            {specPickerError && <p className="error" role="alert">{specPickerError}</p>}
            <div className="spec-picker-list" aria-label="Automation spec files">
              {specPickerLoading && <span className="verification-note">Loading specs...</span>}
              {!specPickerLoading && specPickerEntries.length === 0 && <span className="verification-note">No spec files in this folder.</span>}
              {!specPickerLoading && specPickerEntries.map((entry) => entry.type === 'directory' ? (
                <button className="spec-picker-row directory" type="button" key={entry.path} onClick={() => setSpecPickerPath(entry.path)}>
                  <FolderOpen size={15} /> <span>{entry.name}</span>
                </button>
              ) : (
                <label className="spec-picker-row" key={entry.path}>
                  <input type="checkbox" checked={specPickerSelected.includes(entry.path)} disabled={selectedSpecs.includes(entry.path)} onChange={() => togglePickerSpec(entry.path)} />
                  <span>{entry.path}</span>
                  {selectedSpecs.includes(entry.path) && <small>Selected</small>}
                </label>
              ))}
            </div>
            <footer className="modal-actions">
              <button className="secondary" type="button" onClick={() => setSpecPickerOpen(false)}>Cancel</button>
              <button className="primary" type="button" disabled={specPickerSelected.length === 0 || verificationTestsBusy} onClick={() => void addPickedSpecs()}>Add {specPickerSelected.length || ''} spec{specPickerSelected.length === 1 ? '' : 's'}</button>
            </footer>
          </div>
        </section></ModalLayer>
      )}
    </>
  );
}
