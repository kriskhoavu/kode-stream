import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HarnessSettingsSection } from './HarnessSettingsSection';
import { status } from '../harness/testData';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

describe('HarnessSettingsSection', () => {
	it('shows the harness settings under Settings, with the roles to pause', async () => {
		vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
			const url = String(input);
			if (url === '/api/harness/status') return Promise.resolve({ ok: true, json: async () => status({ roles: [{ role: 'coder', paused: false, working: 0, slots: 1, queued: 0 }] }) });
			if (url === '/api/harness/settings') return Promise.resolve({ ok: true, json: async () => ({ revision: 1, enabled: true, autoStart: false, requireReview: true, askQuestions: true, slots: { scout: 1, sorter: 1, planner: 1, coder: 1, reviewer: 1, qa: 1 }, paused: {}, sources: [], modelPolicy: { planner: { engine: 'claudecode', model: 'opus', maxTurns: 1 }, plan_reviewer: { engine: 'claudecode', model: 'opus', maxTurns: 1 }, coder: { engine: 'claudecode', model: 'opus', maxTurns: 1 }, code_reviewer: { engine: 'claudecode', model: 'opus', maxTurns: 1 }, qa_fixer: { engine: 'claudecode', model: 'opus', maxTurns: 1 } } }) });
			if (url === '/api/workspaces') return Promise.resolve({ ok: true, json: async () => [] });
			return Promise.resolve({ ok: true, json: async () => ({}) });
		}));
		render(<HarnessSettingsSection />);
		expect(await screen.findByRole('heading', { name: 'The harness' })).toBeInTheDocument();
		expect(await screen.findByRole('heading', { name: 'Where work comes from' })).toBeInTheDocument();
	});
});
