import { useEffect, useRef, useState } from 'react';
import { api } from '../../lib/api';
import type { ExplorerTreeMode, WorkspaceContentSearchResult } from '../../lib/types';

type ContentSearchTarget =
	| { kind: 'item'; itemId: string }
	| { kind: 'explorer'; mode: ExplorerTreeMode; workspaceId?: string; includeIgnored: boolean };

export function useContentSearch(target: ContentSearchTarget, debounceMs = 250) {
	const [query, setQuery] = useState('');
	const [caseSensitive, setCaseSensitive] = useState(false);
	const [results, setResults] = useState<WorkspaceContentSearchResult[]>([]);
	const [truncated, setTruncated] = useState(false);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const requestId = useRef(0);
	const targetKey = target.kind === 'item'
		? `item:${target.itemId}`
		: `explorer:${target.mode}:${target.workspaceId ?? '*'}:${target.includeIgnored}`;

	useEffect(() => {
		const normalized = query.trim();
		const id = ++requestId.current;
		if (normalized.length < 2) {
			setResults([]);
			setTruncated(false);
			setLoading(false);
			setError('');
			return;
		}
		setLoading(true);
		const timer = window.setTimeout(() => {
			const request = target.kind === 'item'
				? api.searchItemContent(target.itemId, { q: normalized, caseSensitive })
				: api.searchWorkspaceContent({ q: normalized, mode: target.mode, workspaceId: target.workspaceId, includeIgnored: target.includeIgnored, caseSensitive });
			request.then((response) => {
				if (requestId.current !== id) return;
				setResults(response.results);
				setTruncated(response.truncated);
				setError('');
			}).catch((caught: unknown) => {
				if (requestId.current !== id) return;
				setResults([]);
				setTruncated(false);
				setError(caught instanceof Error ? caught.message : 'Content search failed');
			}).finally(() => {
				if (requestId.current === id) setLoading(false);
			});
		}, debounceMs);
		return () => window.clearTimeout(timer);
	}, [caseSensitive, debounceMs, query, targetKey]);

	const clear = () => setQuery('');
	return { query, setQuery, caseSensitive, setCaseSensitive, results, truncated, loading, error, clear };
}
