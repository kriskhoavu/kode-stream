import { type ReactNode, useEffect, useState } from 'react';
import { ChevronDown, ChevronRight, FolderOpen, Plus, X } from 'lucide-react';
import { ApiError, api } from '../../shared/api';
import type { JiraConnection, KnowledgeSettings, RuntimeType, WorkspaceRuntimeConfig } from '../../lib/types';
import { labels } from '../../lib/vocabulary';
import { parseSources } from './sourceSettings';
import { DEFAULT_SOURCES, emptyJiraConnection, defaultRuntimeConfig, errorMessage, normalizeJiraDraft, removeSource, toggleSource } from './workspaceDraft';

export function OverviewSection({ title, collapsed, onToggle, children }: { title: string; collapsed: boolean; onToggle: () => void; children: ReactNode }) {
  return <section className="workspace-overview-section">
    <button className="workspace-overview-section-toggle" type="button" onClick={onToggle} aria-expanded={!collapsed}>
      {collapsed ? <ChevronRight size={16} /> : <ChevronDown size={16} />}
      <span>{title}</span>
    </button>
    {!collapsed && <div className="workspace-overview-section-content">{children}</div>}
  </section>;
}

export function SourcesField({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const directories = parseSources(value);
  const customDirectories = directories.filter((directory) => !DEFAULT_SOURCES.includes(directory));

  return (
    <label className="repo-field">{labels.sources}
      <div className="directory-input">
        <div className="directory-chips">
          {DEFAULT_SOURCES.map((directory) => {
            const selected = directories.includes(directory);
            return (
              <button type="button" className={selected ? undefined : 'add-directory-chip'} key={directory} onClick={() => onChange(toggleSource(value, directory))}>
                {selected ? <X size={13} /> : <Plus size={13} />}
                {directory}
              </button>
            );
          })}
          {customDirectories.map((directory) => (
            <button type="button" key={directory} onClick={() => onChange(removeSource(value, directory))}>
              {directory}
              <X size={13} />
            </button>
          ))}
        </div>
        <input value={value} onChange={(event) => onChange(event.target.value)} placeholder="Add source" />
      </div>
    </label>
  );
}

export function BranchField({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const normalized = value.trim().toLowerCase();
  return (
    <label className="repo-field">Base Branch
      <div className="directory-input">
        <div className="directory-chips branch-chips">
          <button type="button" className={normalized === 'master' ? undefined : 'add-directory-chip'} onClick={() => onChange('master')}>
            {normalized === 'master' ? <X size={13} /> : <Plus size={13} />}
            master
          </button>
          <button type="button" className={normalized === 'main' ? undefined : 'add-directory-chip'} onClick={() => onChange('main')}>
            {normalized === 'main' ? <X size={13} /> : <Plus size={13} />}
            main
          </button>
        </div>
        <input value={value} onChange={(event) => onChange(event.target.value)} placeholder="Base branch" />
      </div>
    </label>
  );
}

export function KnowledgeSettingsFields({ value, onChange }: { value: KnowledgeSettings; onChange: (value: KnowledgeSettings) => void }) {
	const args = value.enrichArgs ?? [];
	const updateArg = (index: number, argument: string) => onChange({ ...value, enrichArgs: args.map((current, currentIndex) => currentIndex === index ? argument : current) });
	const moveArg = (index: number, offset: number) => { const target=index+offset; if(target<0||target>=args.length)return; const next=[...args]; [next[index],next[target]]=[next[target],next[index]]; onChange({...value,enrichArgs:next}); };
	return <section className="knowledge-settings-fields" aria-label="Knowledge settings"><label className="repo-field jira-connection-toggle"><span className="jira-connection-title"><input type="checkbox" checked={value.enabled !== false} onChange={(event) => onChange({ ...value, enabled: event.target.checked })} />Knowledge Wiki detection</span></label><label className="repo-field">Enrichment executable<input aria-label="Knowledge enrichment executable" value={value.enrichExecutable ?? ''} onChange={(event) => onChange({ ...value, enrichExecutable: event.target.value })} placeholder="wiki-enrich" /><small>This is a command name or path, not a secret.</small></label><div className="repo-field"><span>Literal arguments</span>{args.map((argument,index)=><div className="path-input-row" key={index}><input aria-label={`Knowledge enrichment argument ${index+1}`} value={argument} onChange={(event)=>updateArg(index,event.target.value)} /><button className="secondary icon-action" type="button" aria-label={`Move argument ${index+1} up`} disabled={index===0} onClick={()=>moveArg(index,-1)}>↑</button><button className="secondary icon-action" type="button" aria-label={`Move argument ${index+1} down`} disabled={index===args.length-1} onClick={()=>moveArg(index,1)}>↓</button><button className="secondary icon-action" type="button" aria-label={`Remove argument ${index+1}`} onClick={()=>onChange({...value,enrichArgs:args.filter((_,currentIndex)=>currentIndex!==index)})}>×</button></div>)}<button className="secondary" type="button" onClick={()=>onChange({...value,enrichArgs:[...args,'']})}>Add argument</button><small>Arguments are passed exactly as listed. Shell expansion and environment values are not supported.</small></div></section>;
}

export function RuntimeSettingsFields({ value, onChange }: { value: WorkspaceRuntimeConfig | null; onChange: (value: WorkspaceRuntimeConfig | null) => void }) {
  const runtime = value ?? defaultRuntimeConfig();
  const [activeRuntimeTab, setActiveRuntimeTab] = useState<'runtime' | 'verification'>('runtime');
  const [automationPickerBusy, setAutomationPickerBusy] = useState(false);
  const [automationPickerError, setAutomationPickerError] = useState('');
  const [verificationInputsEnabled, setVerificationInputsEnabled] = useState(true);
  const update = (patch: Partial<WorkspaceRuntimeConfig>) => onChange({ ...runtime, ...patch });
  const updateCommands = (patch: Partial<WorkspaceRuntimeConfig['commands']>) => onChange({ ...runtime, commands: { ...runtime.commands, ...patch } });
  const updateVerify = (patch: Partial<WorkspaceRuntimeConfig['commands']['verify']>) => onChange({ ...runtime, commands: { ...runtime.commands, verify: { ...runtime.commands.verify, ...patch } } });
  const healthChecks = runtime.healthChecks ?? [];
  const paths = runtime.artifacts?.paths ?? [];
  const automation = runtime.automation ?? defaultRuntimeConfig().automation!;
  const updateAutomation = (patch: Partial<NonNullable<WorkspaceRuntimeConfig['automation']>>) => update({ automation: { ...automation, ...patch } });
  const automationArtifactPaths = automation.artifactPaths ?? [];

  useEffect(() => {
    if (!value) {
      setVerificationInputsEnabled(true);
      return;
    }
    const verify = value.commands?.verify;
    const hasVerificationCommands = Boolean((verify?.smoke ?? '').trim() || (verify?.critical ?? '').trim() || (verify?.full ?? '').trim());
    setVerificationInputsEnabled(hasVerificationCommands);
  }, [value]);

  const browseAutomationRepository = async () => {
    setAutomationPickerBusy(true);
    setAutomationPickerError('');
    try {
      const selection = await api.selectDirectory();
      updateAutomation({ repositoryPath: selection.path });
    } catch (err) {
      setAutomationPickerError(errorMessage(err));
    } finally {
      setAutomationPickerBusy(false);
    }
  };

  return <section className="knowledge-settings-fields runtime-settings-fields" aria-label="Runtime settings">
    <div className="settings-subtabs" role="tablist" aria-label="Runtime and verification settings">
      <button type="button" role="tab" aria-selected={activeRuntimeTab === 'runtime'} className={activeRuntimeTab === 'runtime' ? 'active' : undefined} onClick={() => setActiveRuntimeTab('runtime')}>Runtime</button>
      <button type="button" role="tab" aria-selected={activeRuntimeTab === 'verification'} className={activeRuntimeTab === 'verification' ? 'active' : undefined} onClick={() => setActiveRuntimeTab('verification')}>Verification</button>
    </div>
    {activeRuntimeTab === 'runtime' && <>
      <label className="repo-field jira-connection-toggle">
        <span className="jira-connection-title">
          <input type="checkbox" checked={value !== null} onChange={(event) => onChange(event.target.checked ? runtime : null)} />
          Runtime
        </span>
      </label>
      {value && <>
        <div className="registration-mode-toggle" role="radiogroup" aria-label="Runtime type">
          {(['docker-compose', 'procfile', 'makefile', 'custom'] as RuntimeType[]).map((option) => (
            <button key={option} type="button" role="radio" aria-checked={runtime.type === option} className={runtime.type === option ? 'secondary active' : 'secondary'} onClick={() => update({ type: option })}>{option}</button>
          ))}
        </div>
        <label className="repo-field">Runtime config path<input value={runtime.configPath ?? ''} onChange={(event) => update({ configPath: event.target.value })} placeholder="infra/docker-compose.yaml" /></label>
        <label className="repo-field">Startup command<input value={runtime.commands.up} onChange={(event) => updateCommands({ up: event.target.value })} placeholder="docker compose up -d --no-build" /></label>
        <label className="repo-field">Teardown command<input value={runtime.commands.down} onChange={(event) => updateCommands({ down: event.target.value })} placeholder="docker compose down" /></label>
        <label className="repo-field">Changed-only rebuild command<input value={runtime.commands.rebuildChanged ?? ''} onChange={(event) => updateCommands({ rebuildChanged: event.target.value })} placeholder="docker compose build service-a service-b" /></label>
        <label className="repo-field">Rebuild policy<select value={runtime.rebuildPolicy ?? 'changed-only'} onChange={(event) => update({ rebuildPolicy: event.target.value as WorkspaceRuntimeConfig['rebuildPolicy'] })}><option value="never">Never</option><option value="changed-only">Changed only</option><option value="always">Always</option></select></label>
        <div className="repo-field"><span>Health checks</span>{healthChecks.map((check, index) => <div className="path-input-row" key={`${check.target}-${index}`}><select value={check.type} onChange={(event) => onChange({ ...runtime, healthChecks: healthChecks.map((current, currentIndex) => currentIndex === index ? { ...current, type: event.target.value as 'http' | 'command' } : current) })}><option value="http">HTTP</option><option value="command">Command</option></select><input value={check.target} onChange={(event) => onChange({ ...runtime, healthChecks: healthChecks.map((current, currentIndex) => currentIndex === index ? { ...current, target: event.target.value } : current) })} placeholder={check.type === 'http' ? 'http://localhost:3000/health' : 'curl -f http://localhost:3000/health'} /><button className="secondary icon-action" type="button" aria-label={`Remove health check ${index + 1}`} onClick={() => onChange({ ...runtime, healthChecks: healthChecks.filter((_, currentIndex) => currentIndex !== index) })}>×</button></div>)}<button className="secondary" type="button" onClick={() => onChange({ ...runtime, healthChecks: [...healthChecks, { type: 'http', target: '', timeoutSeconds: 30 }] })}>Add health check</button></div>
        <div className="repo-field"><span>Artifact paths</span>{paths.map((artifactPath, index) => <div className="path-input-row" key={`${artifactPath}-${index}`}><input value={artifactPath} onChange={(event) => onChange({ ...runtime, artifacts: { paths: paths.map((current, currentIndex) => currentIndex === index ? event.target.value : current) } })} placeholder="playwright-report" /><button className="secondary icon-action" type="button" aria-label={`Remove artifact path ${index + 1}`} onClick={() => onChange({ ...runtime, artifacts: { paths: paths.filter((_, currentIndex) => currentIndex !== index) } })}>×</button></div>)}<button className="secondary" type="button" onClick={() => onChange({ ...runtime, artifacts: { paths: [...paths, ''] } })}>Add artifact path</button></div>
      </>}
    </>}
    {activeRuntimeTab === 'verification' && <>
      <label className="repo-field jira-connection-toggle">
        <span className="jira-connection-title">
          <input type="checkbox" checked={verificationInputsEnabled} onChange={(event) => setVerificationInputsEnabled(event.target.checked)} />
          Runtime tests
        </span>
      </label>
      <fieldset className="runtime-settings-section-fields" disabled={!verificationInputsEnabled}>
        <div className="repo-field-grid">
          <label className="repo-field">Verify smoke<input value={runtime.commands.verify.smoke} onChange={(event) => updateVerify({ smoke: event.target.value })} placeholder="pnpm test:e2e --grep @smoke" /></label>
          <label className="repo-field">Verify critical<input value={runtime.commands.verify.critical ?? ''} onChange={(event) => updateVerify({ critical: event.target.value })} placeholder="pnpm test:e2e --grep @critical" /></label>
        </div>
        <label className="repo-field">Verify full<input value={runtime.commands.verify.full ?? ''} onChange={(event) => updateVerify({ full: event.target.value })} placeholder="pnpm test:e2e" /></label>
      </fieldset>
      <label className="repo-field jira-connection-toggle">
        <span className="jira-connection-title">
          <input type="checkbox" checked={automation.enabled} onChange={(event) => updateAutomation({ enabled: event.target.checked })} />
          Automation tests
        </span>
      </label>
      <fieldset className="runtime-settings-section-fields" disabled={!automation.enabled}>
        <div className="repo-field-grid automation-field-grid">
          <label className="repo-field automation-repository-field">Automation repository
            <div className="path-input-row automation-repository-row">
              <input value={automation.repositoryPath ?? ''} onChange={(event) => { setAutomationPickerError(''); updateAutomation({ repositoryPath: event.target.value }); }} placeholder="Repository path" />
              <button className="secondary" type="button" onClick={() => void browseAutomationRepository()} disabled={automationPickerBusy || !automation.enabled}><FolderOpen size={15} /> Browse</button>
            </div>
            {automationPickerError && <small className="settings-error">{automationPickerError}</small>}
          </label>
          <label className="repo-field">Automation runner<select value={automation.runner ?? 'cypress'} onChange={(event) => updateAutomation({ runner: event.target.value as NonNullable<WorkspaceRuntimeConfig['automation']>['runner'] })}><option value="cypress">Cypress</option><option value="playwright">Playwright</option></select></label>
        </div>
        <div className="repo-field-grid">
          <label className="repo-field">Default environment<input value={automation.defaultEnvironment ?? 'local'} onChange={(event) => updateAutomation({ defaultEnvironment: event.target.value })} placeholder="local" /></label>
          <label className="repo-field">Automation command template<input value={automation.commandTemplate ?? ''} onChange={(event) => updateAutomation({ commandTemplate: event.target.value })} placeholder={'CYPRESS_EPSAP_ENVIRONMENT={env} npx cypress run --spec {specs}'} /></label>
        </div>
        <div className="repo-field"><span>Automation artifact paths</span>{automationArtifactPaths.map((artifactPath, index) => <div className="path-input-row" key={`automation-${artifactPath}-${index}`}><input value={artifactPath} onChange={(event) => updateAutomation({ artifactPaths: automationArtifactPaths.map((current, currentIndex) => currentIndex === index ? event.target.value : current) })} placeholder="cypress/videos" /><button className="secondary icon-action" type="button" aria-label={`Remove automation artifact path ${index + 1}`} onClick={() => updateAutomation({ artifactPaths: automationArtifactPaths.filter((_, currentIndex) => currentIndex !== index) })}>×</button></div>)}<button className="secondary" type="button" onClick={() => updateAutomation({ artifactPaths: [...automationArtifactPaths, ''] })}>Add automation artifact path</button></div>
      </fieldset>
    </>}
  </section>;
}

export function KnowledgeRoots({ workspaceId, enabled }: { workspaceId: string; enabled: boolean }) {
	const [roots, setRoots] = useState<string[]>([]);
	useEffect(() => {
		const controller = new AbortController();
		if (!enabled) { setRoots([]); return; }
		const loadWikis = api.knowledgeWikis;
		if (typeof loadWikis !== 'function') { setRoots([]); return; }
		void loadWikis(workspaceId, controller.signal).then((wikis) => { if (!controller.signal.aborted) setRoots(wikis.map((wiki) => wiki.root)); }).catch(() => { if (!controller.signal.aborted) setRoots([]); });
		return () => controller.abort();
	}, [enabled, workspaceId]);
	return <small>{enabled ? roots.length ? `Detected: ${roots.join(', ')}` : 'No compatible Wiki roots detected yet.' : 'Detection is disabled.'}</small>;
}

export function JiraConnectionFields({ value, onChange, workspaceId }: { value: JiraConnection | null; onChange: (value: JiraConnection | null) => void; workspaceId?: string }) {
  const [testing, setTesting] = useState(false);
  const [resultTone, setResultTone] = useState<'success' | 'error' | null>(null);
  const [result, setResult] = useState('');
  const update = (patch: Partial<JiraConnection>) => value && onChange({ ...value, ...patch });
  const test = async () => {
    if (!value || !workspaceId) return;
    setTesting(true); setResultTone(null); setResult('');
    try {
      const response = await api.testJiraConnection(workspaceId, normalizeJiraDraft(value));
      setResultTone('success');
      setResult(response.message);
    } catch (caught) {
      setResultTone('error');
      setResult(caught instanceof ApiError && caught.recoveryHint ? `${caught.message} ${caught.recoveryHint}` : errorMessage(caught));
    } finally {
      setTesting(false);
    }
  };
  return <section className="jira-connection-fields">
    <label className="repo-field jira-connection-toggle">
      <span className="jira-connection-title">
        <input type="checkbox" checked={value !== null} onChange={(event) => { setResultTone(null); setResult(''); onChange(event.target.checked ? emptyJiraConnection() : null); }} />
        Jira integration
      </span>
    </label>
    {value && <>
      <div className="registration-mode-toggle" role="radiogroup" aria-label="Jira deployment type"><button type="button" role="radio" aria-checked={value.deploymentType === 'cloud'} className={value.deploymentType === 'cloud' ? 'secondary active' : 'secondary'} onClick={() => update({ deploymentType: 'cloud' })}>Cloud</button><button type="button" role="radio" aria-checked={value.deploymentType === 'server'} className={value.deploymentType === 'server' ? 'secondary active' : 'secondary'} onClick={() => update({ deploymentType: 'server', accountEmail: '' })}>Server / Data Center</button></div>
      <div className="repo-field-grid"><label className="repo-field">Base URL<input aria-label="Jira base URL" value={value.baseUrl} onChange={(event) => update({ baseUrl: event.target.value })} placeholder="https://company.atlassian.net" /></label><label className="repo-field">Project Key<input aria-label="Jira project key" value={value.projectKey} onChange={(event) => update({ projectKey: event.target.value.toUpperCase() })} placeholder="DI" /></label></div>
      {value.deploymentType === 'cloud' && <label className="repo-field">Account Email<input aria-label="Jira account email" value={value.accountEmail ?? ''} onChange={(event) => update({ accountEmail: event.target.value })} /></label>}
      <label className="repo-field">Token Environment Variable<input aria-label="Jira token environment variable" value={value.tokenEnvVar} onChange={(event) => update({ tokenEnvVar: event.target.value })} /><small>Store the token in this environment variable before starting Kode Stream.</small></label>
      {workspaceId && <div className="jira-test-row"><button className="secondary" type="button" disabled={testing} onClick={() => void test()}>{testing ? 'Testing...' : 'Test Jira connection'}</button>{result && <span className={`jira-connection-status ${resultTone ?? 'success'}`} role="status"><span className={`jira-connection-status-dot ${resultTone ?? 'success'}`} aria-hidden="true" />{result}</span>}</div>}
    </>}
  </section>;
}
