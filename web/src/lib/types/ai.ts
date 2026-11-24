import { type SafeSessionRecord } from './canvas';

export type AICapabilityKind = 'provider' | 'terminal';

export interface AICapability {
  id: string;
  kind: AICapabilityKind;
  detected: boolean;
  configured: boolean;
  executable: string;
  reason?: string;
}

export interface AICapabilityDescriptor {
  id: string;
  name: string;
  description?: string;
  kind: 'skill' | 'agent' | string;
  provider: string;
  scope: 'workspace' | 'global' | string;
  sourcePath: string;
}

export interface AIProviderCapabilityCatalog {
  provider: string;
  skills: AICapabilityDescriptor[];
  agents: AICapabilityDescriptor[];
  supportsNativeSelection: boolean;
  supportsPromptFallback: boolean;
}

export interface AILaunchTemplate {
  enabled: boolean;
  executable: string;
  args: string[];
}

export interface AISettings {
  defaultProvider: string;
  defaultTerminal: string;
  providers: Record<string, AILaunchTemplate>;
  terminals: Record<string, AILaunchTemplate>;
  workspaceProviders?: Record<string, string>;
}

export const workspaceDefaultProvider = 'claude';

export function providerFor(settings: Pick<AISettings, 'defaultProvider' | 'workspaceProviders'> & { providers?: AISettings['providers'] }, workspaceId?: string): string {
  if (!workspaceId) return settings.defaultProvider;
  return settings.workspaceProviders?.[workspaceId] || (settings.providers?.[workspaceDefaultProvider]?.enabled ? workspaceDefaultProvider : settings.defaultProvider);
}

export interface AIPlanPreset {
  id: string;
  name: string;
  prompt: string;
  contextMode: AISessionLaunchInput['contextMode'];
  provider?: string;
}

export interface AISessionEligibility {
  editable: boolean;
  cardContextAvailable: boolean;
  missing: string[];
}

export interface AISessionLaunchInput {
  provider: string;
  terminal: string;
  contextMode: 'workspace_only' | 'card_context';
	presetId?: string;
	promptDraft?: string;
	customPrompt?: string;
	selectedSkills?: string[];
	selectedAgents?: string[];
	surface?: 'external' | 'embedded';
	includeJiraDescription?: boolean;
}

export interface AISessionLaunchResult extends AISessionLaunchInput {
  accepted: boolean;
  startedAt: string;
}

export type EmbeddedAISessionState = 'starting' | 'running' | 'exited' | 'cancelled' | 'failed';

export interface EmbeddedAISession {
	id: string;
	itemId: string;
	itemIdentifier?: string;
	itemTitle?: string;
	workspaceId: string;
	provider: string;
	intent: AISessionLaunchInput['contextMode'];
	state: EmbeddedAISessionState;
	startedAt: string;
	exitCode?: number;
}

export interface EmbeddedAISessionResult {
	session: EmbeddedAISession;
	grant: { sessionId: string; token: string; expiresAt: string };
	record?: SafeSessionRecord;
}

export interface EmbeddedAISessionLaunchInput extends Pick<AISessionLaunchInput, 'provider' | 'contextMode' | 'presetId' | 'promptDraft' | 'customPrompt' | 'selectedSkills' | 'selectedAgents'> {
	contextPath?: string;
	expectedWorkspaceId?: string;
	expectedBranch?: string;
	observedCommit?: string;
	idempotencyKey?: string;
	resumeClaudeSessionId?: string;
	columns?: number;
	rows?: number;
}
