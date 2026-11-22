import type { DragEvent } from 'react';
import { Filter } from 'lucide-react';
import { statusLabels } from '../../shared/api';
import type { ItemStatus } from '../../lib/types';
import { isDropStatus } from './dragAndDrop';

export function WorkstreamColumn({ status, label, itemCount, loading, dragActive, dragTargetStatus, onDragOver, onDragLeave, onDrop, onCreate, children }: {
  status: ItemStatus;
  /** The column's name where it is not the status's own. */
  label?: string;
  itemCount: number;
  loading: boolean;
  dragActive: boolean;
  dragTargetStatus: ItemStatus | '';
  onDragOver: (event: DragEvent<HTMLElement>, status: ItemStatus) => void;
  onDragLeave: (event: DragEvent<HTMLElement>, status: ItemStatus) => void;
  onDrop: (event: DragEvent<HTMLElement>, status: ItemStatus) => void;
  onCreate?: () => void;
  children: React.ReactNode;
}) {
  const droppable = dragActive && isDropStatus(status);
  const classes = ['workstream-column', status];
  if (dragActive && droppable) classes.push('drop-enabled');
  if (dragTargetStatus === status && droppable) classes.push('drop-target');

  return (
    <div
      className={classes.join(' ')}
      data-status={status}
      onDragOver={(event) => onDragOver(event, status)}
      onDragLeave={(event) => onDragLeave(event, status)}
      onDrop={(event) => onDrop(event, status)}
    >
      <header>
        <h2>{label ?? statusLabels[status]}</h2>
        <span>{itemCount}</span>
        <Filter size={14} />
      </header>
      <div className="card-stack">
        {loading && Array.from({ length: 3 }).map((_, index) => <div className="plan-card skeleton" key={index} />)}
        {children}
        {!loading && itemCount === 0 && <div className="column-empty">No items</div>}
      </div>
      {onCreate && <button className="new-plan-column-button" type="button" onClick={onCreate}>+ New item</button>}
    </div>
  );
}
