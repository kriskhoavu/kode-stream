import { afterEach, describe, expect, it } from 'vitest';
import { preferenceKey } from '../../shared/preferences/store';
import { defaultAppSettings, loadAppSettings } from './appSettings';
import { backdropVariants, defaultBackdropVariant } from '../../components/HeaderBackdrop';

const storageKey = preferenceKey('planManager.appSettings');

function store(settings: unknown) {
  localStorage.setItem(storageKey, JSON.stringify(settings));
}

afterEach(() => localStorage.clear());

describe('app settings', () => {
  it('defaults the backdrop to the built-in default when nothing is stored', () => {
    expect(loadAppSettings().headerBackdrop).toBe(defaultBackdropVariant);
  });

  it('round-trips every valid variant', () => {
    for (const variant of backdropVariants) {
      store({ headerBackdrop: variant });
      expect(loadAppSettings().headerBackdrop).toBe(variant);
    }
  });

  /*
   * Every install that predates PM-041 reaches this code with the field
   * missing, so the fallback is the part worth pinning: silently yielding
   * undefined would blank the band with no error anyone would notice.
   */
  it('falls back to the default when the stored variant is absent, unknown or not a string', () => {
    for (const stored of [undefined, 'aurora', 42, null, { name: 'neon' }, ['neon']]) {
      store({ headerBackdrop: stored });
      expect(loadAppSettings().headerBackdrop).toBe(defaultBackdropVariant);
    }
  });

  it('adds the backdrop without disturbing the statuses stored beside it', () => {
    store({ visibleWorkstreamStatuses: ['in_progress'], headerBackdrop: 'neon' });
    const settings = loadAppSettings();

    expect(settings.visibleWorkstreamStatuses).toEqual(['in_progress']);
    expect(settings.headerBackdrop).toBe('neon');
  });

  it('survives a corrupt settings blob', () => {
    localStorage.setItem(storageKey, '{not json');
    expect(loadAppSettings()).toEqual(defaultAppSettings);
  });
});
