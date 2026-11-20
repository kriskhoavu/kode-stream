import { useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { editableStatusOrder, statusLabels } from '../shared/api';
import type { ItemStatus } from '../lib/types';
import { useMenuPopover } from './overlay';

/**
 * Moves an item to another status. `toned` draws it as a pill in the status's
 * colour, with a dot before each choice, where a row has room to show it.
 */
export function StatusMenu({ value, onChange, ariaLabel = 'Change item status', toned = false }: { value: ItemStatus; onChange: (status: ItemStatus) => void; ariaLabel?: string; toned?: boolean }) {
  const [open, setOpen] = useState(false);
  const menuRef = useMenuPopover(open, () => setOpen(false));

  const selectStatus = (status: ItemStatus) => {
    setOpen(false);
    if (status !== value) onChange(status);
  };

  return (
    <div className={toned ? `status-move-control toned plan-status-${value}` : 'status-move-control'} ref={menuRef} onPointerDown={(event) => event.stopPropagation()} onClick={(event) => event.stopPropagation()}>
      <button type="button" className="status-move-trigger" onClick={() => setOpen((current) => !current)} aria-haspopup="menu" aria-expanded={open} aria-label={ariaLabel}>
        {toned && <span className="plan-status-dot" aria-hidden="true" />}
        <span>{statusLabels[value]}</span>
        <ChevronDown className={open ? 'status-move-chevron open' : 'status-move-chevron'} size={15} aria-hidden="true" />
      </button>
      {open && (
        <div className="status-move-popover" role="menu" aria-label="Item status">
          {editableStatusOrder.map((status) => (
            <button type="button" role="menuitemradio" aria-checked={status === value} className={[status === value ? 'active' : '', toned ? `plan-status-${status}` : ''].filter(Boolean).join(' ') || undefined} key={status} onClick={() => selectStatus(status)}>
              {toned && <span className="plan-status-dot" aria-hidden="true" />}
              {statusLabels[status]}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
