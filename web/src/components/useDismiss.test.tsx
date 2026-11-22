import { fireEvent, render } from '@testing-library/react';
import { useRef } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { useDismiss } from './useDismiss';

function Menu({ open, onDismiss }: { open: boolean; onDismiss: () => void }) {
	const ref = useRef<HTMLDivElement>(null);
	useDismiss(ref, open, onDismiss);
	return <div><div ref={ref} data-testid="inside">menu</div><span data-testid="outside">page</span></div>;
}

describe('useDismiss', () => {
	it('dismisses on a pointer outside or Escape, not on a pointer inside', () => {
		const onDismiss = vi.fn();
		const { getByTestId } = render(<Menu open onDismiss={onDismiss} />);
		fireEvent.pointerDown(getByTestId('inside'));
		expect(onDismiss).not.toHaveBeenCalled();
		fireEvent.pointerDown(getByTestId('outside'));
		fireEvent.keyDown(document, { key: 'Escape' });
		expect(onDismiss).toHaveBeenCalledTimes(2);
	});

	it('does not listen while closed', () => {
		const onDismiss = vi.fn();
		const { getByTestId } = render(<Menu open={false} onDismiss={onDismiss} />);
		fireEvent.pointerDown(getByTestId('outside'));
		fireEvent.keyDown(document, { key: 'Escape' });
		expect(onDismiss).not.toHaveBeenCalled();
	});
});
