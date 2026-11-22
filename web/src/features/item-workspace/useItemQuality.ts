import { useEffect, useMemo, useRef, useState } from 'react';
import type { E2ERunbook, ItemDetail, ItemVerificationTests, VerificationJob, VerificationTestSelection, VerifyProfile, WorkspaceConfig, WorkspaceTreeEntry } from '../../lib/types';
import { api } from '../../shared/api';
import { isAutomationSpecPath, normalizeLocalPath } from './itemPaths';

/**
 * An item's Quality: its verification job and the polling that follows it,
 * the automation specs it runs, the E2E runbooks that cover it, and the spec
 * browser and artifact preview the Quality panel opens.
 */
export function useItemQuality({ plan, workspaceConfig, workspaces }: { plan: ItemDetail | null; workspaceConfig: WorkspaceConfig | null; workspaces: WorkspaceConfig[] }) {
  const verificationTestsAbortRef = useRef<AbortController | undefined>(undefined);
  const e2eRunbooksAbortRef = useRef<AbortController | undefined>(undefined);
  const specPickerAbortRef = useRef<AbortController | undefined>(undefined);
  const [verificationJob, setVerificationJob] = useState<VerificationJob | null>(null);
  const [verificationBusy, setVerificationBusy] = useState(false);
  const [verificationError, setVerificationError] = useState('');
  const [verificationTests, setVerificationTests] = useState<ItemVerificationTests | null>(null);
  const [e2eRunbooks, setE2ERunbooks] = useState<{ runbooks: E2ERunbook[]; diagnostic?: string }>({ runbooks: [] });
  const [verificationTestsBusy, setVerificationTestsBusy] = useState(false);
  const [automationLaunchBusy, setAutomationLaunchBusy] = useState(false);
  const [manualSpec, setManualSpec] = useState('');
  const [specPickerOpen, setSpecPickerOpen] = useState(false);
  const [specPickerPath, setSpecPickerPath] = useState('');
  const [specPickerEntries, setSpecPickerEntries] = useState<WorkspaceTreeEntry[]>([]);
  const [specPickerSelected, setSpecPickerSelected] = useState<string[]>([]);
  const [specPickerLoading, setSpecPickerLoading] = useState(false);
  const [specPickerError, setSpecPickerError] = useState('');
  const [artifactPreview, setArtifactPreview] = useState<{ title: string; path: string; content: string; loading: boolean; error: string } | null>(null);
  useEffect(() => {
    verificationTestsAbortRef.current?.abort();
    if (!plan) {
      setVerificationTests(null);
      setVerificationTestsBusy(false);
      return;
    }
    const controller = new AbortController();
    verificationTestsAbortRef.current = controller;
    setVerificationTestsBusy(true);
    const loadVerificationTests = api.itemVerificationTests;
    if (typeof loadVerificationTests !== 'function') {
      setVerificationTests(null);
      setVerificationTestsBusy(false);
      verificationTestsAbortRef.current = undefined;
      return;
    }
    void loadVerificationTests(plan.id, controller.signal)
      .then((tests) => { if (!controller.signal.aborted) setVerificationTests(tests); })
      .catch(() => { if (!controller.signal.aborted) setVerificationTests(null); })
      .finally(() => {
        if (!controller.signal.aborted) setVerificationTestsBusy(false);
        if (verificationTestsAbortRef.current === controller) verificationTestsAbortRef.current = undefined;
      });
    return () => controller.abort();
  }, [plan?.id]);
  const loadE2ERunbooks = () => {
    e2eRunbooksAbortRef.current?.abort();
    if (!plan) {
      setE2ERunbooks({ runbooks: [] });
      return;
    }
    const load = api.itemE2ERunbooks;
    if (typeof load !== 'function') { setE2ERunbooks({ runbooks: [] }); return; }
    const controller = new AbortController();
    e2eRunbooksAbortRef.current = controller;
    void load(plan.id, controller.signal)
      .then((result) => { if (!controller.signal.aborted) setE2ERunbooks(result ?? { runbooks: [] }); })
      .catch(() => { if (!controller.signal.aborted) setE2ERunbooks({ runbooks: [], diagnostic: 'E2E coverage could not be loaded.' }); })
      .finally(() => {
        if (e2eRunbooksAbortRef.current === controller) e2eRunbooksAbortRef.current = undefined;
      });
  };

  useEffect(() => {
    loadE2ERunbooks();
    return () => e2eRunbooksAbortRef.current?.abort();
  }, [plan?.id]);
  useEffect(() => {
    if (!plan || !verificationJob || verificationJob.status === 'failed' || verificationJob.status === 'passed') return;
    const controller = new AbortController();
    let inFlight = false;
    const timer = window.setInterval(() => {
      if (inFlight) return;
      inFlight = true;
      void api.verificationJob(plan.workspaceId, verificationJob.id, controller.signal)
        .then((job) => {
          if (controller.signal.aborted) return;
          setVerificationJob(job);
        })
        .catch(() => undefined)
        .finally(() => { inFlight = false; });
    }, 1000);
    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, [plan?.workspaceId, verificationJob?.id, verificationJob?.status]);
  const runVerification = async (profile: VerifyProfile) => {
    if (!plan) return;
    setVerificationBusy(true);
    setVerificationError('');
    try {
      const job = await api.createVerificationJob(plan.workspaceId, { profile, trigger: 'manual_checkpoint', terminalMode: 'embedded' });
      setVerificationJob(job);
    } catch (err) {
      setVerificationError(err instanceof Error ? err.message : 'Failed to start verification');
    } finally {
      setVerificationBusy(false);
    }
  };

  const saveVerificationTests = async (selection: VerificationTestSelection) => {
    if (!plan) return null;
    setVerificationTestsBusy(true);
    setVerificationError('');
    try {
		const tests = await api.saveItemVerificationTests(plan.id, { ...selection, expectedRevision: verificationTests?.selection.revision ?? plan.metadataRevision });
      setVerificationTests(tests);
      return tests;
    } catch (err) {
      setVerificationError(err instanceof Error ? err.message : 'Failed to save automation tests');
      return null;
    } finally {
      setVerificationTestsBusy(false);
    }
  };

  const selectedSpecs = verificationTests?.selection.selectedSpecs ?? [];
  const automationEnvironment = verificationTests?.selection.environment || workspaceConfig?.runtime?.automation?.defaultEnvironment || 'local';
  const automationDisplayMode = verificationTests?.selection.displayMode === 'visible' ? 'visible' : 'silent';
  const automationJobActive = verificationJob?.mode === 'automation' && (verificationJob.status === 'queued' || verificationJob.status === 'running');
  const automationRunBusy = automationLaunchBusy || automationJobActive;
  const automationRepoPath = workspaceConfig?.runtime?.automation?.repositoryPath ?? '';
  const automationWorkspace = useMemo(() => {
    const target = normalizeLocalPath(automationRepoPath);
    if (!target) return undefined;
    return workspaces.find((workspace) => normalizeLocalPath(workspace.path) === target);
  }, [automationRepoPath, workspaces]);
  const setSelectedSpecs = async (specs: string[]) => {
    await saveVerificationTests({ selectedSpecs: specs, environment: automationEnvironment, displayMode: automationDisplayMode });
  };
  const addSelectedSpec = async (spec: string) => {
    const clean = spec.trim();
    if (!clean || selectedSpecs.includes(clean)) return;
    await setSelectedSpecs([...selectedSpecs, clean]);
  };
  const removeSelectedSpec = async (spec: string) => {
    await setSelectedSpecs(selectedSpecs.filter((candidate) => candidate !== spec));
  };
  const saveAutomationEnvironment = async (environment: string) => {
    await saveVerificationTests({ selectedSpecs, environment, displayMode: automationDisplayMode });
  };
  const saveAutomationDisplayMode = async (displayMode: 'silent' | 'visible') => {
    await saveVerificationTests({ selectedSpecs, environment: automationEnvironment, displayMode });
  };
  const addManualSpec = async () => {
    const clean = manualSpec.trim();
    if (!clean) return;
    await addSelectedSpec(clean);
    setManualSpec('');
  };
  const openSpecPicker = () => {
    setSpecPickerPath('');
    setSpecPickerSelected([]);
    setSpecPickerError('');
    setSpecPickerOpen(true);
  };
  const loadSpecPickerDirectory = async (path: string) => {
    if (!automationWorkspace) return;
    specPickerAbortRef.current?.abort();
    const controller = new AbortController();
    specPickerAbortRef.current = controller;
    setSpecPickerLoading(true);
    setSpecPickerError('');
    try {
      const listing = await api.workspaceTree(automationWorkspace.id, path, false, false, controller.signal);
      if (controller.signal.aborted) return;
      setSpecPickerPath(listing.path);
      setSpecPickerEntries(listing.entries.filter((entry) => entry.type === 'directory' || isAutomationSpecPath(entry.path)));
    } catch (err) {
      if (!controller.signal.aborted) setSpecPickerError(err instanceof Error ? err.message : 'Failed to browse automation specs');
    } finally {
      if (!controller.signal.aborted) setSpecPickerLoading(false);
      if (specPickerAbortRef.current === controller) specPickerAbortRef.current = undefined;
    }
  };
  const togglePickerSpec = (path: string) => {
    setSpecPickerSelected((current) => current.includes(path) ? current.filter((spec) => spec !== path) : [...current, path]);
  };
  const addPickedSpecs = async () => {
    const nextSpecs = [...selectedSpecs];
    specPickerSelected.forEach((spec) => {
      if (!nextSpecs.includes(spec)) nextSpecs.push(spec);
    });
    await setSelectedSpecs(nextSpecs);
    setSpecPickerOpen(false);
    setSpecPickerSelected([]);
  };

  useEffect(() => {
    if (!specPickerOpen || !automationWorkspace) {
      specPickerAbortRef.current?.abort();
      setSpecPickerLoading(false);
      return;
    }
    void loadSpecPickerDirectory(specPickerPath);
    return () => specPickerAbortRef.current?.abort();
  }, [specPickerOpen, automationWorkspace?.id, specPickerPath]);

  const runAutomationVerification = async () => {
    if (!plan) return;
    setVerificationBusy(true);
    setAutomationLaunchBusy(true);
    setVerificationError('');
    try {
      const tests = await saveVerificationTests({ selectedSpecs, environment: automationEnvironment, displayMode: automationDisplayMode });
      const specs = tests?.selection.selectedSpecs ?? selectedSpecs;
      const environment = tests?.selection.environment || automationEnvironment;
      const displayMode = tests?.selection.displayMode === 'visible' ? 'visible' : automationDisplayMode;
      const job = await api.createVerificationJob(plan.workspaceId, { mode: 'automation', environment, displayMode, selectedSpecs: specs, trigger: 'manual_checkpoint', terminalMode: 'embedded' });
      setVerificationJob(job);
    } catch (err) {
      setVerificationError(err instanceof Error ? err.message : 'Failed to start automation verification');
    } finally {
      setVerificationBusy(false);
      setAutomationLaunchBusy(false);
    }
  };

  const rerunVerification = async (profile?: VerifyProfile) => {
    if (!plan || !verificationJob) return;
    setVerificationBusy(true);
    setVerificationError('');
    try {
      const job = await api.rerunVerificationJob(plan.workspaceId, verificationJob.id, profile);
      setVerificationJob(job);
    } catch (err) {
      setVerificationError(err instanceof Error ? err.message : 'Failed to rerun verification');
    } finally {
      setVerificationBusy(false);
    }
  };

  const artifactAbsolutePath = (artifact: { root?: string; path: string }) => {
    // Older job payloads do not carry the automation root. The workspace runtime
    // configuration remains the authoritative compatible fallback for their locators.
    const root = artifact.root === 'automation' ? verificationJob?.automationRepoPath || automationRepoPath : workspaceConfig?.path;
    if (!root || artifact.path.startsWith('/') || artifact.path.includes('..')) return '';
    return `${root.replace(/\/$/, '')}/${artifact.path}`;
  };

  const openArtifactPath = async (artifact: { root?: string; path: string }) => {
    const path = artifactAbsolutePath(artifact);
    if (!path) {
      setVerificationError('Could not resolve the artifact path safely.');
      return;
    }
    try {
      await api.openPath(path);
    } catch {
      setVerificationError(`Could not open artifact path: ${path}`);
    }
  };

  const previewArtifact = async (kind: string, artifact: { root?: string; path: string }) => {
    if (!plan) return;
    const absolutePath = artifactAbsolutePath(artifact);
    setArtifactPreview({ title: kind, path: artifact.path, content: '', loading: true, error: '' });
    if (artifact.root !== 'workspace' || !absolutePath) {
      setArtifactPreview({ title: kind, path: artifact.path, content: '', loading: false, error: 'Preview unavailable for this artifact path. Use Open to view it externally.' });
      return;
    }
    try {
      const file = await api.workspaceFile(plan.workspaceId, artifact.path);
      setArtifactPreview({ title: kind, path: artifact.path, content: file.content, loading: false, error: '' });
    } catch (caught) {
      setArtifactPreview({ title: kind, path: artifact.path, content: '', loading: false, error: caught instanceof Error ? caught.message : 'Could not load artifact preview' });
    }
  };


  return { verificationJob, setVerificationJob, verificationBusy, setVerificationBusy, verificationError, setVerificationError, verificationTests, setVerificationTests, e2eRunbooks, setE2ERunbooks, verificationTestsBusy, setVerificationTestsBusy, automationLaunchBusy, setAutomationLaunchBusy, manualSpec, setManualSpec, specPickerOpen, setSpecPickerOpen, specPickerPath, setSpecPickerPath, specPickerEntries, setSpecPickerEntries, specPickerSelected, setSpecPickerSelected, specPickerLoading, setSpecPickerLoading, specPickerError, setSpecPickerError, artifactPreview, setArtifactPreview, loadE2ERunbooks, runVerification, saveVerificationTests, selectedSpecs, automationEnvironment, automationDisplayMode, automationJobActive, automationRunBusy, automationRepoPath, automationWorkspace, setSelectedSpecs, addSelectedSpec, removeSelectedSpec, saveAutomationEnvironment, saveAutomationDisplayMode, addManualSpec, openSpecPicker, loadSpecPickerDirectory, togglePickerSpec, addPickedSpecs, runAutomationVerification, rerunVerification, artifactAbsolutePath, openArtifactPath, previewArtifact };
}

export type ItemQuality = ReturnType<typeof useItemQuality>;
