import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { FileMenu } from './FileMenu';

describe('FileMenu', () => {
  it('supports Escape and roving arrow focus', () => {
    render(<FileMenu value="one" options={[{ id: 'one', label: 'One' }, { id: 'two', label: 'Two' }]} onChange={vi.fn()} />);
    const trigger = screen.getByRole('button', { name: 'Select document' });
    fireEvent.click(trigger);
    const one = screen.getByRole('menuitemradio', { name: 'One' });
    one.focus();
    fireEvent.keyDown(window, { key: 'ArrowDown' });
    expect(screen.getByRole('menuitemradio', { name: 'Two' })).toHaveFocus();
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('menu', { name: 'Documents' })).not.toBeInTheDocument();
  });
});
