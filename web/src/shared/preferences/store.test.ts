import { describe, expect, it } from 'vitest';
import { preferenceKey, readPreference, readStringPreference, removePreference, writePreference } from './store';

function memory(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial));
  return { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => values.set(key, value), removeItem: (key: string) => values.delete(key), values };
}

describe('preference store', () => {
  it('migrates readable legacy values and writes only its namespaced owner key', () => {
    const source = memory({ theme: 'dark' });
    expect(readStringPreference('theme', 'light', source)).toBe('dark');
    expect(writePreference('theme', 'light', source)).toBe(true);
    expect(source.values.get(preferenceKey('theme'))).toBe('light');
    expect(source.values.get('theme')).toBe('dark');
  });
  it('handles corrupt, oversized, and throwing storage without disrupting the UI', () => {
    const source = memory({ state: '{bad', huge: 'x'.repeat(20 * 1024) });
    expect(readPreference('state', () => 'valid', 'fallback', source)).toBe('fallback');
    expect(readStringPreference('huge', 'fallback', source)).toBe('fallback');
    const broken = { getItem: () => { throw new Error('blocked'); }, setItem: () => { throw new Error('blocked'); }, removeItem: () => { throw new Error('blocked'); } };
    expect(writePreference('state', { enabled: true }, broken)).toBe(false);
    expect(removePreference('state', broken)).toBe(false);
  });
});
