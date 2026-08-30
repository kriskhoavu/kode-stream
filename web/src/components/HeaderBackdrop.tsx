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

/*
 * Stays `neon` until the lattice variant exists, so no phase of PM-041 leaves
 * the band defaulting to a treatment that has not been built. F3 flips it.
 */
export const defaultBackdropVariant: BackdropVariant = 'neon';

/** Narrow an unvalidated stored value to a variant, falling back to the default. */
export function toBackdropVariant(value: unknown): BackdropVariant {
  return (backdropVariants as readonly string[]).includes(value as string)
    ? (value as BackdropVariant)
    : defaultBackdropVariant;
}
