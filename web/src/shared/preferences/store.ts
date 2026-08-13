const prefix = 'kodeStream.';
const maxPreferenceBytes = 16 * 1024;

export type PreferenceStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

function storage(): PreferenceStorage | undefined {
  try {
    return globalThis.localStorage;
  } catch {
    return undefined;
  }
}

export function preferenceKey(key: string): string {
  return key.startsWith(prefix) ? key : `${prefix}${key}`;
}

export function readPreference<T>(key: string, parse: (value: unknown) => T | undefined, fallback: T, source = storage()): T {
  try {
    const raw = source?.getItem(preferenceKey(key)) ?? source?.getItem(key);
    if (!raw || raw.length > maxPreferenceBytes) return fallback;
    return parse(JSON.parse(raw)) ?? fallback;
  } catch {
    return fallback;
  }
}

export function readStringPreference(key: string, fallback = '', source = storage()): string {
  try {
    const value = source?.getItem(preferenceKey(key)) ?? source?.getItem(key);
    return value && value.length <= maxPreferenceBytes ? value : fallback;
  } catch {
    return fallback;
  }
}

export function writePreference(key: string, value: unknown, source = storage()): boolean {
  try {
    const serialized = typeof value === 'string' ? value : JSON.stringify(value);
    if (serialized.length > maxPreferenceBytes) return false;
    source?.setItem(preferenceKey(key), serialized);
    return Boolean(source);
  } catch {
    return false;
  }
}

export function removePreference(key: string, source = storage()): boolean {
  try {
    source?.removeItem(preferenceKey(key));
    // Retire the legacy key only after a successful write/remove attempt. It remains
    // readable above so existing installations retain their valid preferences.
    source?.removeItem(key);
    return Boolean(source);
  } catch {
    return false;
  }
}
