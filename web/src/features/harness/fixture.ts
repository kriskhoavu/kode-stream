import type {
	HarnessAgentRole,
	HarnessRunPlan,
	HarnessAgentRun,
	HarnessApproval,
	HarnessApprovalGate,
	HarnessCostSummary,
	HarnessEvent,
	HarnessFinding,
	HarnessRoleHealth,
	HarnessRun,
	HarnessRunDetail,
	HarnessSettings,
	HarnessStage,
	HarnessStageKind,
	HarnessStatus,
	HarnessTask, ClaudeTurn,
	SafeSessionRecord,
	SessionChanges
} from '../../lib/types';
import type { HarnessClient, HarnessFeedHandlers } from './harnessClient';

// Loaded only in development (see `loadHarnessClient`), so it never reaches the production bundle.

const OPUS = 'claude-opus-5-5';
const SONNET = 'claude-sonnet-5-5';
const min = 60_000;

type TaskSeed = [key: string, milestone: string, title: string, deps: string[], extra?: Partial<HarnessTask>];

const seeds: TaskSeed[] = [
	['PM-01', 'M0', 'Create Jira project and board', [], { state: 'done' }],
	['PM-02', 'M0', 'Add login form', [], { state: 'running', estimatedMinutes: 90 }],
	['PM-04', 'M0', 'Write conventions', [], { state: 'ranked' }],
	['UX-11', 'M0', 'Language and terminology', [], { state: 'running', needsYou: true }],
	['ART-01', 'M0', 'Art direction brief', [], { state: 'running', estimatedMinutes: 60 }],
	['UX-01', 'M0', 'Session design', [], { state: 'ranked' }],
	['UX-02', 'M0', 'Navigation map', [], { state: 'done', finishedOutside: { by: 'Anna' } }],
	['PM-05', 'M0', 'Record asset licenses', [], { state: 'ranked' }],
	['ART-03', 'M0', 'Visual target mockup', ['ART-01']],
	['ART-04', 'M0', 'Decide non-character art', ['ART-01']],
	['ART-14', 'M1', 'Pick fonts', ['ART-01']],
	['UX-03', 'M1', 'Wireframe: world map', ['PM-04']],
	['UX-08', 'M1', 'Interaction patterns', ['UX-01']],
	['SRV-01', 'M1', 'Go module and service skeleton', ['PM-02'], { state: 'running', estimatedMinutes: 80 }],
	['CLI-01', 'M1', 'Godot project', ['PM-01'], { state: 'running' }],
	['UX-04', 'M1', 'Onboarding flow', ['UX-02'], { state: 'ranked' }],
	['PM-03', 'M1', 'Set up CI', ['SRV-01']],
	['DATA-01', 'M1', 'Data format and loader', ['SRV-01']],
	['SRV-02', 'M1', 'Local PostgreSQL', ['SRV-01'], { estimatedMinutes: 70 }],
	['CLI-04', 'M1', 'Sprite pipeline', ['CLI-01']],
	['CLI-06', 'M1', 'Mobile builds', ['CLI-01']],
	['SIM-01', 'M2', 'Fixed-point math', ['CLI-01']],
	['SIM-02', 'M2', 'Deterministic RNG', ['CLI-01']],
	['ART-05', 'M5', 'World map tileset', ['ART-03']],
	['ART-02', 'M1', 'On-device scale test', ['ART-05']],
	['DATA-02', 'M1', 'Unit data', ['DATA-01']],
	['SRV-03', 'M1', 'Guest login', ['SRV-02'], { estimatedMinutes: 60 }],
	['SRV-05', 'M1', 'Serve game data', ['SRV-02']],
	['DATA-03', 'M2', 'Skill and hero data', ['DATA-02']],
	['SIM-03', 'M2', 'Event queue', ['SIM-01', 'SIM-02']],
	['BAT-01', 'M2', 'Grid model', ['SIM-01']],
	['BAT-05', 'M2', 'Damage', ['BAT-01']],
	['SRV-04', 'M1', 'WebSocket gateway', ['SRV-03'], { state: 'blocked', needsYou: true, estimatedMinutes: 120 }],
	['BAT-02', 'M2', 'Turn queue', ['BAT-01']],
	['BAT-03', 'M2', 'Targeting', ['BAT-01']],
	['BAT-06', 'M2', 'Mana and effects', ['BAT-05']],
	['BAT-08', 'M2', 'Stacked hits', ['BAT-05']],
	['BTV-01', 'M2', 'Battle scene', ['BAT-02']],
	['QA-01', 'M2', 'Rules tests', ['BAT-02', 'SIM-03']],
	['DATA-08', 'M3', 'Data validation tests', ['DATA-03']],
	['CLI-02', 'M1', 'Networking autoload', ['SRV-04'], { estimatedMinutes: 45 }],
	['BAT-04', 'M2', 'Movement', ['BAT-02']],
	['BAT-07', 'M2', 'Troop skills', ['BAT-03']],
	['BAT-10', 'M2', 'Towers', ['BAT-06']],
	['BAT-12', 'M2', 'Pacing', ['BAT-08']],
	['BAT-13', 'M2', 'Battle event format', ['BTV-01']],
	['SIM-05', 'M3', 'Two clocks', ['SIM-03']],
	['SIM-06', 'M3', 'Continuous accrual', ['SIM-05']]
];

