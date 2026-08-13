import { useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { editableStatusOrder, statusLabels } from '../shared/api';
import type { ItemStatus } from '../lib/types';
import { useMenuPopover } from './overlay';

export function StatusMenu({ value, onChange, ariaLabel = 'Change item status' }: { value: ItemStatus; onChange: (status: ItemStatus) => void; ariaLabel?: string }) {
  const [open, setOpen] = useState(false);
  const menuRef = useMenuPopover(open, () => setOpen(false));

  const selectStatus = (status: ItemStatus) => {
    setOpen(false);
    if (status !== value) onChange(status);
  };

  return (
    <div className="status-move-control" ref={menuRef} onPointerDown={(event) => event.stopPropagation()} onClick={(event) => event.stopPropagation()}>
      <button type="button" className="status-move-trigger" onClick={() => setOpen((current) => !current)} aria-haspopup="menu" aria-expanded={open} aria-label={ariaLabel}>
        <span>{statusLabels[value]}</span>
        <ChevronDown className={open ? 'status-move-chevron open' : 'status-move-chevron'} size={15} aria-hidden="true" />
      </button>
      {open && (
        <div className="status-move-popover" role="menu" aria-label="Item status">
          {editableStatusOrder.map((status) => (
            <button type="button" role="menuitemradio" aria-checked={status === value} className={status === value ? 'active' : undefined} key={status} onClick={() => selectStatus(status)}>
              {statusLabels[status]}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
