import { useEffect, useId, useRef } from 'react';

const focusable = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/** Shared modal behavior: focus starts inside, tab stays inside, Escape closes, and focus returns to the opener. */
export function useModalDialog(onClose: () => void, canDismiss = true, open = true) {
  const ref = useRef<HTMLElement | null>(null);
  const titleId = useId();
  const closeRef = useRef(onClose);
  const dismissRef = useRef(canDismiss);
  closeRef.current = onClose;
  dismissRef.current = canDismiss;
  useEffect(() => {
    if (!open) return;
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const initial = ref.current?.querySelector<HTMLElement>('[data-autofocus]') ?? ref.current?.querySelector<HTMLElement>(focusable);
    initial?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { if (dismissRef.current) { event.preventDefault(); closeRef.current(); } return; }
      if (event.key !== 'Tab' || !ref.current) return;
      const controls = [...ref.current.querySelectorAll<HTMLElement>(focusable)];
      if (!controls.length) return;
      const index = controls.indexOf(document.activeElement as HTMLElement);
      if (event.shiftKey && (index <= 0)) { event.preventDefault(); controls.at(-1)?.focus(); }
      else if (!event.shiftKey && index === controls.length - 1) { event.preventDefault(); controls[0].focus(); }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => { window.removeEventListener('keydown', onKeyDown); previous?.focus(); };
  }, [open]);
  return { ref, titleId };
}

export function useMenuPopover(open: boolean, close: () => void, onArrow?: (direction: 1 | -1) => void) {
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!open) return;
    const outside = (event: PointerEvent) => { if (ref.current && !ref.current.contains(event.target as Node)) close(); };
    const keys = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); close(); }
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        event.preventDefault();
        const options = [...(ref.current?.querySelectorAll<HTMLElement>('button:not([disabled]), [role="menuitem"]:not([aria-disabled="true"])') ?? [])];
        if (!options.length) return;
        const direction = event.key === 'ArrowDown' ? 1 : -1;
        const index = options.indexOf(document.activeElement as HTMLElement);
        options[(index + direction + options.length) % options.length].focus();
        onArrow?.(direction as 1 | -1);
      }
    };
    document.addEventListener('pointerdown', outside);
    window.addEventListener('keydown', keys);
    return () => { document.removeEventListener('pointerdown', outside); window.removeEventListener('keydown', keys); };
  }, [close, onArrow, open]);
  return ref;
}
