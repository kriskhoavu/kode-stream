/** A fresh key for one user intent, so a retried request is recognised as the same one. */
export function newIdempotencyKey(prefix = 'card'): string {
	return globalThis.crypto?.randomUUID?.() ?? `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}
