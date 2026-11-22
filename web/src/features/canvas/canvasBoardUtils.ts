import type { CanvasNode as DomainNode } from '../../lib/types';

export const focusPreferenceKey = 'canvas-focus';

/** Session nodes counted as active by Focus mode: launched and still live. */
export function isFocusActiveSession(node: DomainNode): boolean {
	const record = node.session?.record;
	return Boolean(record?.live) && (record?.state === 'starting' || record?.state === 'running');
}

/* Virtual focus layout constants. Plans stack in a left column at x = 0; each
 * plan's sessions sit in a row to its right, expanded so the terminal shows.
 * The session pitch is sized off the same node dimensions used elsewhere on
 * the board (245x116 collapsed, 660x500 expanded); row height is derived per
 * row from the tallest node it contains (see focusNodeSize). */
export const FOCUS_GUTTER = 40;
export const FOCUS_PLAN_WIDTH = 245;
export const DEFAULT_SESSION_SIZE = { width: 660, height: 500 };

export function focusNodeSize(node: DomainNode, focusOn: boolean): { width: number; height: number } {
	const expanded = Boolean(node.session) && (focusOn || !node.collapsed);
	return expanded ? node.size ?? DEFAULT_SESSION_SIZE : { width: 245, height: 116 };
}

export function canMove(node: DomainNode, workspaceLayoutAvailable = false) {
	const actions = node.workspace?.actions ?? node.plan?.actions;
	return actions?.['layout.move']?.state === 'available' || (!actions && workspaceLayoutAvailable);
}

export function nodeSearchText(node: DomainNode) {
	if (node.workspace) return `${node.workspace.name ?? ''} ${node.workspace.branch ?? ''}`.trim();
	if (node.plan) return `${node.plan.identifier ?? ''} ${node.plan.title ?? ''} ${node.plan.branch ?? ''}`.trim();
	if (node.session) return `${node.session.record.provider} ${node.session.record.requestedBranch} ${node.session.record.state}`;
	return `${node.kind} ${node.entityRef.identifier ?? node.entityRef.sessionId ?? ''}`.trim();
}

export function formatFacet(value: string) { return value.replaceAll('_', ' ').replace(/^./, (character) => character.toUpperCase()); }
export function toggleValue(values: string[], value: string) { return values.includes(value) ? values.filter((candidate) => candidate !== value) : [...values, value]; }
