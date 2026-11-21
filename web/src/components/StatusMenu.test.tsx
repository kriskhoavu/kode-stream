import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { StatusMenu } from './StatusMenu';

describe('StatusMenu', () => {
  it('uses semantic radio menu items and roving keyboard focus', () => {
    render(<StatusMenu value="draft" onChange={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: 'Change item status' }));
    const draft = screen.getByRole('menuitemradio', { name: 'Draft' });
    expect(draft).toHaveAttribute('aria-checked', 'true');
    draft.focus(); fireEvent.keyDown(window, { key: 'ArrowDown' });
    expect(screen.getByRole('menuitemradio', { name: 'In Progress' })).toHaveFocus();
  });
});
