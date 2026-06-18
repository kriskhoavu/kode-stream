import { useEffect, useRef, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { editableStatusOrder, statusLabels } from '../lib/api';
import type { ItemStatus } from '../lib/types';

export function StatusMenu({ value, onChange, ariaLabel = 'Change item status' }: { value: ItemStatus; onChange: (status: ItemStatus) => void; ariaLabel?: string }) {
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;
    const closeOnOutsideClick = (event: PointerEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('pointerdown', closeOnOutsideClick);
    return () => document.removeEventListener('pointerdown', closeOnOutsideClick);
  }, [open]);

  const selectStatus = (status: ItemStatus) => {
    setOpen(false);
    if (status !== value) onChange(status);
  };

  return (
    <div className="status-move-control" ref={menuRef} onClick={(event) => event.stopPropagation()}>
      <button type="button" className="status-move-trigger" onClick={() => setOpen((current) => !current)} aria-expanded={open} aria-label={ariaLabel}>
        <span>{statusLabels[value]}</span>
        <ChevronDown className={open ? 'status-move-chevron open' : 'status-move-chevron'} size={15} aria-hidden="true" />
      </button>
      {open && (
        <div className="status-move-popover">
          {editableStatusOrder.map((status) => (
            <button type="button" className={status === value ? 'active' : undefined} key={status} onClick={() => selectStatus(status)}>
              {statusLabels[status]}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
