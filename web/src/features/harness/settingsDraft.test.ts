import { describe, expect, it } from 'vitest';
import type { HarnessSettings } from '../../lib/types';
import { effectiveRoleModel, emptyWorkspaceSettings, ownSettings, withOwnSettings, withRoleSkill } from './settingsDraft';

const draft = {
	workspaces: [{ ...emptyWorkspaceSettings('a'), models: { coder: { engine: 'codex', model: 'gpt' } } }],
	modelPolicy: { coder: { engine: 'claude', model: 'sonnet', hardModel: 'opus', maxTurns: 40 } }
} as unknown as HarnessSettings;

describe('harness settings draft', () => {
	it('finds a workspace’s own settings, else empty ones', () => {
		expect(ownSettings(draft, 'a').models.coder?.engine).toBe('codex');
		expect(ownSettings(draft, 'b')).toEqual(emptyWorkspaceSettings('b'));
	});

	it('changes a listed workspace in place and adds an unlisted one', () => {
		expect(withOwnSettings(draft, ownSettings(draft, 'a'), { scope: ['src'] }).map((item) => [item.workspaceId, item.scope])).toEqual([['a', ['src']]]);
		expect(withOwnSettings(draft, ownSettings(draft, 'b'), { scope: ['x'] }).map((item) => item.workspaceId)).toEqual(['a', 'b']);
	});

	it('fills a role model from the defaults where the workspace chose nothing', () => {
		expect(effectiveRoleModel(draft, ownSettings(draft, 'a'), 'coder' as never)).toEqual({ engine: 'codex', model: 'gpt', highDifficultyModel: '', maxTurns: 40 });
		expect(effectiveRoleModel(draft, ownSettings(draft, 'b'), 'coder' as never)).toEqual({ engine: 'claude', model: 'sonnet', highDifficultyModel: 'opus', maxTurns: 40 });
	});

	it('sets a role skill and track, and drops a role left with neither', () => {
		const set = withRoleSkill({}, 'coder' as never, { skill: 'tdd', track: 'api' });
		expect(set).toEqual({ coder: { skill: 'tdd', track: 'api' } });
		expect(withRoleSkill(set, 'coder' as never, { skill: '', track: '' })).toEqual({});
	});
});
