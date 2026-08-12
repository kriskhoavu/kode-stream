import type { KeyboardEvent, RefObject } from 'react';
import { Search } from 'lucide-react';
import type { WorkspaceContentSearchResult, WorkspacePathSearchResult } from '../../lib/types';

const maxVisibleResults = 20;
const maxVisibleSnippetCharacters = 120;

export function ContentSearchInput({ query, onQueryChange, caseSensitive = false, onCaseSensitiveChange, label }: {
	query: string;
	onQueryChange: (query: string) => void;
	caseSensitive?: boolean;
	onCaseSensitiveChange?: (value: boolean) => void;
	label: string;
}) {
	return <div className="content-search-input" role="search">
		<label><Search size={15} /><input aria-label={label} value={query} onChange={(event) => onQueryChange(event.target.value)} placeholder={label} /></label>
		{onCaseSensitiveChange && <button type="button" aria-label="Match case" aria-pressed={caseSensitive} className={caseSensitive ? 'active' : ''} onClick={() => onCaseSensitiveChange(!caseSensitive)}>Aa</button>}
	</div>;
}

export function ContentSearchResults({ query, results, truncated, loading, error, activeIndex, onActiveIndex, onOpen, onEscape, treeRef, showWorkspaceContext = true }: {
	query: string;
	results: WorkspaceContentSearchResult[];
	truncated: boolean;
	loading: boolean;
	error: string;
	activeIndex: number;
	onActiveIndex: (index: number) => void;
	onOpen: (result: WorkspaceContentSearchResult) => void;
	onEscape: () => void;
	treeRef?: RefObject<HTMLElement | null>;
	showWorkspaceContext?: boolean;
}) {
	const visibleResults = results.slice(0, maxVisibleResults);
	const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
		if (event.key === 'Escape') {
			event.preventDefault();
			onEscape();
			treeRef?.current?.focus();
			return;
		}
		if (!visibleResults.length) return;
		if (event.key === 'ArrowDown') { event.preventDefault(); onActiveIndex(Math.min(activeIndex + 1, visibleResults.length - 1)); }
		if (event.key === 'ArrowUp') { event.preventDefault(); onActiveIndex(Math.max(activeIndex - 1, 0)); }
		if (event.key === 'Enter') { event.preventDefault(); onOpen(visibleResults[activeIndex] ?? visibleResults[0]); }
	};
	return <div className="content-search-results" role="listbox" aria-label="Content search results" tabIndex={0} onKeyDown={onKeyDown}>
		<div className="content-search-live" aria-live="polite">{loading ? 'Searching file contents…' : `${results.length} content matches`}</div>
		{error && <p className="content-search-message error">{error}</p>}
		{!loading && !error && results.length === 0 && <p className="content-search-message">No content matches.</p>}
		{visibleResults.map((result, index) => <ContentSearchResultRow key={result.id} result={result} query={query} active={index === activeIndex} showWorkspaceContext={showWorkspaceContext} onActive={() => onActiveIndex(index)} onOpen={() => onOpen(result)} />)}
		{results.length > maxVisibleResults && <p className="content-search-message">Showing the first {maxVisibleResults} of {results.length} matches. Refine the query to narrow the list.</p>}
		{truncated && results.length <= maxVisibleResults && <p className="content-search-message">More matches exist. Refine the query.</p>}
	</div>;
}

export function ContentSearchResultRow({ result, query, active, onActive, onOpen, showWorkspaceContext = true }: {
	result: WorkspaceContentSearchResult;
	query: string;
	active: boolean;
	onActive: () => void;
	onOpen: () => void;
	showWorkspaceContext?: boolean;
}) {
	return <button type="button" role="option" aria-label={`${result.name}, line ${result.lineNumber}, columns ${result.columnStart} to ${result.columnEnd}`} aria-selected={active} className={`content-search-result${active ? ' active' : ''}`} title={`${result.workspaceName} · ${result.path}`} onMouseEnter={onActive} onClick={onOpen}>
		<span className="content-search-result-header"><strong>{result.name}</strong><span className="content-search-line">L{result.lineNumber}</span></span>
		<span className="content-search-path">{showWorkspaceContext && <>{result.workspaceName} · </>}{compactParentPath(result.path)}</span>
		<span className="content-search-snippet">{highlightSnippet(compactSnippet(result.snippet, query), query, result.id)}</span>
	</button>;
}

export function PathSearchResultRow({ result, query, active, onActive, onOpen }: {
	result: WorkspacePathSearchResult;
	query: string;
	active: boolean;
	onActive: () => void;
	onOpen: () => void;
}) {
	const kind = result.type === 'directory' ? 'Folder' : 'File';
	return <button type="button" role="option" aria-label={`${result.name}, ${kind.toLocaleLowerCase()}, ${result.context || 'workspace root'}`} aria-selected={active} className={`content-search-result${active ? ' active' : ''}`} title={`${result.workspaceName} · ${result.path}`} onMouseEnter={onActive} onClick={onOpen}>
		<span className="content-search-result-header"><strong>{highlightSnippet(result.name, query, result.id)}</strong><span className="content-search-line content-search-kind">{kind}</span></span>
		<span className="content-search-path">{result.workspaceName} · {result.context || 'Workspace root'}</span>
	</button>;
}

function compactParentPath(path: string): string {
	const segments = path.split('/').filter(Boolean);
	const parents = segments.slice(0, -1);
	return parents.slice(-2).join(' / ') || 'Workspace root';
}

function compactSnippet(snippet: string, query: string): string {
	if (snippet.length <= maxVisibleSnippetCharacters) return snippet;
	const matchIndex = snippet.toLocaleLowerCase().indexOf(query.toLocaleLowerCase());
	const start = Math.max(0, Math.min(snippet.length - maxVisibleSnippetCharacters, matchIndex - 40));
	const end = Math.min(snippet.length, start + maxVisibleSnippetCharacters);
	return `${start > 0 ? '…' : ''}${snippet.slice(start, end)}${end < snippet.length ? '…' : ''}`;
}

function highlightSnippet(snippet: string, query: string, key: string) {
	const index = snippet.toLocaleLowerCase().indexOf(query.toLocaleLowerCase());
	if (!query || index < 0) return snippet;
	return <>{snippet.slice(0, index)}<mark key={key}>{snippet.slice(index, index + query.length)}</mark>{snippet.slice(index + query.length)}</>;
}
