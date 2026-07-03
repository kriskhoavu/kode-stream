import '@testing-library/jest-dom/vitest';

const storage = new Map<string, string>();
const testLocalStorage: Storage = {
	get length() { return storage.size; },
	clear: () => storage.clear(),
	getItem: (key) => storage.get(key) ?? null,
	key: (index) => [...storage.keys()][index] ?? null,
	removeItem: (key) => { storage.delete(key); },
	setItem: (key, value) => { storage.set(key, String(value)); }
};
Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: testLocalStorage });
