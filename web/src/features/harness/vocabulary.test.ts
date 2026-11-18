import { describe, expect, it } from 'vitest';
import { formatClock, formatCost, formatMinutes, modelLabel, stageStatus, taskStatus, workerLabel, hxStatuses } from './vocabulary';
import { task } from './testData';

describe('harness vocabulary', () => {
	it('names models in plain words and keeps raw ids out of the label', () => {
		expect(modelLabel('claude-opus-5-5')).toBe('Opus 5.5');
		expect(modelLabel('sonnet')).toBe('Sonnet');
		expect(workerLabel('claudecode', 'claude-sonnet-5-5')).toBe('Claude · Sonnet 5.5');
		expect(workerLabel(undefined, undefined)).toBe('No model');
	});

	it('gives every status a word and an icon', () => {
		expect(Object.values(hxStatuses).map((status) => status.label)).toEqual(['Not started', 'Up next', 'Starting', 'Working', 'Waiting on you', 'Done', 'Needs a fix', "Didn't pass", 'Stopped', 'Skipped', 'Done by hand']);
		expect(Object.values(hxStatuses).every((status) => Boolean(status.icon))).toBe(true);
	});

	it('derives task and stage statuses', () => {
		expect(taskStatus(task('A'))).toBe('up_next');
		expect(taskStatus(task('A'), ['B'])).toBe('not_started');
		expect(taskStatus(task('A', { startRequested: true }))).toBe('starting');
		expect(taskStatus(task('A', { needsYou: true, state: 'running' }))).toBe('waiting');
		expect(taskStatus(task('A', { state: 'done', finishedOutside: { by: 'Anna' } }))).toBe('manual');
		expect(taskStatus(task('A', { state: 'stale' }))).toBe('skipped');
		expect(stageStatus('retry_wait')).toBe('needs_fix');
		expect(stageStatus('failed')).toBe('failed');
	});

	it('formats time and money for people', () => {
		expect(formatCost(4.471)).toBe('$4.47');
		expect(formatMinutes(10 * 60_000)).toBe('10 min');
		expect(formatMinutes(20_000)).toBe('under 1 min');
		expect(formatMinutes(125 * 60_000)).toBe('2 h 5 min');
		expect(formatClock(432_000)).toBe('7:12');
		expect(formatClock(3_723_000)).toBe('1:02:03');
	});
});
