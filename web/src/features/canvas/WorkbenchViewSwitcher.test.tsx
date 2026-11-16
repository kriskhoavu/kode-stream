import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { WorkbenchViewSwitcher } from './WorkbenchViewSwitcher';

describe('WorkbenchViewSwitcher', () => {
	it('puts Sessions, the first view, on the left of Canvas', () => {
		const onChange = vi.fn();
		render(<WorkbenchViewSwitcher view="sessions" sessionCount={2} onChange={onChange} />);
		const switcher = screen.getByRole('group', { name: 'Workbench view' });
		expect(within(switcher).getAllByRole('button').map((button) => button.textContent?.trim())).toEqual(['Sessions (2)', 'Canvas']);
		expect(within(switcher).getByRole('button', { name: /Sessions/ })).toHaveAttribute('aria-pressed', 'true');
		fireEvent.click(within(switcher).getByRole('button', { name: 'Canvas' }));
		expect(onChange).toHaveBeenCalledWith('canvas');
	});
});
