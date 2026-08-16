import { cleanup, render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { NeonHeaderBackdrop, neonBandRoutes } from './NeonHeaderBackdrop';

afterEach(cleanup);

describe('NeonHeaderBackdrop', () => {
  it('renders a decorative layer hidden from assistive technology', () => {
    const { container } = render(<NeonHeaderBackdrop />);
    const layer = container.querySelector('.neon-backdrop');

    expect(layer).toBeTruthy();
    expect(layer).toHaveAttribute('aria-hidden', 'true');
  });

  it('paints a stable scattered icon field with per-icon motion offsets', () => {
    const { container } = render(<NeonHeaderBackdrop />);
    const icons = [...container.querySelectorAll<HTMLElement>('.neon-backdrop-icon')];

    expect(icons.length).toBeGreaterThanOrEqual(20);
    expect(new Set(icons.map((icon) => `${icon.style.left}|${icon.style.top}`)).size).toBe(icons.length);
    expect(new Set(icons.map((icon) => icon.style.animationDelay)).size).toBeGreaterThan(1);
    for (const icon of icons) {
      expect(icon.style.getPropertyValue('--neon-hue')).not.toBe('');
      expect(icon.querySelector('svg')).toBeTruthy();
    }
  });

  it('decorates the workstream, workbench and knowledge routes only', () => {
    expect([...neonBandRoutes]).toEqual(['workstream', 'canvas', 'knowledge']);
  });
});