const epics: Record<string, string> = { PM: 'Project setup', UX: 'Experience', ART: 'Art', SRV: 'Server', CLI: 'Client', DATA: 'Game data', SIM: 'Simulation', BAT: 'Battle', BTV: 'Battle', QA: 'Quality' };

function sourceOf(key: string): HarnessTask['source']['kind'] {
	return key.startsWith('SRV') || key.startsWith('DATA') ? 'github' : key.startsWith('CLI') ? 'bitbucket' : 'jira';
}

function sourceURL(key: string): string {
	const kind = sourceOf(key);
	if (kind === 'github') return `https://github.com/example/airship/issues/${key.replace(/\D/g, '')}`;
	if (kind === 'bitbucket') return `https://bitbucket.org/example/airship-client/issues/${key.replace(/\D/g, '')}`;
	return `https://jira.example.com/browse/${key}`;
}

interface Store {
	seq: number;
	status: HarnessStatus;
	tasks: Map<string, HarnessTask>;
	runs: Map<string, HarnessRun>;
	stages: Map<string, HarnessStage>;
	agentRuns: Map<string, HarnessAgentRun>;
	approvals: Map<string, HarnessApproval>;
	settings: HarnessSettings;
}

export function createFixtureStore(now = Date.now()): Store {
	const at = (offsetMs: number) => new Date(now - offsetMs).toISOString();
	const tasks = new Map<string, HarnessTask>();
	const ids = new Map<string, string>();
	seeds.forEach(([key]) => ids.set(key, `task-${key.toLowerCase()}`));
	seeds.forEach(([key, milestone, title, deps, extra], index) => {
		const id = ids.get(key) as string;
		const prefix = key.split('-')[0];
		tasks.set(id, {
			id,
			workspaceId: prefix === 'CLI' ? 'airship-client' : 'airship',
			source: { kind: sourceOf(key), key, url: sourceURL(key), status: 'To Do', labels: ['harness'], priority: index < 8 ? 'High' : 'Medium', assignee: ['Anna', 'Khoa', 'Mai'][index % 3], updatedAt: at((index + 2) * 37 * min) },
			title,
			state: 'ranked',
			score: 100 - index,
			scoreReasons: [index < 8 ? 'High priority in the source' : 'Medium priority', `Waiting ${1 + (index % 9)} days`],
			priorityOverride: 0,
			pinned: key === 'PM-02',
			needsYou: false,
			attempts: 0,
			revision: 1,
			dependsOn: deps.map((dep) => ids.get(dep) as string),
			milestone,
			epic: epics[prefix],
			risk: key === 'SRV-04' ? 'high' : key.startsWith('SRV') ? 'medium' : 'low',
			estimatedMinutes: 30 + ((index * 7) % 40),
			createdAt: at(9 * 24 * 60 * min),
			updatedAt: at(index * min),
			...extra
		});
	});
	const task = (key: string) => tasks.get(ids.get(key) as string) as HarnessTask;
	const uxDone = task('UX-02');
	uxDone.finishedOutside = { by: 'Anna', at: at(95 * min) };

	const store: Store = {
		seq: 0,
		status: {
			enabled: true,
			storageSupported: true,
			mode: 'local',
			running: true,
			waitingCount: 0,
			lastEventSeq: 0,
			roles: [],
			engines: [
				{ id: 'claudecode', available: true, version: '2.1.303' },
				{ id: 'codex', available: false, limits: ['Not installed on this machine.'] },
				{ id: 'opencode', available: true, version: '0.9.2', limits: ['Cannot limit which tools an agent may use.'] }
			]
		},
		tasks,
		runs: new Map(),
		stages: new Map(),
		agentRuns: new Map(),
		approvals: new Map(),
		settings: {
			revision: 3,
			enabled: true,
			autoStart: false,
			requireReview: true,
			askQuestions: true,
			slots: { scout: 1, sorter: 1, planner: 1, coder: 2, reviewer: 1, qa: 1 },
			paused: { qa: false },
			sources: [
				{ workspaceId: 'airship', kind: 'jira', enabled: true, query: 'project = PM AND labels = harness AND statusCategory != Done', lastCheckedAt: at(12 * min), lastFound: 3 },
				{ workspaceId: 'airship', kind: 'github', enabled: true, query: 'example/airship', labels: ['harness'], tokenEnvVar: 'GITHUB_TOKEN', lastCheckedAt: at(12 * min), lastFound: 0 },
				{ workspaceId: 'airship-client', kind: 'bitbucket', enabled: false, query: 'example/airship-client', tokenEnvVar: 'BITBUCKET_TOKEN' }
			],
			modelPolicy: {
				planner: { engine: 'claudecode', model: OPUS, maxTurns: 60 },
				plan_reviewer: { engine: 'claudecode', model: OPUS, maxTurns: 25 },
				coder: { engine: 'claudecode', model: SONNET, hardModel: OPUS, maxTurns: 120 },
				code_reviewer: { engine: 'claudecode', model: OPUS, maxTurns: 40 },
				qa_fixer: { engine: 'claudecode', model: SONNET, maxTurns: 60 }
			},
			workspaces: [
				{ workspaceId: 'airship', verificationCommands: ['npm test'], scope: [], models: { code_reviewer: { model: SONNET } }, procedure: { planner: { skill: 'feature-planning', track: 'Plan' } } }
			]
		}
	};

	const kinds: HarnessStageKind[] = ['plan', 'plan_approval', 'implement', 'qa', 'review', 'draft_pr'];
	const addRun = (key: string, startedAgo: number, extra: Partial<HarnessRun> = {}) => {
		const owner = task(key);
		const run: HarnessRun = {
			id: `run-${key.toLowerCase()}`,
			taskId: owner.id,
			workspaceId: owner.workspaceId,
			branch: `harness/${key}-${key.length}f3a`,
			state: 'running',
			totals: { turns: 0, durationMs: 0, costUsd: 0, blocking: 0, advisory: 0 },
			startedAt: at(startedAgo),
			revision: 4,
			...extra
		};
		owner.runId = run.id;
		store.runs.set(run.id, run);
		kinds.forEach((kind, ordinal) => store.stages.set(`${run.id}-${kind}`, {
			id: `${run.id}-${kind}`, runId: run.id, kind, ordinal, state: 'pending', round: 1,
			maxRounds: kind === 'plan' || kind === 'implement' ? 3 : kind === 'qa' ? 2 : 1,
			attempt: 1, maxAttempts: 3, turns: 0, durationMs: 0, costUsd: 0, blocking: 0, advisory: 0
		}));
		return run;
	};
	const stage = (run: HarnessRun, kind: HarnessStageKind, patch: Partial<HarnessStage>) => {
		const current = store.stages.get(`${run.id}-${kind}`) as HarnessStage;
		Object.assign(current, patch);
		if (patch.state === 'running' || patch.state === 'awaiting_approval') run.currentStageId = current.id;
		return current;
	};
	let agentCounter = 0;
	const agent = (run: HarnessRun, kind: HarnessStageKind, role: HarnessAgentRole, round: number, startedAgo: number, patch: Partial<HarnessAgentRun>) => {
		const model = role === 'coder' || role === 'qa_fixer' ? SONNET : OPUS;
		const item: HarnessAgentRun = {
			id: `agent-${++agentCounter}`, runId: run.id, stageId: `${run.id}-${kind}`, role, round, engine: 'claudecode', model,
			state: 'succeeded', startedAt: at(startedAgo), turns: 0, durationMs: 0, costUsd: 0, blocking: 0, advisory: 0, ...patch
		};
		if (item.state !== 'running' && !item.finishedAt) item.finishedAt = at(Math.max(0, startedAgo - item.durationMs));
		store.agentRuns.set(item.id, item);
		return item;
	};
	const approval = (id: string, key: string, gate: HarnessApprovalGate, requestedAgo: number, patch: Partial<HarnessApproval>) => {
		const owner = task(key);
		const item: HarnessApproval = { id, taskId: owner.id, runId: owner.runId, gate, state: 'requested', requestedAt: at(requestedAgo), revision: 1, ...patch };
		store.approvals.set(id, item);
		return item;
	};

	const planFindings: HarnessFinding[] = [
		{ severity: 'blocking', title: 'The plan writes the session cookie without the Secure flag.', path: 'plans/PM-02/solution-01-backend.md', line: 42, resolved: true },
		{ severity: 'advisory', title: 'Name the rate limit for failed sign-ins.' },
		{ severity: 'advisory', title: 'Reuse the existing Dialog for the error state.', path: 'web/src/components/ConfirmDialog.tsx' },
		{ severity: 'advisory', title: 'Add a test for an expired token.' },
		{ severity: 'advisory', title: 'Say which page opens after sign-in.' }
	];

	const pm02 = addRun('PM-02', 31 * min);
	approval('appr-pm02-start', 'PM-02', 'start', 32 * min, { state: 'approved', decidedAt: at(31 * min), decidedBy: { actorId: 'khoa', displayName: 'Khoa' }, channel: 'dashboard', revision: 2 });
	agent(pm02, 'plan', 'planner', 1, 30 * min, { turns: 69, durationMs: 7 * min, costUsd: 2.73, summary: 'Wrote a five-step plan for the login form.' });
	agent(pm02, 'plan', 'plan_reviewer', 1, 23 * min, { turns: 41, durationMs: 2 * min, costUsd: 1.32, verdict: 'changes_requested', blocking: 1, advisory: 4, findings: planFindings });
	agent(pm02, 'plan', 'planner', 2, 21 * min, { turns: 1, durationMs: 1 * min, costUsd: 0.3, summary: 'Added the Secure flag to the cookie step.' });
	agent(pm02, 'plan', 'plan_reviewer', 2, 20 * min, { turns: 2, durationMs: 1 * min, costUsd: 0.12, verdict: 'approve', findings: planFindings.filter((finding) => finding.severity === 'advisory') });
	stage(pm02, 'plan', { state: 'passed', round: 2, turns: 113, durationMs: 11 * min, costUsd: 4.47, advisory: 4, startedAt: at(30 * min), finishedAt: at(19 * min), engine: 'claudecode', model: OPUS });
	approval('appr-pm02-plan', 'PM-02', 'plan', 19 * min, { state: 'approved', decidedAt: at(6 * min), decidedBy: { actorId: 'anna', displayName: 'Anna', email: 'anna@example.com' }, channel: 'webex', revision: 2, context: { verdict: 'approve', blocking: 0, advisory: 4, rounds: 2, costUsd: 4.47, durationMs: 11 * min } });
	stage(pm02, 'plan_approval', { state: 'passed', startedAt: at(19 * min), finishedAt: at(6 * min) });
	agent(pm02, 'implement', 'coder', 1, 5 * min - 14_000, { state: 'running', turns: 23, costUsd: 1.9, progress: 'Writing LoginForm.tsx and its test' });
	stage(pm02, 'implement', { state: 'running', round: 1, turns: 23, costUsd: 1.9, startedAt: at(5 * min - 14_000), engine: 'claudecode', model: SONNET });

	const art01 = addRun('ART-01', 8 * min);
	approval('appr-art01-start', 'ART-01', 'start', 9 * min, { state: 'approved', decidedAt: at(8 * min), decidedBy: { actorId: 'anna', displayName: 'Anna' }, channel: 'webex', revision: 2 });
	agent(art01, 'plan', 'planner', 1, 7 * min - 12_000, { state: 'running', turns: 31, costUsd: 0.84, progress: 'Reading the style references' });
	stage(art01, 'plan', { state: 'running', startedAt: at(7 * min - 12_000), engine: 'claudecode', model: OPUS });

	const srv01 = addRun('SRV-01', 52 * min);
	agent(srv01, 'plan', 'planner', 1, 51 * min, { turns: 40, durationMs: 6 * min, costUsd: 1.8 });
	agent(srv01, 'plan', 'plan_reviewer', 1, 45 * min, { turns: 12, durationMs: 2 * min, costUsd: 0.6, verdict: 'approve' });
	stage(srv01, 'plan', { state: 'passed', turns: 52, durationMs: 8 * min, costUsd: 2.4, startedAt: at(51 * min), finishedAt: at(43 * min) });
	approval('appr-srv01-plan', 'SRV-01', 'plan', 43 * min, { state: 'approved', decidedAt: at(40 * min), decidedBy: { actorId: 'khoa', displayName: 'Khoa' }, channel: 'dashboard', revision: 2 });
	stage(srv01, 'plan_approval', { state: 'passed', startedAt: at(43 * min), finishedAt: at(40 * min) });
	agent(srv01, 'implement', 'coder', 1, 39 * min, { turns: 88, durationMs: 33 * min, costUsd: 5.1 });
	agent(srv01, 'implement', 'code_reviewer', 1, 3 * min - 41_000, { state: 'running', turns: 9, costUsd: 0.42, progress: 'Reading internal/server/server.go' });
	stage(srv01, 'implement', { state: 'running', turns: 97, durationMs: 33 * min, costUsd: 5.52, startedAt: at(39 * min) });

	const cli01 = addRun('CLI-01', 70 * min);
	stage(cli01, 'plan', { state: 'passed', turns: 30, durationMs: 9 * min, costUsd: 2.2, startedAt: at(70 * min), finishedAt: at(61 * min) });
	approval('appr-cli01-plan', 'CLI-01', 'plan', 61 * min, { state: 'approved', decidedAt: at(58 * min), decidedBy: { actorId: 'anna', displayName: 'Anna' }, channel: 'webex', revision: 2 });
	stage(cli01, 'plan_approval', { state: 'passed', startedAt: at(61 * min), finishedAt: at(58 * min) });
	stage(cli01, 'implement', { state: 'passed', turns: 70, durationMs: 40 * min, costUsd: 4.9, startedAt: at(58 * min), finishedAt: at(18 * min) });
	stage(cli01, 'qa', { state: 'running', round: 2, startedAt: at(60_000 + 5_000) });

	const ux11 = addRun('UX-11', 14 * min);
	agent(ux11, 'plan', 'planner', 1, 13 * min, { turns: 52, durationMs: 6 * min, costUsd: 2.6 });
	agent(ux11, 'plan', 'plan_reviewer', 1, 7 * min, { turns: 18, durationMs: 2 * min, costUsd: 0.9, verdict: 'changes_requested', blocking: 1, advisory: 3 });
	agent(ux11, 'plan', 'planner', 2, 5 * min, { turns: 6, durationMs: 1 * min, costUsd: 0.62 });
	agent(ux11, 'plan', 'plan_reviewer', 2, 4 * min, { turns: 7, durationMs: 1 * min, costUsd: 0.35, verdict: 'approve', advisory: 4 });
	stage(ux11, 'plan', { state: 'passed', round: 2, turns: 83, durationMs: 10 * min, costUsd: 4.47, advisory: 4, startedAt: at(13 * min), finishedAt: at(3 * min) });
	stage(ux11, 'plan_approval', { state: 'awaiting_approval', startedAt: at(3 * min) });
	approval('appr-ux11-plan', 'UX-11', 'plan', 3 * min, {
		summary: 'A glossary of 40 game terms, a tone guide, and the rule that every on-screen word comes from one file.',
		estimate: { minMinutes: 5, maxMinutes: 10, costUsd: 3 },
		context: { verdict: 'approve', blocking: 0, advisory: 4, rounds: 2, durationMs: 10 * min, costUsd: 4.47, difficulty: 'easy', subtasks: 4 }
	});

	const srv04 = addRun('SRV-04', 49 * min, { state: 'blocked' });
	for (let round = 1; round <= 3; round++) {
		agent(srv04, 'plan', 'planner', round, (50 - round * 13) * min, { turns: 30 + round, durationMs: 8 * min, costUsd: 1.4 });
		agent(srv04, 'plan', 'plan_reviewer', round, (42 - round * 13) * min, { turns: 9, durationMs: 3 * min, costUsd: 0.6, verdict: 'changes_requested', blocking: 1, advisory: 1, findings: [{ severity: 'blocking', title: 'The ticket does not say which clients must stay connected during a deploy.' }] });
	}
	stage(srv04, 'plan', { state: 'failed', round: 3, turns: 123, durationMs: 41 * min, costUsd: 6.1, blocking: 1, startedAt: at(49 * min), finishedAt: at(8 * min), failureReason: 'Three rounds and one must-fix item still open.' });
	srv04.currentStageId = `${srv04.id}-plan`;
	approval('appr-srv04-resume', 'SRV-04', 'resume', 8 * min, {
		summary: 'The Checker still finds one blocker: the ticket does not say which clients must stay connected during a deploy. The Planner cannot decide that alone.',
		context: { blocking: 1, rounds: 3, durationMs: 41 * min, costUsd: 6.1 }
	});

	const pm01 = addRun('PM-01', 3 * 60 * min, { state: 'passed', finishedAt: at(2 * 60 * min), prUrl: 'https://github.com/example/airship/pull/3103' });
	kinds.forEach((kind) => stage(pm01, kind, { state: 'passed', turns: kind === 'implement' ? 120 : 20, durationMs: 10 * min, costUsd: kind === 'implement' ? 9.4 : 2.05, startedAt: at(3 * 60 * min), finishedAt: at(2 * 60 * min) }));
	approval('appr-pm01-pr', 'PM-01', 'review', 2 * 60 * min + 5 * min, { state: 'approved', decidedAt: at(2 * 60 * min + 2 * min), decidedBy: { actorId: 'khoa', displayName: 'Khoa' }, channel: 'dashboard', revision: 2 });

	for (const run of store.runs.values()) recomputeTotals(store, run);
	refreshRoles(store, now);
	return store;
}

