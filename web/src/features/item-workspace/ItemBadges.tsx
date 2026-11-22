import { FileText } from 'lucide-react';
import { statusLabels } from '../../shared/api';
import type { ItemDetail } from '../../lib/types';

export function EmptyDocumentState({ hasFiles }: { hasFiles: boolean }) {
  return (
    <div className="document-empty">
      <FileText size={22} />
      <strong>{hasFiles ? 'Select a file' : 'No files found'}</strong>
      <span>{hasFiles ? 'Choose a file from the explorer to preview its content.' : 'This item folder does not contain any readable files yet.'}</span>
    </div>
  );
}

export function StatusBadge({ status }: { status: ItemDetail['status'] }) {
  return <span className={`status-badge ${status}`}>{statusLabel(status)}</span>;
}

export function statusLabel(status: ItemDetail['status']): string {
  return statusLabels[status] ?? status;
}
