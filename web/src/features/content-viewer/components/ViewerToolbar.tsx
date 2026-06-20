import type { ViewerMode } from '../types';

const modeLabels: Record<ViewerMode, string> = {
  rendered: 'Rendered',
  structured: 'Tree',
  source: 'Source'
};

export function ViewerToolbar({ modes, mode, onChange }: { modes: ViewerMode[]; mode: ViewerMode; onChange: (mode: ViewerMode) => void }) {
  if (modes.length < 2) return null;
  return (
    <div className="viewer-toolbar viewer-mode-toolbar" role="tablist" aria-label="Viewer mode">
      {modes.map((item) => (
        <button
          key={item}
          type="button"
          role="tab"
          aria-selected={mode === item}
          className={mode === item ? 'active' : ''}
          onClick={() => onChange(item)}
        >
          {modeLabels[item]}
        </button>
      ))}
    </div>
  );
}
