

export class ApiError extends Error {
  recoveryHint?: string;
  operationLog?: string;
	code?: string;
	nodeIds?: string[];
	details?: Record<string, string>;
	status?: number;

  constructor(message: string, recoveryHint?: string, operationLog?: string, metadata?: { code?: string; nodeIds?: string[]; details?: Record<string, string>; status?: number }) {
    super(message);
    this.name = 'ApiError';
    this.recoveryHint = recoveryHint;
    this.operationLog = operationLog;
		this.code = metadata?.code;
		this.nodeIds = metadata?.nodeIds;
		this.details = metadata?.details;
		this.status = metadata?.status;
  }
}

export const inFlightRequests = new Map<string, Promise<unknown>>();
export const defaultLocalAPIOrigin = 'http://127.0.0.1:4317';
export const localAPIOriginStorageKey = 'kodeStreamApiOrigin';
export const maxJSONResponseBytes = 2 * 1024 * 1024;
export const maxSSEBufferBytes = 256 * 1024;
export const maxSSETotalBytes = 2 * 1024 * 1024;

export class ApiResponseError extends Error {
  constructor(message: string) { super(message); this.name = 'ApiResponseError'; }
}

export function isExtensionSurface(protocol = globalThis.location?.protocol): boolean {
  return protocol === 'chrome-extension:';
}

export function localAPIOrigin(storage: Pick<Storage, 'getItem'> = localStorage): string {
  const configured = storage.getItem(localAPIOriginStorageKey)?.trim().replace(/\/+$/, '');
  return configured || defaultLocalAPIOrigin;
}

export function apiURL(path: string, extensionSurface = isExtensionSurface()): string {
  if (!path.startsWith('/')) return path;
  if (!extensionSurface) return path;
  return `${localAPIOrigin()}${path}`;
}

export async function request<T>(path: string, options?: RequestInit, dedupe = options?.method === undefined || options.method === 'GET'): Promise<T> {
  const key = dedupe ? `${options?.method ?? 'GET'} ${path} ${options?.body ?? ''}` : '';
  const existing = key ? inFlightRequests.get(key) : undefined;
  if (existing) return existing as Promise<T>;

  const pending = (async () => {
    const res = await fetch(apiURL(path), {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...(options?.headers ?? {})
      }
    });
    const payload = await readJSONResponse(res);
    if (!res.ok) {
      throw new ApiError(stringField(payload, 'error') || stringField(payload, 'message') || `Request failed: ${res.status}`, stringField(payload, 'recoveryHint'), stringField(payload, 'operationLog') || stringField(payload, 'log'), { code: stringField(payload, 'code'), nodeIds: arrayField(payload, 'nodeIds'), details: recordField(payload, 'details'), status: res.status });
    }
    return payload as T;
  })();

  if (key) inFlightRequests.set(key, pending);
  try {
    return await pending;
  } finally {
    if (key && inFlightRequests.get(key) === pending) inFlightRequests.delete(key);
  }
}

export function stringField(value: Record<string, unknown>, key: string): string | undefined { return typeof value[key] === 'string' ? value[key] : undefined; }
export function arrayField(value: Record<string, unknown>, key: string): string[] | undefined { return Array.isArray(value[key]) && value[key].every((entry) => typeof entry === 'string') ? value[key] as string[] : undefined; }
export function recordField(value: Record<string, unknown>, key: string): Record<string, string> | undefined {
  const candidate = value[key];
  return candidate && typeof candidate === 'object' && Object.values(candidate).every((entry) => typeof entry === 'string') ? candidate as Record<string, string> : undefined;
}

export async function readJSONResponse(res: Response): Promise<Record<string, unknown>> {
  // Lightweight test doubles and intentional legacy adapters may expose json()
  // without a stream. Browser Responses always provide headers/body.
  if (!res.body) {
    const json = (res as unknown as { json?: () => Promise<unknown> }).json;
    if (!json) return {};
    const payload = await json();
    return payload && typeof payload === 'object' ? payload as Record<string, unknown> : {};
  }
  const contentType = res.headers?.get?.('content-type')?.toLowerCase() ?? '';
  if (res.status !== 204 && contentType && !contentType.includes('application/json') && !contentType.includes('+json')) {
    throw new ApiResponseError('The server returned an unexpected response format.');
  }
  if (res.status === 204) return {};
  const body = await readResponseText(res, maxJSONResponseBytes);
  if (!body.trim()) return {};
  try {
    const payload: unknown = JSON.parse(body);
    return payload && typeof payload === 'object' ? payload as Record<string, unknown> : {};
  } catch {
    throw new ApiResponseError('The server returned an invalid response.');
  }
}

export async function readResponseText(res: Response, limit: number): Promise<string> {
  if (!res.body) return '';
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let total = 0;
  let result = '';
  while (true) {
    const { value, done } = await reader.read();
    if (done) return result + decoder.decode();
    total += value.byteLength;
    if (total > limit) {
      await reader.cancel();
      throw new ApiResponseError('The server response is too large.');
    }
    result += decoder.decode(value, { stream: true });
  }
}

export function parseSSEMessage(raw: string): { event: string; data: any } | null {
  const lines = raw.split('\n').map((line) => line.trimEnd());
  const eventLine = lines.find((line) => line.startsWith('event: '));
  const dataLines = lines.filter((line) => line.startsWith('data: ')).map((line) => line.slice(6));
  if (!eventLine || dataLines.length === 0) return null;
  const event = eventLine.slice(7).trim();
  const dataText = dataLines.join('\n');
  try {
    return { event, data: JSON.parse(dataText) };
  } catch {
    return { event, data: { chunk: dataText } };
  }
}
