import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { parseStructuredContent, StructuredDataView } from './StructuredDataView';

describe('StructuredDataView', () => {
  it('parses JSON and YAML into plain structured values', () => {
    expect(parseStructuredContent('{"enabled":true}', 'json')).toEqual({ enabled: true });
    expect(parseStructuredContent('name: viewer\nitems:\n  - one', 'yaml')).toEqual({ name: 'viewer', items: ['one'] });
  });

  it('shows parse errors without throwing', () => {
    render(<StructuredDataView content="{invalid" language="json" />);

    expect(screen.getByRole('alert')).toHaveTextContent('JSON could not be parsed.');
  });

  it('allows tree nodes to collapse and expand', () => {
    render(<StructuredDataView content='{"nested":{"value":1}}' language="json" />);

    const root = screen.getByRole('button', { name: /root/ });
    expect(root).toHaveAttribute('aria-expanded', 'true');
    fireEvent.click(root);
    expect(root).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(root);
    expect(screen.getByText('nested')).toBeInTheDocument();
  });
});
