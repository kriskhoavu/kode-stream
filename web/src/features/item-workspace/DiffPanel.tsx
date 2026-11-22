import { GitCompare, RotateCcw } from 'lucide-react';
import type { DiffFile } from '../../shared/domain/diff';
import { type DiffMode } from './itemWorkspaceTypes';

export function DiffPanel({ diff, files, mode, selectedPath, selectedFileHasDiff, reverting, onModeChange, onRevertFile }: {
  diff: string;
  files: DiffFile[];
  mode: DiffMode;
  selectedPath: string;
  selectedFileHasDiff: boolean;
  reverting: boolean;
  onModeChange: (mode: DiffMode) => void;
  onRevertFile: () => void;
}) {
  const shownFiles = selectedPath ? files.filter((item) => item.path === selectedPath || item.oldPath === selectedPath) : files;
  const reviewFiles = shownFiles.length > 0 ? shownFiles : files;
  return (
    <section className="diff-panel">
      <header className="diff-toolbar">
        <div className="diff-mode-switch" role="tablist" aria-label="Diff view mode">
          <button type="button" className={mode === 'review' ? 'active' : ''} onClick={() => onModeChange('review')}>Review</button>
          <button type="button" className={mode === 'raw' ? 'active' : ''} onClick={() => onModeChange('raw')}>Git</button>
        </div>
        <div className="diff-actions">
          <span>{files.length} changed file{files.length === 1 ? '' : 's'}</span>
          <button className="danger-action" type="button" disabled={!selectedFileHasDiff || reverting} onClick={onRevertFile}>
            <RotateCcw size={15} /> {reverting ? 'Reverting...' : 'Revert File'}
          </button>
        </div>
      </header>
      {mode === 'raw' && <pre className="diff-view">{diff || 'No local changes.'}</pre>}
      {mode === 'review' && (
        <div className="diff-review">
          {reviewFiles.length === 0 && <div className="document-empty"><GitCompare size={22} /><strong>No local changes</strong><span>The selected plan has no Git diff.</span></div>}
          {reviewFiles.map((item) => (
            <article className={item.path === selectedPath ? 'diff-file active' : 'diff-file'} key={`${item.oldPath ?? item.path}-${item.path}`}>
              <header>
                <strong>{item.path}</strong>
                {item.oldPath && item.oldPath !== item.path && <span>renamed from {item.oldPath}</span>}
                <div>
                  <span className="diff-add">+{item.additions}</span>
                  <span className="diff-delete">-{item.deletions}</span>
                </div>
              </header>
              <div className="diff-lines">
                {item.lines.map((line, index) => (
                  <div className={`diff-line ${line.type}`} key={`${item.path}-${index}`}>
                    <span className="line-number">{line.oldLine ?? ''}</span>
                    <span className="line-number">{line.newLine ?? ''}</span>
                    <code>{line.text || ' '}</code>
                  </div>
                ))}
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
