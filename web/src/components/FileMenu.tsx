import { useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { useMenuPopover } from './overlay';

export type FileMenuOption = {
  id: string;
  label: string;
};

export function FileMenu({ value, options, onChange }: { value: string; options: FileMenuOption[]; onChange: (id: string) => void | Promise<void> }) {
  const [open, setOpen] = useState(false);
  const menuRef = useMenuPopover(open, () => setOpen(false));
  const selected = options.find((option) => option.id === value);
  const disabled = options.length === 0;

  const selectFile = (id: string) => {
    setOpen(false);
    if (id !== value) void onChange(id);
  };

  return (
    <div className="file-menu-control" ref={menuRef}>
      <button type="button" className="file-menu-trigger" disabled={disabled} onClick={() => setOpen((current) => !current)} aria-haspopup="menu" aria-expanded={open} aria-label="Select document">
        <span>{selected?.label ?? 'No files available'}</span>
        <ChevronDown className={open ? 'file-menu-chevron open' : 'file-menu-chevron'} size={15} aria-hidden="true" />
      </button>
      {open && (
        <div className="file-menu-popover" role="menu" aria-label="Documents">
          {options.map((option) => (
            <button type="button" role="menuitemradio" aria-checked={option.id === value} className={option.id === value ? 'active' : undefined} key={option.id} onClick={() => selectFile(option.id)}>
              {option.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
