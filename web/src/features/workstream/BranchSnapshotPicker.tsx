import { useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties, KeyboardEvent as ReactKeyboardEvent } from 'react';
import { Check, ChevronDown, GitBranch, GitCommitHorizontal, Search } from 'lucide-react';

export function BranchSnapshotPicker({
  selectedBranch,
  currentCheckoutBranch,
  sourceMode,
  branches,
  disabled = false,
  ariaLabel,
  listboxLabel,
  onSelect
}: {
  selectedBranch: string;
  currentCheckoutBranch: string;
  sourceMode: 'working_tree' | 'snapshot';
  branches: string[];
  disabled?: boolean;
  ariaLabel: string;
  listboxLabel: string;
  onSelect: (branch: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);
  const pickerRef = useRef<HTMLDivElement | null>(null);
  const optionRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const displayBranch = selectedBranch || currentCheckoutBranch;
  const options = useMemo(() => {
    const query = search.trim().toLowerCase();
    const all = orderBranchOptions(unique([...branches, currentCheckoutBranch, selectedBranch]), currentCheckoutBranch);
    const pinned = all.filter((branch) => isPrimaryBranch(branch) || branch === currentCheckoutBranch);
    const matched = all.filter((branch) => !query || branch.toLowerCase().includes(query));
    return orderBranchOptions(unique([...pinned, ...matched]), currentCheckoutBranch);
  }, [branches, currentCheckoutBranch, search, selectedBranch]);
  const selectBranch = (branch: string) => {
    setOpen(false);
    setSearch('');
    onSelect(branch);
  };

  useEffect(() => {
    if (!open) return;
    const closeOnOutsidePointer = (event: PointerEvent) => {
      if (pickerRef.current?.contains(event.target as Node | null)) return;
      setOpen(false);
      setSearch('');
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      setOpen(false);
      setSearch('');
    };
    document.addEventListener('pointerdown', closeOnOutsidePointer);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('pointerdown', closeOnOutsidePointer);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const selectedIndex = options.findIndex((branch) => branch === displayBranch);
    setActiveIndex(selectedIndex >= 0 ? selectedIndex : 0);
  }, [displayBranch, open, options]);

  useEffect(() => {
    if (!open) return;
    optionRefs.current[activeIndex]?.scrollIntoView?.({ block: 'nearest' });
  }, [activeIndex, open]);

  const handleOptionKeyDown = (event: ReactKeyboardEvent) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setActiveIndex((index) => options.length === 0 ? 0 : (index + 1) % options.length);
      return;
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActiveIndex((index) => options.length === 0 ? 0 : (index - 1 + options.length) % options.length);
      return;
    }
    if (event.key === 'Home') {
      event.preventDefault();
      setActiveIndex(0);
      return;
    }
    if (event.key === 'End') {
      event.preventDefault();
      setActiveIndex(Math.max(0, options.length - 1));
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      const branch = options[activeIndex];
      if (branch) selectBranch(branch);
    }
  };

  return (
    <div
      className={sourceMode === 'snapshot' ? 'branch-context-chip active' : 'branch-context-chip'}
      style={{ '--branch-name-length': displayBranch.length } as CSSProperties & Record<'--branch-name-length', number>}
    >
      <GitBranch size={14} />
      <span>Branch</span>
      <div className="branch-selector-wrap" ref={pickerRef}>
        <button
          type="button"
          className="branch-picker-trigger"
          aria-label={ariaLabel}
          aria-haspopup="listbox"
          aria-expanded={open}
          disabled={disabled}
          onKeyDown={(event) => {
            if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
            event.preventDefault();
            setOpen(true);
            setSearch('');
          }}
          onClick={() => {
            setOpen((value) => !value);
            setSearch('');
          }}
        >
          <span>{displayBranch}</span>
          <ChevronDown size={14} />
        </button>
        {open && (
          <div className="branch-picker-menu">
            <label className="branch-picker-search">
              <Search size={14} />
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                onKeyDown={handleOptionKeyDown}
                placeholder="Search branches..."
                aria-label="Search branches"
                autoFocus
              />
            </label>
            <div className="branch-picker-options" role="listbox" aria-label={listboxLabel} onKeyDown={handleOptionKeyDown}>
              {options.map((branch, index) => (
                <button
                  type="button"
                  role="option"
                  aria-selected={branch === displayBranch}
                  data-active={index === activeIndex ? 'true' : undefined}
                  ref={(element) => { optionRefs.current[index] = element; }}
                  key={branch}
                  title={branch === currentCheckoutBranch ? 'Current checkout branch' : undefined}
                  onMouseEnter={() => setActiveIndex(index)}
                  onClick={() => selectBranch(branch)}
                >
                  <span className="branch-option-checkout-slot">
                    {branch === currentCheckoutBranch && <GitCommitHorizontal className="branch-option-checkout" size={14} aria-hidden="true" />}
                  </span>
                  <span className="branch-option-icon-slot">
                    {branch === displayBranch && <Check className="branch-option-check" size={14} aria-hidden="true" />}
                  </span>
                  <span className="branch-option-label">{branch}</span>
                </button>
              ))}
              {options.length === 0 && <span className="branch-picker-empty">No branches found</span>}
            </div>
          </div>
        )}
      </div>
      {sourceMode === 'snapshot' && <small title={`Snapshot; writes copy into ${currentCheckoutBranch}`}>snapshot {'->'} {currentCheckoutBranch}</small>}
    </div>
  );
}

function isPrimaryBranch(branch: string): boolean {
  const normalized = branch.toLowerCase();
  return normalized === 'main' || normalized === 'master';
}

function orderBranchOptions(values: string[], currentCheckoutBranch: string): string[] {
  return [...values].sort((a, b) => {
    const primaryRank = (branch: string) => {
      const normalized = branch.toLowerCase();
      if (normalized === 'main') return 0;
      if (normalized === 'master') return 1;
      if (branch === currentCheckoutBranch) return 2;
      return 3;
    };
    const rankDiff = primaryRank(a) - primaryRank(b);
    if (rankDiff !== 0) return rankDiff;
    return a.localeCompare(b);
  });
}

function unique(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean))).sort((a, b) => a.localeCompare(b));
}
