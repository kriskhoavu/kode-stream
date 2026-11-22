import { X } from 'lucide-react';
import type { WorkspaceConfig, ScanResult } from '../../lib/types';

export type WorkspaceNotice = {
  tone: 'success' | 'error' | 'info';
  title: string;
  details?: string[];
};
export function WorkspaceNoticePanel({ notice, onDismiss }: { notice: WorkspaceNotice; onDismiss: () => void }) {
  return (
    <section className={`workspace-notice ${notice.tone}`} role="status" aria-live="polite">
      <div>
        <strong>{notice.title}</strong>
        {notice.details && notice.details.length > 0 && (
          <ul>
            {notice.details.map((detail, index) => <li key={`${detail}-${index}`}>{detail}</li>)}
          </ul>
        )}
      </div>
      <button className="icon-button" type="button" onClick={onDismiss} aria-label="Dismiss notification">
        <X size={15} />
      </button>
    </section>
  );
}

export function scanNotice(repo: WorkspaceConfig, result: ScanResult, title = 'Workspace scanned'): WorkspaceNotice {
  const warnings = scanWarnings(result);
  return {
    tone: warnings.length > 0 ? 'info' : 'success',
    title,
    details: [
      scanSummary(repo, result),
      ...warnings.slice(0, 3).map((warning) => `Warning${warning.itemPath ? ` (${warning.itemPath})` : ''}: ${warning.message}`)
    ]
  };
}

export function sourceSettingsNotice(title: string, repo: WorkspaceConfig, scan?: ScanResult): WorkspaceNotice {
  return scan ? scanNotice(repo, scan, title) : { tone: 'success', title, details: [repo.name] };
}

export function scanSummary(repo: WorkspaceConfig, result: ScanResult): string {
  const warningCount = scanWarnings(result).length;
  return `${repo.name}: ${result.itemCount} item${result.itemCount === 1 ? '' : 's'} indexed at ${formatScanTime(result.scannedAt)}${warningCount > 0 ? ` with ${warningCount} warning${warningCount === 1 ? '' : 's'}` : ''}.`;
}

export function scanWarnings(result: ScanResult): ScanResult['warnings'] {
  return Array.isArray(result.warnings) ? result.warnings : [];
}

export function formatScanTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}
