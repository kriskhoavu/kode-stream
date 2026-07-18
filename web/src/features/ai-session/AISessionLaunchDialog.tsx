import type { Dispatch, ReactNode, SetStateAction } from 'react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Bot, ChevronDown, ChevronRight, FolderTree, Globe2, Search, Ticket, X } from 'lucide-react';
import { api } from '../../lib/api';
import type {
	AICapability,
	AICapabilityDescriptor,
	AIPlanPreset,
	AIProviderCapabilityCatalog,
	AISessionEligibility,
	AISettings,
	AISessionLaunchInput,
	AISessionLaunchResult,
	EmbeddedAISessionResult
} from '../../lib/types';
import { appendJiraDescriptionPrompt, removeJiraDescriptionPrompt } from './jiraPrompt';

export function AISessionLaunchDialog({ itemId, preference, allowEmbedded = true, onClose, onLaunched }: { itemId: string; preference?: AISessionLaunchInput | null; allowEmbedded?: boolean; onClose: () => void; onLaunched: (result: AISessionLaunchResult | EmbeddedAISessionResult, input: AISessionLaunchInput) => void }) {
	const [settings, setSettings] = useState<AISettings | null>(null);
	const [capabilities, setCapabilities] = useState<AICapability[]>([]);
	const [presets, setPresets] = useState<AIPlanPreset[]>([]);
	const [eligibility, setEligibility] = useState<AISessionEligibility | null>(null);
	const [providerCatalog, setProviderCatalog] = useState<AIProviderCapabilityCatalog | null>(null);
	const [providerCatalogError, setProviderCatalogError] = useState('');
	const [provider, setProvider] = useState('');
	const [terminal, setTerminal] = useState('');
	const [contextMode, setContextMode] = useState<AISessionLaunchInput['contextMode']>('card_context');
	const [presetId, setPresetId] = useState('');
	const [promptDraft, setPromptDraft] = useState('');
	const [promptDirty, setPromptDirty] = useState(false);
	const [includeJiraDescription, setIncludeJiraDescription] = useState(false);
	const [jiraPromptLoading, setJiraPromptLoading] = useState(false);
	const [jiraPromptError, setJiraPromptError] = useState('');
	const [selectedSkills, setSelectedSkills] = useState<string[]>([]);
	const [selectedAgents, setSelectedAgents] = useState<string[]>([]);
	const [surface, setSurface] = useState<'external' | 'embedded'>('external');
	const [loading, setLoading] = useState(true);
	const [launching, setLaunching] = useState(false);
	const [error, setError] = useState('');
	const closeRef = useRef<HTMLButtonElement | null>(null);
	const dialogRef = useRef<HTMLElement | null>(null);

	const providers = toolOptions(settings?.providers, capabilities, 'provider');
	const terminals = toolOptions(settings?.terminals, capabilities, 'terminal');
	const canLaunch = !loading && !launching && (contextMode === 'workspace_only' || eligibility?.cardContextAvailable) && providers.some((item) => item.id === provider) && (surface === 'embedded' || terminals.some((item) => item.id === terminal));

	useEffect(() => {
		let active = true;
		Promise.all([api.aiSettings(), api.aiCapabilities(), api.aiPresets(), api.aiSessionEligibility(itemId)]).then(([nextSettings, nextCapabilities, nextPresets, nextEligibility]) => {
			if (!active) return;
			const nextProvider = preference?.provider ?? nextSettings.defaultProvider;
			const nextPresetId = preference?.presetId ?? nextPresets[0]?.id ?? '';
			const presetPrompt = nextPresets.find((preset) => preset.id === nextPresetId)?.prompt ?? '';
			const nextPromptDraft = preference?.promptDraft ?? preference?.customPrompt ?? presetPrompt;
			setSettings(nextSettings);
			setCapabilities(nextCapabilities);
			setPresets(nextPresets);
			setEligibility(nextEligibility);
			setProvider(nextProvider);
			setTerminal(preference?.terminal ?? nextSettings.defaultTerminal);
			setContextMode(preference?.contextMode ?? 'card_context');
			setPresetId(nextPresetId);
			setPromptDraft(nextPromptDraft);
			setPromptDirty(Boolean(preference?.promptDraft ?? preference?.customPrompt) && nextPromptDraft.trim() !== presetPrompt.trim());
			setIncludeJiraDescription(preference?.includeJiraDescription === true);
			setJiraPromptError('');
			setSelectedSkills([]);
			setSelectedAgents([]);
			setSurface(allowEmbedded ? preference?.surface ?? 'external' : 'external');
		}).catch((caught) => active && setError(caught instanceof Error ? caught.message : 'AI session options are unavailable.')).finally(() => active && setLoading(false));
		return () => { active = false; };
	}, [allowEmbedded, itemId, preference]);

	useEffect(() => {
		if (!provider) return;
		let active = true;
		setProviderCatalog(null);
		setProviderCatalogError('');
		api.aiProviderCapabilities(provider, itemId).then((catalog) => {
			if (!active) return;
			setProviderCatalog(catalog);
			setSelectedSkills((current) => current.filter((item) => catalog.skills.some((skill) => skill.id === item)));
			setSelectedAgents((current) => current.filter((item) => catalog.agents.some((agent) => agent.id === item)));
		}).catch((caught) => {
			if (!active) return;
			setProviderCatalog({ provider, skills: [], agents: [], supportsNativeSelection: false, supportsPromptFallback: false });
			setProviderCatalogError(caught instanceof Error ? caught.message : 'Provider capabilities are unavailable.');
			setSelectedSkills([]);
			setSelectedAgents([]);
		});
		return () => { active = false; };
	}, [provider, itemId]);

	useEffect(() => {
		if (!includeJiraDescription) {
			setPromptDraft((current) => removeJiraDescriptionPrompt(current));
			setJiraPromptError('');
			setJiraPromptLoading(false);
			return;
		}
		let active = true;
		setJiraPromptLoading(true);
		setJiraPromptError('');
		api.jiraIssue(itemId).then((state) => {
			if (!active) return;
			const issue = state.issue;
			if (state.state !== 'available' || !issue) {
				setJiraPromptError(state.message || 'Jira ticket description is unavailable for this item.');
				return;
			}
			if (!issue.description.trim()) {
				setJiraPromptError('The Jira ticket does not have a description.');
				return;
			}
			setPromptDraft((current) => appendJiraDescriptionPrompt(current, issue));
			setPromptDirty(true);
		}).catch((caught) => {
			if (!active) return;
			setJiraPromptError(caught instanceof Error ? caught.message : 'Jira ticket description is unavailable.');
		}).finally(() => active && setJiraPromptLoading(false));
		return () => { active = false; };
	}, [includeJiraDescription, itemId, presetId]);

	useEffect(() => {
		closeRef.current?.focus();
		const onKeyDown = (event: KeyboardEvent) => {
			if (event.key === 'Escape' && !launching) onClose();
			if (event.key !== 'Tab' || !dialogRef.current) return;
			const controls = Array.from(dialogRef.current.querySelectorAll<HTMLElement>('button:not([disabled]), select:not([disabled]), input:not([disabled]), textarea:not([disabled])'));
			if (controls.length === 0) return;
			const first = controls[0];
			const last = controls[controls.length - 1];
			if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
			if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
		};
		window.addEventListener('keydown', onKeyDown);
		return () => window.removeEventListener('keydown', onKeyDown);
	}, [launching, onClose]);

	const handlePresetChange = (nextPresetId: string) => {
		const nextPreset = presets.find((preset) => preset.id === nextPresetId);
		setPresetId(nextPresetId);
		if (!promptDirty || promptDraft.trim() === '' || !presetId) {
			setPromptDraft(nextPreset?.prompt ?? '');
			setPromptDirty(false);
		}
	};

	const toggleSelection = (setCurrent: Dispatch<SetStateAction<string[]>>, id: string) => {
		setCurrent((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
	};

	const selectCapabilities = (setCurrent: Dispatch<SetStateAction<string[]>>, ids: string[]) => {
		setCurrent((current) => Array.from(new Set([...current, ...ids])));
	};

	const clearCapabilities = (setCurrent: Dispatch<SetStateAction<string[]>>, ids: string[]) => {
		const idSet = new Set(ids);
		setCurrent((current) => current.filter((item) => !idSet.has(item)));
	};

	const launch = async () => {
		setLaunching(true);
		setError('');
		try {
			const input: AISessionLaunchInput = {
				provider,
				terminal,
				contextMode,
				surface,
				presetId: presetId || undefined,
				promptDraft: promptDraft.trim() || undefined,
				selectedSkills: selectedSkills.length > 0 ? selectedSkills : undefined,
				selectedAgents: selectedAgents.length > 0 ? selectedAgents : undefined,
				includeJiraDescription: includeJiraDescription || undefined
			};
			const result = surface === 'embedded'
				? await api.startEmbeddedAISession(itemId, { provider, contextMode, presetId: input.presetId, promptDraft: input.promptDraft, selectedSkills: input.selectedSkills, selectedAgents: input.selectedAgents, columns: 80, rows: 24 })
				: await api.launchAISession(itemId, input);
			onLaunched(result, input);
			onClose();
		} catch (caught) {
			setError(caught instanceof Error ? caught.message : 'AI session launch failed.');
		} finally {
			setLaunching(false);
		}
	};

	return (
		<div className="modal-backdrop ai-launch-backdrop" role="presentation">
			<section ref={dialogRef} className="modal-panel ai-launch-dialog" role="dialog" aria-modal="true" aria-labelledby="ai-launch-title">
				<header><div><h2 id="ai-launch-title"><Bot size={19} /> Open AI session</h2><span>Start an interactive CLI with this workspace and item context.</span></div><button ref={closeRef} className="icon-button" type="button" aria-label="Close AI session dialog" disabled={launching} onClick={onClose}><X size={18} /></button></header>
				{loading && <p role="status">Loading available tools...</p>}
				{error && <p className="error" role="alert">{error}</p>}
				{settings && eligibility && <div className="ai-launch-fields">
					<label>AI provider<select value={provider} onChange={(event) => setProvider(event.target.value)}>{providers.map((item) => <option key={item.id} value={item.id}>{label(item.id)}</option>)}</select></label>
					<fieldset><legend>Session surface</legend><label><input type="radio" name="ai-surface" checked={surface === 'external'} onChange={() => setSurface('external')} /> Integrated terminal</label>{allowEmbedded && <label><input type="radio" name="ai-surface" checked={surface === 'embedded'} onChange={() => setSurface('embedded')} /> Embedded terminal</label>}</fieldset>
					{surface === 'external' && <label>Terminal<select value={terminal} onChange={(event) => setTerminal(event.target.value)}>{terminals.map((item) => <option key={item.id} value={item.id}>{label(item.id)}</option>)}</select></label>}
					<fieldset><legend>Session context</legend><label><input type="radio" name="ai-context" checked={contextMode === 'workspace_only'} onChange={() => setContextMode('workspace_only')} /> Workspace only — start with a free prompt</label><label><input type="radio" name="ai-context" checked={contextMode === 'card_context'} disabled={!eligibility.cardContextAvailable} aria-describedby="card-context-readiness" onChange={() => setContextMode('card_context')} /> Selected card — provide its path and related documents</label></fieldset>
					{contextMode === 'workspace_only' && <p className="eligibility-ready">No card context will be injected. The AI opens at the workspace root so you can manually reference any relevant file or directory.</p>}
					{contextMode === 'card_context' && <p id="card-context-readiness" className={eligibility.cardContextAvailable ? 'eligibility-ready' : 'eligibility-blocked'}>{eligibility.cardContextAvailable ? 'The selected card path will be provided as context. The AI will read relevant documents from that path and wait for your request.' : `Card context unavailable: ${eligibility.missing.join(', ') || 'the card is not available in the working tree'}.`}</p>}
					<label>Prompt preset<select aria-label="AI prompt" value={presetId} onChange={(event) => handlePresetChange(event.target.value)}>
						{presets.map((preset) => <option key={preset.id} value={preset.id}>{preset.name}</option>)}
						<option value="">Free prompt</option>
					</select></label>
					<div className="ai-launch-prompt-field">
						<div className="ai-launch-prompt-heading"><span>Prompt</span><label className="ai-launch-check"><input type="checkbox" checked={includeJiraDescription} onChange={(event) => setIncludeJiraDescription(event.target.checked)} /> <span><Ticket size={14} /> Fetch Jira ticket description into prompt</span></label></div>
						<textarea aria-label="Prompt" rows={4} value={promptDraft} onChange={(event) => { setPromptDraft(event.target.value); setPromptDirty(true); }} placeholder="Tell the AI what to do with this item." />
					</div>
					{jiraPromptLoading && <p className="eligibility-ready" role="status">Fetching Jira ticket description...</p>}
					{jiraPromptError && <p className="eligibility-blocked">{jiraPromptError}</p>}
					{providerCatalogError && <p className="eligibility-blocked">{providerCatalogError}</p>}
					<CapabilitySection title="Skills" items={providerCatalog?.skills ?? []} selected={selectedSkills} onToggle={(id) => toggleSelection(setSelectedSkills, id)} onSelect={(ids) => selectCapabilities(setSelectedSkills, ids)} onClear={(ids) => clearCapabilities(setSelectedSkills, ids)} />
					<CapabilitySection title="Agents" items={providerCatalog?.agents ?? []} selected={selectedAgents} onToggle={(id) => toggleSelection(setSelectedAgents, id)} onSelect={(ids) => selectCapabilities(setSelectedAgents, ids)} onClear={(ids) => clearCapabilities(setSelectedAgents, ids)} />
					{providerCatalog && providerCatalog.skills.length === 0 && providerCatalog.agents.length === 0 && <p className="eligibility-ready">No {label(provider)} workspace or global skills/agents were discovered for this item. Launch will continue without capability injection.</p>}
					{!eligibility.editable && contextMode !== 'workspace_only' && <p className="error">Card context requires an editable working-tree item.</p>}
					{(providers.length === 0 || (surface === 'external' && terminals.length === 0)) && <p className="error">Enable and detect the tools required for this session in Settings.</p>}
				</div>}
				<footer className="modal-actions"><button className="ghost" type="button" disabled={launching} onClick={onClose}>Cancel</button><button className="primary" type="button" disabled={!canLaunch} onClick={() => void launch()}>{launching ? 'Opening...' : 'Open session'}</button></footer>
			</section>
		</div>
	);
}

function CapabilitySection({ title, items, selected, onToggle, onSelect, onClear }: { title: string; items: AICapabilityDescriptor[]; selected: string[]; onToggle: (id: string) => void; onSelect: (ids: string[]) => void; onClear: (ids: string[]) => void }) {
	const [query, setQuery] = useState('');
	const [collapsed, setCollapsed] = useState(true);
	const [activeScope, setActiveScope] = useState<'workspace' | 'global'>('workspace');
	const normalizedQuery = query.trim().toLowerCase();
	const filteredItems = useMemo(() => items.filter((item) => {
		if (normalizedQuery === '') return true;
		const haystack = `${item.name} ${item.sourcePath} ${item.scope} ${item.provider}`.toLowerCase();
		return haystack.includes(normalizedQuery);
	}), [items, normalizedQuery]);
	const selectedVisible = filteredItems.filter((item) => selected.includes(item.id)).length;
	if (items.length === 0) return null;
	const workspaceItems = filteredItems.filter((item) => item.scope === 'workspace');
	const globalItems = filteredItems.filter((item) => item.scope !== 'workspace');
	const selectedItems = items.filter((item) => selected.includes(item.id));
	const scopeTabs: Array<{ id: 'workspace' | 'global'; title: string; icon: ReactNode; count: number; items: AICapabilityDescriptor[] }> = [
		{ id: 'workspace', title: 'Workspace', icon: <FolderTree size={14} />, count: workspaceItems.length, items: workspaceItems },
		{ id: 'global', title: 'Global', icon: <Globe2 size={14} />, count: globalItems.length, items: globalItems }
	];
	const currentScope = scopeTabs.find((tab) => tab.id === activeScope) ?? scopeTabs[0];
	return <fieldset className={`ai-launch-capabilities${collapsed ? ' collapsed' : ''}`}><legend>{title}</legend>
		<div className="ai-launch-capability-heading">
			<button className="ai-launch-capability-toggle" type="button" aria-expanded={!collapsed} aria-controls={`ai-launch-capabilities-${title.toLowerCase()}`} onClick={() => setCollapsed((current) => !current)}>
				<span>{collapsed ? <ChevronRight size={16} /> : <ChevronDown size={16} />}{title}</span>
			</button>
			{selectedItems.length > 0 && <div className="ai-launch-selected-badges" aria-label={`Selected ${title.toLowerCase()}`}>{selectedItems.map((item) => <button key={item.id} className="ai-launch-selected-badge" type="button" title={`Remove ${item.name}`} aria-label={`Remove ${item.name}`} onClick={() => onToggle(item.id)}>{item.name}<X size={12} /></button>)}</div>}
		</div>
		{!collapsed && <div id={`ai-launch-capabilities-${title.toLowerCase()}`} className="ai-launch-capability-body">
			<div className="ai-launch-capability-tabs" role="tablist" aria-label={`${title} source scope`}>
				{scopeTabs.map((tab) => <button key={tab.id} type="button" role="tab" aria-selected={activeScope === tab.id} className={activeScope === tab.id ? 'active' : ''} onClick={() => setActiveScope(tab.id)}><span>{tab.icon}{tab.title}</span><small>{tab.count}</small></button>)}
			</div>
			<CapabilityScopeGroup title={currentScope.title} capabilityTitle={title} items={currentScope.items} selected={selected} onToggle={onToggle} onSelect={onSelect} onClear={onClear} query={query} onQueryChange={setQuery} filterStatus={normalizedQuery !== '' ? `${selectedVisible} of ${filteredItems.length} matching ${title.toLowerCase()} selected` : ''} />
		</div>}
	</fieldset>;
}

function CapabilityScopeGroup({ title, capabilityTitle, items, selected, onToggle, onSelect, onClear, query, onQueryChange, filterStatus }: { title: string; capabilityTitle: string; items: AICapabilityDescriptor[]; selected: string[]; onToggle: (id: string) => void; onSelect: (ids: string[]) => void; onClear: (ids: string[]) => void; query: string; onQueryChange: (value: string) => void; filterStatus: string }) {
	const [expanded, setExpanded] = useState(false);
	const defaultVisibleCount = 6;
	const visibleItems = expanded ? items : items.slice(0, defaultVisibleCount);
	const hiddenCount = Math.max(0, items.length - visibleItems.length);
	const hasOverflow = items.length > defaultVisibleCount;
	const selectedIds = items.filter((item) => selected.includes(item.id)).map((item) => item.id);
	const allSelected = items.length > 0 && selectedIds.length === items.length;
	const someSelected = selectedIds.length > 0 && !allSelected;
	if (items.length === 0) return <section className="ai-launch-capability-group ai-launch-capability-group-empty"><div className="ai-launch-capability-actions"><label className="ai-launch-capability-search"><Search size={14} /><input type="search" value={query} onChange={(event) => onQueryChange(event.target.value)} placeholder={`Filter ${capabilityTitle.toLowerCase()} by name or path`} aria-label={`Filter ${capabilityTitle.toLowerCase()}`} /></label><button className="ghost" type="button" disabled>Select all</button><button className="ghost" type="button" disabled>Clear</button></div>{filterStatus && <p className="ai-launch-capability-filter-status">{filterStatus}</p>}<p>No {title.toLowerCase()} entries found.</p></section>;
	return <section className="ai-launch-capability-group"><div className="ai-launch-capability-actions"><label className="ai-launch-capability-search"><Search size={14} /><input type="search" value={query} onChange={(event) => onQueryChange(event.target.value)} placeholder={`Filter ${capabilityTitle.toLowerCase()} by name or path`} aria-label={`Filter ${capabilityTitle.toLowerCase()}`} /></label><button className="ghost" type="button" disabled={allSelected} onClick={() => onSelect(items.map((item) => item.id))}>Select all</button><button className="ghost" type="button" disabled={!someSelected && !allSelected} onClick={() => onClear(selectedIds)}>Clear</button></div>{filterStatus && <p className="ai-launch-capability-filter-status">{filterStatus}</p>}<div className="ai-launch-capability-list">{visibleItems.map((item) => {
		const isSelected = selected.includes(item.id);
		return <label key={item.id} className={`ai-launch-capability-item${isSelected ? ' selected' : ''}`}><input type="checkbox" aria-label={item.name} checked={isSelected} onChange={() => onToggle(item.id)} /><span><span className="ai-launch-capability-topline"><strong title={item.name}>{item.name}</strong></span><code title={item.sourcePath}>{item.sourcePath}</code></span></label>;
	})}</div>{hasOverflow && <button className="ai-launch-capability-expand" type="button" onClick={() => setExpanded((current) => !current)}>{expanded ? 'Show less' : `Show ${hiddenCount} more`}</button>}</section>;
}

function toolOptions(templates: Record<string, { enabled: boolean }> | undefined, capabilities: AICapability[], kind: 'provider' | 'terminal') {
	return Object.keys(templates ?? {}).filter((id) => templates?.[id].enabled && capabilities.some((item) => item.kind === kind && item.id === id && item.detected)).map((id) => ({ id }));
}

function label(id: string) {
	return ({ claude: 'Claude', codex: 'Codex', copilot: 'Copilot', opencode: 'OpenCode', terminal: 'Terminal', iterm2: 'iTerm2', wezterm: 'WezTerm' } as Record<string, string>)[id] ?? id;
}
