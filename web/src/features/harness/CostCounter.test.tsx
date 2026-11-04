import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CostCounter } from './CostCounter';
import { ConnectionIndicator } from './ConnectionIndicator';

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
	vi.useRealTimers();
});

describe('CostCounter', () => {
	it('counts up to a new value with the increase beside it, and reads the final value', async () => {
		vi.useFakeTimers({ toFake: ['setTimeout', 'setInterval', 'requestAnimationFrame', 'cancelAnimationFrame', 'performance', 'Date'] });
		vi.stubGlobal('matchMedia', () => ({ matches: false }));
		const { rerender, container } = render(<CostCounter value={11.32} />);
		rerender(<CostCounter value={11.63} />);
		expect(screen.getByText('$11.63', { selector: '.sr-only' })).toBeInTheDocument();
		expect(container.querySelector('.hx-cost-ghost')).toHaveTextContent('+$0.31');
		await act(async () => { await vi.advanceTimersByTimeAsync(700); });
		expect(container.querySelector('[aria-hidden="true"].hx-num')).toHaveTextContent('$11.63');
	});

	it('jumps straight to the new value with reduced motion', () => {
		vi.stubGlobal('matchMedia', () => ({ matches: true }));
		const { rerender, container } = render(<CostCounter value={1} />);
		rerender(<CostCounter value={2} />);
		expect(container.querySelector('[aria-hidden="true"].hx-num')).toHaveTextContent('$2.00');
		expect(container.querySelector('.hx-cost-ghost')).toBeNull();
	});
});

describe('ConnectionIndicator', () => {
	it('always says the state in words', () => {
		const { rerender } = render(<ConnectionIndicator connection="reconnecting" />);
		expect(screen.getByText('Reconnecting…')).toBeInTheDocument();
		rerender(<ConnectionIndicator connection="polling" />);
		expect(screen.getByText('Checking every 10 s')).toBeInTheDocument();
	});
});
