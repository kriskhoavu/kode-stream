import { LatticeBackdrop } from './LatticeBackdrop';
import { NeonBackdrop } from './NeonBackdrop';
import './header-backdrop.css';

/**
 * The header backdrop is the decorative layer filling the band that runs from
 * the topbar down through each page's toolbar and filter rows.
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

/** Routes whose header-to-filter band carries a backdrop. */
export const backdropRoutes = ['workstream', 'canvas', 'knowledge'] as const;

/**
 * The shell classes this route and variant call for, empty when nothing paints.
 *
 * One source for both the class list and whether the backdrop renders. The band
 * class also strips the topbar's fill, blur and border, so a rendered-but-empty
 * band would leave the topbar with no chrome and nothing behind it.
 *
 * The variant class is here because the page beneath the band sometimes has to
 * know which treatment it is under: `.main-content`'s corner glows suit Neon
 * and muddy the lattice.
 */
export function bandClasses(routeName: string, variant: BackdropVariant): string {
  if (variant === 'none' || !(backdropRoutes as readonly string[]).includes(routeName)) return '';
  return `header-band header-band-${routeName} backdrop-${variant}`;
}

/**
 * The band, filled by the chosen variant. Presentational only: hidden from
 * assistive technology and transparent to pointer events.
 *
 * Renders nothing for `none`. The shell must drop the band class in that case
 * too — the class also strips the topbar's fill, blur and border, so an empty
 * band would leave the topbar with no chrome and nothing behind it.
 */
export function HeaderBackdrop({ variant }: { variant: BackdropVariant }) {
  if (variant === 'none') return null;

  return (
    <div className="header-backdrop" aria-hidden="true">
      {variant === 'lattice' && <LatticeBackdrop />}
      {variant === 'neon' && <NeonBackdrop />}
    </div>
  );
}
