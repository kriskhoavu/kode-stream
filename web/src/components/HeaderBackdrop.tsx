import { useLayoutEffect } from 'react';
import { LatticeBackdrop } from './LatticeBackdrop';
import { NeonBackdrop } from './NeonBackdrop';
import './header-backdrop.css';

/**
 * The header backdrop is the decorative layer filling the band that runs from
 * the topbar down to the bottom of each page's title row, so the board, its
 * toolbar and its filters sit on a still background.
 *
 * This module owns what is true of every variant — the variant union and the
 * routes that carry a band — so adding a treatment touches one module rather
 * than the shell, the settings and the styles at once.
 */

/** Which treatment the band paints, or `none` to leave the band off entirely. */
export type BackdropVariant = 'lattice' | 'neon' | 'none';

/** Every valid variant, in the order Settings offers them. */
export const backdropVariants = ['lattice', 'neon', 'none'] as const satisfies readonly BackdropVariant[];

export const defaultBackdropVariant: BackdropVariant = 'lattice';

/** Narrow an unvalidated stored value to a variant, falling back to the default. */
export function toBackdropVariant(value: unknown): BackdropVariant {
  return (backdropVariants as readonly string[]).includes(value as string)
    ? (value as BackdropVariant)
    : defaultBackdropVariant;
}

/**
 * The row the band ends under in kode-stream: each band route's page title row.
 * An application with other page markup passes its own to HeaderBackdrop.
 */
export const bandEndSelector = '#app-main .page-title, #app-main .knowledge-heading-row';

/**
 * Holds the band to the page's title row. The band is absolutely placed in the
 * shell and does not scroll, so its height follows the title row's bottom as
 * the page scrolls, never shorter than the topbar it sits behind. Without a
 * title row the stylesheet's default height stands.
 */
function useBandEnd(endSelector: string) {
  useLayoutEffect(() => {
    const shell = document.querySelector<HTMLElement>('.app-shell');
    if (!shell) return;
    let timer = 0;
    const measure = () => {
      const end = document.querySelector(endSelector);
      if (!end) {
        shell.style.removeProperty('--band-height');
        return;
      }
      const top = shell.getBoundingClientRect().top;
      const topbar = shell.querySelector('.topbar');
      const floor = topbar ? topbar.getBoundingClientRect().bottom - top : 0;
      const height = Math.max(floor, end.getBoundingClientRect().bottom - top);
      shell.style.setProperty('--band-height', `${Math.round(height)}px`);
    };
    // A timer rather than an animation frame: a hidden tab paints no frames,
    // and the band must already fit when it is shown.
    const schedule = () => {
      window.clearTimeout(timer);
      timer = window.setTimeout(measure, 0);
    };
    measure();
    const resize = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(schedule);
    resize?.observe(shell);
    const mutations = new MutationObserver(schedule);
    mutations.observe(shell, { childList: true, subtree: true });
    window.addEventListener('resize', schedule);
    // Capture, because scroll does not bubble: whichever element scrolls the page.
    document.addEventListener('scroll', measure, { capture: true, passive: true });
    return () => {
      document.removeEventListener('scroll', measure, { capture: true });
      window.clearTimeout(timer);
      resize?.disconnect();
      mutations.disconnect();
      window.removeEventListener('resize', schedule);
      shell.style.removeProperty('--band-height');
    };
  }, [endSelector]);
}

/** Routes whose header-to-filter band carries a backdrop. */
export const backdropRoutes = ['workstream', 'canvas', 'knowledge', 'harness'] as const;

/**
 * The shell classes this route and variant call for, empty when nothing paints.
 *
 * One source for both the class list and whether the backdrop renders. The band
 * class also strips the topbar's fill, blur and border, so a rendered-but-empty
 * band would leave the topbar with no chrome and nothing behind it.
 *
 * The variant class is here because the page beneath the band has to know which
 * treatment it is under: `.main-content`'s corner glows suit Neon and muddy the
 * lattice. There is no per-route class — the band is one height everywhere, and
 * nothing else has ever needed to style a band by route.
 */
export function bandClasses(routeName: string, variant: BackdropVariant): string {
  if (variant === 'none' || !(backdropRoutes as readonly string[]).includes(routeName)) return '';
  return `header-band backdrop-${variant}`;
}

/**
 * The band, filled by the chosen variant. Presentational only: hidden from
 * assistive technology and transparent to pointer events.
 *
 * Renders nothing for `none`. The shell must drop the band class in that case
 * too — the class also strips the topbar's fill, blur and border, so an empty
 * band would leave the topbar with no chrome and nothing behind it.
 */
export function HeaderBackdrop({ variant, endSelector = bandEndSelector }: { variant: BackdropVariant; endSelector?: string }) {
  useBandEnd(endSelector);
  if (variant === 'none') return null;

  return (
    <div className="header-backdrop" aria-hidden="true">
      {variant === 'lattice' && <LatticeBackdrop />}
      {variant === 'neon' && <NeonBackdrop />}
    </div>
  );
}
