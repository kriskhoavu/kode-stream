import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ErrorBoundary } from './ErrorBoundary';

function Broken({ fail }: { fail: boolean }) {
  if (fail) throw new Error('internal token=secret');
  return <p>Recovered view</p>;
}

describe('ErrorBoundary', () => {
  it('redacts thrown text and can reset on an owned navigation recovery signal', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const { rerender } = render(<ErrorBoundary resetKey="one"><Broken fail /></ErrorBoundary>);
    expect(screen.getByText(/Something went wrong/)).toBeInTheDocument();
    expect(screen.queryByText(/token=secret/)).not.toBeInTheDocument();
    rerender(<ErrorBoundary resetKey="two"><Broken fail={false} /></ErrorBoundary>);
    expect(screen.getByText('Recovered view')).toBeInTheDocument();
  });

  it('offers a non-reload recovery action', () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const navigate = vi.fn();
    window.addEventListener('kode-stream:navigate', navigate, { once: true });
    render(<ErrorBoundary><Broken fail /></ErrorBoundary>);
    fireEvent.click(screen.getByRole('button', { name: 'Return to Workstream' }));
    expect(navigate).toHaveBeenCalled();
  });
});
