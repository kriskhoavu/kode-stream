import { useModalDialog } from './overlay';

export function ConfirmDialog({ title, message, confirmLabel, busy, danger, onCancel, onConfirm }: {
  title: string;
  message: string;
  confirmLabel: string;
  busy?: boolean;
  danger?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const dialog = useModalDialog(onCancel, !busy);

  return (
    <div className="confirm-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onCancel(); }}>
      <section ref={dialog.ref} className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby={dialog.titleId}>
        <header>
          <h2 id={dialog.titleId}>{title}</h2>
          <button className="icon-button" type="button" aria-label="Close dialog" disabled={busy} onClick={onCancel}>×</button>
        </header>
        <p>{message}</p>
        <footer>
          <button className="ghost" type="button" disabled={busy} onClick={onCancel}>Cancel</button>
          <button data-autofocus className={danger ? 'danger-confirm' : 'primary'} type="button" disabled={busy} onClick={onConfirm}>{confirmLabel}</button>
        </footer>
      </section>
    </div>
  );
}
