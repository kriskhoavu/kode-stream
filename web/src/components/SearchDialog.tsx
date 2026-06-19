import { Clock3, FileText, Search, X } from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { api } from '../lib/api';
import type { RecentItem, SearchResult } from '../lib/types';
import { useGlobalSearch } from '../features/search/hooks';

export function SearchDialog({ workspaceId, onClose, onNavigate }: { workspaceId?: string; onClose: () => void; onNavigate: (route: string) => void }) {
  const [allWorkspaces, setAllWorkspaces] = useState(false);
  const [recents, setRecents] = useState<RecentItem[]>([]);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const search = useGlobalSearch({
    workspaceId,
    allWorkspaces,
    onNavigate: (route) => {
      onNavigate(route);
      onClose();
    }
  });

  useEffect(() => {
    inputRef.current?.focus();
    api.recentItems(8).then(setRecents).catch(() => setRecents([]));
  }, []);

  const groups = useMemo(() => groupResults(search.results), [search.results]);
  return (
    <div className="search-backdrop" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className="search-dialog" role="dialog" aria-modal="true" aria-label="Global search">
        <header>
          <label className="global-search-input">
            <Search size={18} />
            <input ref={inputRef} value={search.query} onChange={(event) => search.setQuery(event.target.value)} onKeyDown={search.onKeyDown} placeholder="Search items across workspaces" />
          </label>
          <button className="icon-button" type="button" onClick={onClose} aria-label="Close search"><X size={17} /></button>
        </header>
        <div className="search-scope" role="group" aria-label="Search scope">
          <button className={!allWorkspaces ? 'active' : ''} type="button" onClick={() => setAllWorkspaces(false)}>Current workspace</button>
          <button className={allWorkspaces ? 'active' : ''} type="button" onClick={() => setAllWorkspaces(true)}>All workspaces</button>
        </div>
        <div className="search-results" aria-busy={search.loading}>
          {search.loading && <span className="search-empty">Searching...</span>}
          {!search.loading && search.error && <span className="error">{search.error}</span>}
          {!search.loading && search.query && !search.error && search.results.length === 0 && <span className="search-empty">No matching items.</span>}
          {!search.query && <RecentResults recents={recents} onSelect={onNavigate} />}
          {groups.map(([type, results]) => (
            <section className="search-result-group" key={type}>
              <h3>{resultGroupLabel(type)}</h3>
              {results.map((result) => {
                const index = search.results.indexOf(result);
                return (
                  <button className={index === search.selectedIndex ? 'search-result active' : 'search-result'} key={`${result.type}:${result.id}`} type="button" onMouseEnter={() => search.setSelectedIndex(index)} onClick={() => search.selectResult(result)}>
                    <FileText size={16} />
                    <span><strong>{result.title}</strong><small>{result.subtitle || result.context}</small></span>
                  </button>
                );
              })}
            </section>
          ))}
        </div>
      </section>
    </div>
  );
}

function RecentResults({ recents, onSelect }: { recents: RecentItem[]; onSelect: (route: string) => void }) {
  if (recents.length === 0) return <span className="search-empty">No recent items.</span>;
  return (
    <section className="search-result-group">
      <h3>Recent items</h3>
      {recents.map((item) => (
        <button className="search-result" key={item.itemId} type="button" onClick={() => onSelect(item.route)}>
          <Clock3 size={16} />
          <span><strong>{item.title}</strong><small>{item.subtitle}</small></span>
        </button>
      ))}
    </section>
  );
}

function groupResults(results: SearchResult[]) {
  const groups = new Map<string, SearchResult[]>();
  for (const result of results) groups.set(result.type, [...(groups.get(result.type) ?? []), result]);
  return [...groups.entries()];
}

function resultGroupLabel(type: string) {
  return type === 'savedFilter' ? 'Saved filters' : `${type.charAt(0).toUpperCase()}${type.slice(1)}s`;
}
