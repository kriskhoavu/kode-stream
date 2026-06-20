import { useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';

export type StructuredValue = null | boolean | number | string | StructuredValue[] | { [key: string]: StructuredValue };

export function TreeNode({ name, value, depth = 0 }: { name?: string; value: StructuredValue; depth?: number }) {
  const expandable = Array.isArray(value) || isRecord(value);
  const [expanded, setExpanded] = useState(depth < 2);
  const entries = expandable ? Object.entries(value) : [];
  const label = name === undefined ? 'root' : name;

  if (!expandable) {
    return (
      <div className="structured-tree-scalar" style={{ '--tree-depth': depth } as React.CSSProperties}>
        {name !== undefined && <span className="structured-tree-key">{name}:</span>}
        <span className={`structured-tree-value ${valueType(value)}`}>{formatScalar(value)}</span>
      </div>
    );
  }

  return (
    <div className="structured-tree-node">
      <button
        className="structured-tree-toggle"
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((current) => !current)}
        style={{ '--tree-depth': depth } as React.CSSProperties}
      >
        {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <span className="structured-tree-key">{label}</span>
        <span className="structured-tree-summary">{Array.isArray(value) ? `[${entries.length}]` : `{${entries.length}}`}</span>
      </button>
      {expanded && (
        <div role="group">
          {entries.map(([key, child]) => <TreeNode key={key} name={key} value={child} depth={depth + 1} />)}
        </div>
      )}
    </div>
  );
}

function isRecord(value: StructuredValue): value is { [key: string]: StructuredValue } {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function valueType(value: StructuredValue): string {
  if (value === null) return 'null';
  return typeof value;
}

function formatScalar(value: StructuredValue): string {
  if (typeof value === 'string') return JSON.stringify(value);
  if (value === null) return 'null';
  return String(value);
}