function recomputeTotals(store: Store, run: HarnessRun) {
	const stages = [...store.stages.values()].filter((stage) => stage.runId === run.id);
	for (const stage of stages) {
		const agents = [...store.agentRuns.values()].filter((agent) => agent.stageId === stage.id);
		if (agents.length) {
			stage.turns = agents.reduce((sum, agent) => sum + agent.turns, 0);
			stage.costUsd = round2(agents.reduce((sum, agent) => sum + agent.costUsd, 0));
			stage.durationMs = agents.reduce((sum, agent) => sum + agent.durationMs, 0);
		}
	}
	run.totals = {
		turns: stages.reduce((sum, stage) => sum + stage.turns, 0),
		durationMs: stages.reduce((sum, stage) => sum + stage.durationMs, 0),
		costUsd: round2(stages.reduce((sum, stage) => sum + stage.costUsd, 0)),
		blocking: stages.reduce((sum, stage) => sum + stage.blocking, 0),
		advisory: stages.reduce((sum, stage) => sum + stage.advisory, 0)
	};
	run.currentStage = run.currentStageId ? { ...(store.stages.get(run.currentStageId) as HarnessStage) } : undefined;
	const active = [...store.agentRuns.values()].find((agent) => agent.runId === run.id && agent.state === 'running');
	run.activeAgentRun = active ? { ...active } : undefined;
}

