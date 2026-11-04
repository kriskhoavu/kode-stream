import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { LiveTimer } from './LiveTimer';

beforeEach(() => vi.useFakeTimers({ now: new Date('2026-11-04T10:00:00.000Z') }));
afterEach(() => {
	cleanup();
	vi.useRealTimers();
});

describe('LiveTimer on the shared clock', () => {
	it('ticks every second, and every timer on the page ticks together', () => {
		render(<><LiveTimer since="2026-11-04T09:52:48.000Z" /><LiveTimer since="2026-11-04T09:56:19.000Z" /></>);
		expect(screen.getByText('7:12')).toBeInTheDocument();
		expect(screen.getByText('3:41')).toBeInTheDocument();
		act(() => { vi.advanceTimersByTime(1000); });
		expect(screen.getByText('7:13')).toBeInTheDocument();
		expect(screen.getByText('3:42')).toBeInTheDocument();
		expect(vi.getTimerCount()).toBe(1);
	});
});
