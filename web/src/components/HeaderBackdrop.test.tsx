import { cleanup, render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { HeaderBackdrop, backdropRoutes, backdropVariants, bandClasses, toBackdropVariant } from './HeaderBackdrop';

afterEach(cleanup);

describe('HeaderBackdrop', () => {
  it('renders a decorative layer hidden from assistive technology', () => {
    const { container } = render(<HeaderBackdrop variant="neon" />);
    const layer = container.querySelector('.header-backdrop');

    expect(layer).toBeTruthy();
    expect(layer).toHaveAttribute('aria-hidden', 'true');
  });

  it('dispatches to the neon variant', () => {
    const { container } = render(<HeaderBackdrop variant="neon" />);
    expect(container.querySelectorAll('.neon-icon').length).toBeGreaterThan(0);
  });

  it('renders nothing at all for none', () => {
    const { container } = render(<HeaderBackdrop variant="none" />);
    expect(container.firstChild).toBeNull();
  });

  it('decorates the workstream, workbench and knowledge routes only', () => {
    expect([...backdropRoutes]).toEqual(['workstream', 'canvas', 'knowledge']);
  });

  /*
   * The band class strips the topbar's fill, blur and border, so it must never
   * be applied where nothing will paint. Both ways of getting there — an
   * off-band route and the none variant — have to answer the same.
   */
  it('claims no band class where nothing will paint', () => {
    expect(bandClasses('workstream', 'none')).toBe('');
    expect(bandClasses('settings', 'neon')).toBe('');
    expect(bandClasses('settings', 'none')).toBe('');
  });

  /*
   * The variant class is what lets the page beneath react to the treatment
   * above it — `.main-content` drops its corner glows under the lattice. The
   * route does not appear: every band is the same height.
   */
  it('names the variant on the band class, and not the route', () => {
    expect(bandClasses('workstream', 'lattice')).toBe('header-band backdrop-lattice');
    expect(bandClasses('canvas', 'neon')).toBe('header-band backdrop-neon');
  });

  it('narrows unknown stored values to a known variant', () => {
    for (const variant of backdropVariants) expect(toBackdropVariant(variant)).toBe(variant);
    for (const bad of [undefined, null, 'aurora', 7, {}]) {
      expect(backdropVariants).toContain(toBackdropVariant(bad));
    }
  });
});
