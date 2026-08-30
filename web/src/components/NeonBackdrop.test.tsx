import { cleanup, render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { NeonBackdrop } from './NeonBackdrop';

afterEach(cleanup);

describe('NeonBackdrop', () => {
  it('paints a stable scattered icon field with per-icon motion offsets', () => {
    const { container } = render(<NeonBackdrop />);
    const icons = [...container.querySelectorAll<HTMLElement>('.neon-icon')];

    expect(icons.length).toBeGreaterThanOrEqual(20);
    expect(new Set(icons.map((icon) => `${icon.style.left}|${icon.style.top}`)).size).toBe(icons.length);
    expect(new Set(icons.map((icon) => icon.style.animationDelay)).size).toBeGreaterThan(1);
    for (const icon of icons) {
      expect(icon.style.getPropertyValue('--neon-hue')).not.toBe('');
      expect(icon.querySelector('svg')).toBeTruthy();
    }
  });
});