function refreshRoles(store: Store, now: number) {
	const running = [...store.agentRuns.values()].filter((agent) => agent.state === 'running');
	const count = (roles: HarnessAgentRole[]) => running.filter((agent) => roles.includes(agent.role)).length;
	const queued = [...store.tasks.values()].filter((task) => task.state === 'ranked').length;
	const paused = store.settings.paused;
	const roles: HarnessRoleHealth[] = [
		{ role: 'scout', paused: Boolean(paused.scout), working: 0, slots: 1, queued: 0, lastRunAt: new Date(now - 12 * min).toISOString(), found: 3 },
		{ role: 'sorter', paused: Boolean(paused.sorter), working: 0, slots: 1, queued },
		{ role: 'planner', paused: Boolean(paused.planner), working: count(['planner']), slots: store.settings.slots.planner, queued: 0, engine: 'claudecode', model: OPUS },
		{ role: 'coder', paused: Boolean(paused.coder), working: count(['coder', 'qa_fixer']), slots: store.settings.slots.coder, queued: 0, engine: 'claudecode', model: SONNET },
		{ role: 'reviewer', paused: Boolean(paused.reviewer), working: count(['plan_reviewer', 'code_reviewer']), slots: store.settings.slots.reviewer, queued: 0, engine: 'claudecode', model: OPUS },
		{ role: 'qa', paused: Boolean(paused.qa), working: [...store.stages.values()].filter((stage) => stage.kind === 'qa' && stage.state === 'running').length, slots: store.settings.slots.qa, queued: 0 }
	];
	for (const role of roles) if (role.paused) { role.pausedBy = 'Khoa'; role.pausedAt = new Date(now).toISOString(); }
	store.status = { ...store.status, roles, waitingCount: [...store.approvals.values()].filter((approval) => approval.state === 'requested').length, lastEventSeq: store.seq };
}

