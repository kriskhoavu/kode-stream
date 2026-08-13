import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ConfirmDialog } from './ConfirmDialog';

describe('ConfirmDialog', () => {
  it('traps focus and does not restore focus while a busy dialog remains mounted', () => {
    const opener = document.createElement('button');
    document.body.append(opener); opener.focus();
    const cancel = vi.fn();
    const { rerender } = render(<ConfirmDialog title="Remove" message="Confirm" confirmLabel="Remove" onCancel={cancel} onConfirm={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Remove' })).toHaveFocus();
    rerender(<ConfirmDialog title="Remove" message="Confirm" confirmLabel="Remove" busy onCancel={cancel} onConfirm={vi.fn()} />);
    expect(document.activeElement).not.toBe(opener);
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(cancel).not.toHaveBeenCalled();
  });
});
