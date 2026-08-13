import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ViewerErrorBoundary } from './ViewerErrorBoundary';

function Preview({ fail }: { fail: boolean }) { if (fail) throw new Error('bad preview'); return <span>New file preview</span>; }

describe('ViewerErrorBoundary', () => {
  it('recovers when the selected content identity changes', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const { rerender } = render(<ViewerErrorBoundary resetKey="old"><Preview fail /></ViewerErrorBoundary>);
    expect(screen.getByText(/could not be rendered safely/i)).toBeInTheDocument();
    rerender(<ViewerErrorBoundary resetKey="new"><Preview fail={false} /></ViewerErrorBoundary>);
    expect(screen.getByText('New file preview')).toBeInTheDocument();
  });
});