function round2(value: number) {
	return Math.round(value * 100) / 100;
}

const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value)) as T;
const progressLines = ['Reading the plan', 'Running npm test', 'Editing the form validation', 'Checking the diff', 'Writing a test for an expired token', 'Running the type check'];

export function createFixtureClient(options: { tickMs?: number; arrivalMs?: number; now?: () => number } = {}): HarnessClient {
	const now = options.now ?? Date.now;
	const store = createFixtureStore(now());
	const listeners = new Set<HarnessFeedHandlers>();
	let tick = 0;

	const emit = (event: Omit<HarnessEvent, 'seq' | 'at'>) => {
		store.seq += 1;
		store.status.lastEventSeq = store.seq;
		const full = { ...clone(event), seq: store.seq, at: new Date(now()).toISOString() } as HarnessEvent;
		listeners.forEach((listener) => listener.onEvent(full));
	};
	const runOf = (id?: string) => (id ? store.runs.get(id) : undefined);
	const sync = (run: HarnessRun) => { recomputeTotals(store, run); refreshRoles(store, now()); };
	const startStage = (run: HarnessRun, kind: HarnessStageKind, role?: HarnessAgentRole) => {
		const stage = store.stages.get(`${run.id}-${kind}`) as HarnessStage;
		stage.state = role || kind === 'qa' || kind === 'draft_pr' ? 'running' : 'awaiting_approval';
		stage.startedAt = new Date(now()).toISOString();
		run.currentStageId = stage.id;
		emit({ type: 'stage.started', stage } as Omit<HarnessEvent, 'seq' | 'at'>);
		if (role) {
			const agent: HarnessAgentRun = {
				id: `agent-live-${store.seq}`, runId: run.id, stageId: stage.id, role, round: stage.round, engine: 'claudecode',
				model: role === 'coder' || role === 'qa_fixer' ? SONNET : OPUS, state: 'running', startedAt: stage.startedAt,
				turns: 0, durationMs: 0, costUsd: 0, blocking: 0, advisory: 0, progress: 'Starting'
			};
			store.agentRuns.set(agent.id, agent);
			emit({ type: 'agent.started', agentRun: agent } as Omit<HarnessEvent, 'seq' | 'at'>);
		}
		sync(run);
		emit({ type: 'run.started', run } as Omit<HarnessEvent, 'seq' | 'at'>);
	};
	const finishStage = (run: HarnessRun, kind: HarnessStageKind, state: HarnessStage['state'] = 'passed') => {
		const stage = store.stages.get(`${run.id}-${kind}`) as HarnessStage;
		stage.state = state;
		stage.finishedAt = new Date(now()).toISOString();
		emit({ type: 'stage.finished', stage } as Omit<HarnessEvent, 'seq' | 'at'>);
	};

	const decide = (approval: HarnessApproval, decision: 'approve' | 'reject', reason?: string): HarnessApproval | undefined => {
		const current = store.approvals.get(approval.id);
		if (!current) throw new Error('This approval no longer exists.');
		if (current.state !== 'requested') return undefined;
		current.state = decision === 'approve' ? 'approved' : 'rejected';
		current.decidedAt = new Date(now()).toISOString();
		current.decidedBy = { actorId: 'you', displayName: 'You' };
		current.channel = 'dashboard';
		current.reason = reason;
		current.revision += 1;
		const task = store.tasks.get(current.taskId) as HarnessTask;
		task.needsYou = false;
		task.revision += 1;
		const run = runOf(current.runId);
		emit({ type: 'approval.decided', approval: current } as Omit<HarnessEvent, 'seq' | 'at'>);
		if (run) {
			if (current.gate === 'plan') {
				if (decision === 'approve') {
					finishStage(run, 'plan_approval');
					startStage(run, 'implement', 'coder');
				} else {
					const plan = store.stages.get(`${run.id}-plan`) as HarnessStage;
					plan.round = Math.min(plan.maxRounds, plan.round + 1);
					(store.stages.get(`${run.id}-plan_approval`) as HarnessStage).state = 'pending';
					startStage(run, 'plan', 'planner');
				}
			} else if (current.gate === 'review') {
				if (decision === 'approve') {
					finishStage(run, 'review');
					startStage(run, 'draft_pr');
				}
			} else if (current.gate === 'question') {
				const stage = run.currentStageId ? store.stages.get(run.currentStageId) : undefined;
				if (decision === 'approve' && stage) stage.state = 'running';
				else if (decision === 'reject') { run.state = 'cancelled'; task.state = 'cancelled'; }
			} else if (current.gate === 'resume') {
				if (decision === 'approve') {
					run.state = 'running';
					task.state = 'running';
					const plan = store.stages.get(`${run.id}-plan`) as HarnessStage;
					plan.maxRounds += 1;
					plan.round += 1;
					startStage(run, 'plan', 'planner');
				} else {
					run.state = 'cancelled';
					task.state = 'cancelled';
					sync(run);
					emit({ type: 'run.finished', run } as Omit<HarnessEvent, 'seq' | 'at'>);
				}
			}
			sync(run);
		}
		emit({ type: 'task.updated', task } as Omit<HarnessEvent, 'seq' | 'at'>);
		return clone(current);
	};

	const advance = () => {
		tick += 1;
		for (const agent of store.agentRuns.values()) {
			if (agent.state !== 'running') continue;
			agent.turns += 1 + (tick % 3);
			agent.costUsd = round2(agent.costUsd + 0.04 + (tick % 4) * 0.03);
			agent.progress = progressLines[(tick + agent.turns) % progressLines.length];
			const run = runOf(agent.runId);
			if (run) sync(run);
			emit({ type: 'agent.progress', agentRun: agent } as Omit<HarnessEvent, 'seq' | 'at'>);
		}
	};

	const arrive = () => {
		const run = runOf('run-cli-01');
		if (!run || run.state !== 'running') return;
		finishStage(run, 'qa');
		const gate = store.stages.get(`${run.id}-review`) as HarnessStage;
		gate.state = 'awaiting_approval';
		gate.startedAt = new Date(now()).toISOString();
		run.currentStageId = gate.id;
		const task = store.tasks.get(run.taskId) as HarnessTask;
		task.needsYou = true;
		const approval: HarnessApproval = {
			id: 'appr-cli01-pr', taskId: task.id, runId: run.id, gate: 'review', state: 'requested', requestedAt: new Date(now()).toISOString(), revision: 1,
			context: { verdict: 'approve', blocking: 0, advisory: 2, rounds: 1, costUsd: run.totals.costUsd, durationMs: run.totals.durationMs }
		};
		store.approvals.set(approval.id, approval);
		sync(run);
		emit({ type: 'stage.started', stage: gate } as Omit<HarnessEvent, 'seq' | 'at'>);
		emit({ type: 'task.updated', task } as Omit<HarnessEvent, 'seq' | 'at'>);
		emit({ type: 'approval.requested', approval } as Omit<HarnessEvent, 'seq' | 'at'>);
	};

	let ticker: number | undefined;
	let arrival: number | undefined;
	const ensureTimers = () => {
		if (ticker === undefined && options.tickMs !== 0) ticker = window.setInterval(advance, options.tickMs ?? 4000);
		if (arrival === undefined && options.arrivalMs !== 0) arrival = window.setTimeout(arrive, options.arrivalMs ?? 30_000);
	};
	const stopTimers = () => {
		if (ticker !== undefined) window.clearInterval(ticker);
		if (arrival !== undefined) window.clearTimeout(arrival);
		ticker = undefined;
		arrival = undefined;
	};

	const detail = (runId: string): HarnessRunDetail => {
		const run = runOf(runId);
		if (!run) throw new Error('This run does not exist in the demo data.');
		sync(run);
		return clone({
			run,
			task: store.tasks.get(run.taskId) as HarnessTask,
			stages: [...store.stages.values()].filter((stage) => stage.runId === runId).sort((a, b) => a.ordinal - b.ordinal),
			agentRuns: [...store.agentRuns.values()].filter((agent) => agent.runId === runId),
			approvals: [...store.approvals.values()].filter((approval) => approval.runId === runId)
		});
	};

	const delay = <T,>(value: () => T) => new Promise<T>((resolve, reject) => window.setTimeout(() => {
		try { resolve(value()); } catch (error) { reject(error); }
	}, 120));

	return {
		fixture: true,
		status: () => delay(() => { refreshRoles(store, now()); return clone(store.status); }),
		tasks: () => delay(() => clone([...store.tasks.values()])),
		runs: () => delay(() => { store.runs.forEach((run) => recomputeTotals(store, run)); return clone([...store.runs.values()]); }),
		approvals: () => delay(() => clone([...store.approvals.values()])),
		cost: (groupBy) => delay((): HarnessCostSummary => {
			const groups = new Map<string, { costUsd: number; agentRuns: number; turns: number; durationMs: number }>();
			for (const agent of store.agentRuns.values()) {
				const key = groupBy === 'model' ? agent.model : groupBy === 'run' ? agent.runId : agent.role;
				const group = groups.get(key) ?? { costUsd: 0, agentRuns: 0, turns: 0, durationMs: 0 };
				group.costUsd = round2(group.costUsd + agent.costUsd);
				group.agentRuns += 1;
				group.turns += agent.turns;
				group.durationMs += agent.durationMs;
				groups.set(key, group);
			}
			const list = [...groups].map(([key, group]) => ({ key, ...group }));
			return { groupBy, from: new Date(now() - 24 * 60 * min).toISOString(), to: new Date(now()).toISOString(), totalUsd: round2(list.reduce((sum, group) => sum + group.costUsd, 0)), agentRuns: list.reduce((sum, group) => sum + group.agentRuns, 0), groups: list };
		}),
		run: (runId) => delay(() => detail(runId)),
		runChanges: (runId) => delay((): SessionChanges => fixtureChanges(runId)),
		runPlan: (runId) => delay((): HarnessRunPlan => {
			const run = store.runs.get(runId);
			if (!run) throw new Error('This run does not exist in the demo data.');
			const task = store.tasks.get(run.taskId);
			const key = task?.source.key ?? 'TASK';
			return { branch: run.branch, from: run.state === 'running' || run.state === 'blocked' ? 'worktree' : 'branch', files: [
				{ path: `plans/${key}/README.md`, status: 'added', content: `# ${task?.title ?? key}\n\nWhat the Planner proposes, in three steps.\n\n1. Read the current flow.\n2. Change the smallest part that fixes it.\n3. Add a test that fails before the change.\n` },
				{ path: `plans/${key}/plan.yaml`, status: 'added', content: `plan:\n  identifier: ${key}\n  status: draft\n` }
			] };
		}),
		agentRun: (agentRunId) => delay(() => {
			const agentRun = store.agentRuns.get(agentRunId);
			if (!agentRun) throw new Error('This agent run does not exist in the demo data.');
			return { agentRun: clone(agentRun), session: undefined as SafeSessionRecord | undefined };
		}),
		agentRunTranscript: (agentRunId) => delay(() => {
			const agentRun = store.agentRuns.get(agentRunId);
			if (!agentRun) throw new Error('This agent run does not exist in the demo data.');
			const turns: ClaudeTurn[] = [
				{ role: 'user', text: `Brief for the ${agentRun.role.replace('_', ' ')}, round ${agentRun.round}.` },
				{ role: 'assistant', text: agentRun.progress ?? agentRun.summary ?? 'Reading the code.' }
			];
			return { agentRunId, sessionId: `sess-${agentRunId}`, available: true, turns };
		}),
		approve: (approval, _key, reason) => delay(() => decide(approval, 'approve', reason)),
		reject: (approval, _key, reason) => delay(() => decide(approval, 'reject', reason)),
		pin: (task, pinned) => delay(() => {
			const current = store.tasks.get(task.id) as HarnessTask;
			current.pinned = pinned;
			current.revision += 1;
			emit({ type: 'task.pinned', task: current } as Omit<HarnessEvent, 'seq' | 'at'>);
			return clone(current);
		}),
		setQuestions: (task, askQuestions) => delay(() => {
			const current = store.tasks.get(task.id) as HarnessTask;
			current.askQuestions = askQuestions;
			current.revision += 1;
			emit({ type: 'task.updated', task: current } as Omit<HarnessEvent, 'seq' | 'at'>);
			return clone(current);
		}),
		start: (task) => delay(() => {
			const current = store.tasks.get(task.id) as HarnessTask;
			current.startRequested = true;
			if (current.state === 'cancelled' || current.state === 'discovered') { current.state = 'ranked'; current.note = undefined; }
			current.revision += 1;
			emit({ type: 'task.updated', task: current } as Omit<HarnessEvent, 'seq' | 'at'>);
			return clone(current);
		}),
		setPriority: (task, input) => delay(() => {
			const current = store.tasks.get(task.id) as HarnessTask;
			if (input.before) {
				const above = store.tasks.get(input.before);
				if (above) current.score = above.score + 0.5;
			} else if (input.priorityOverride !== undefined) {
				current.priorityOverride = input.priorityOverride;
				current.score += input.priorityOverride;
			}
			current.revision += 1;
			emit({ type: 'task.ranked', task: current } as Omit<HarnessEvent, 'seq' | 'at'>);
			return clone(current);
		}),
		pauseRole: (role) => delay(() => {
			store.settings.paused = { ...store.settings.paused, [role]: true };
			refreshRoles(store, now());
			const health = store.status.roles.find((item) => item.role === role) as HarnessRoleHealth;
			emit({ type: 'role.paused', role: health } as Omit<HarnessEvent, 'seq' | 'at'>);
			return clone(health);
		}),
		resumeRole: (role) => delay(() => {
			store.settings.paused = { ...store.settings.paused, [role]: false };
			refreshRoles(store, now());
			const health = store.status.roles.find((item) => item.role === role) as HarnessRoleHealth;
			emit({ type: 'role.resumed', role: health } as Omit<HarnessEvent, 'seq' | 'at'>);
			return clone(health);
		}),
		settings: () => delay(() => clone(store.settings)),
		workspaces: () => delay(() => [
			{ id: 'airship', name: 'Airship', jiraProjectKey: 'PM' },
			{ id: 'airship-client', name: 'Airship client' },
			{ id: 'airship-docs', name: 'Airship docs', jiraProjectKey: 'DOC' }
		]),
		saveSettings: (settings) => delay(() => {
			if (settings.revision !== store.settings.revision) throw new Error('Someone changed the settings first.');
			store.settings = { ...clone(settings), revision: settings.revision + 1 };
			return clone(store.settings);
		}),
		cancelRun: (target, reason) => delay(() => {
			const run = store.runs.get(target.id) as HarnessRun;
			const task = store.tasks.get(run.taskId) as HarnessTask;
			run.state = 'cancelled';
			run.revision += 1;
			task.state = 'cancelled';
			task.startRequested = false;
			task.revision += 1;
			for (const agentRun of store.agentRuns.values()) if (agentRun.runId === run.id && agentRun.state === 'running') agentRun.state = 'cancelled';
			sync(run);
			emit({ type: 'run.finished', run } as Omit<HarnessEvent, 'seq' | 'at'>);
			emit({ type: 'task.updated', task } as Omit<HarnessEvent, 'seq' | 'at'>);
			return clone(run);
		}),
		clearQueue: (workspaceId) => delay(() => {
			const clearable = new Set(['discovered', 'ranked', 'stale', 'dismissed']);
			const gone = [...store.tasks.values()].filter((task) => clearable.has(task.state) && (!workspaceId || task.workspaceId === workspaceId));
			for (const task of gone) {
				store.tasks.delete(task.id);
				emit({ type: 'task.removed', taskId: task.id } as Omit<HarnessEvent, 'seq' | 'at'>);
			}
			return { removed: gone.length };
		}),
		subscribe: (_afterSeq, handlers) => {
			listeners.add(handlers);
			ensureTimers();
			window.setTimeout(() => handlers.onOpen(), 0);
			return () => {
				listeners.delete(handlers);
				if (listeners.size === 0) stopTimers();
			};
		}
	};
}

function fixtureChanges(runId: string): SessionChanges {
	const files: SessionChanges['files'] = runId === 'run-pm-02'
		? [{ path: 'web/src/features/auth/LoginForm.tsx', status: 'A' }, { path: 'web/src/features/auth/LoginForm.test.tsx', status: 'A' }, { path: 'web/src/app/router.ts', status: 'M' }, { path: 'internal/server/api/session_controller.go', status: 'M' }]
		: [{ path: 'README.md', status: 'M' }];
	return { sessionId: runId, launchCommit: 'a1b2c3d', headCommit: 'e4f5a6b', headMoved: 0, files, additions: 267, deletions: 5, truncated: false, availability: 'available' };
}
