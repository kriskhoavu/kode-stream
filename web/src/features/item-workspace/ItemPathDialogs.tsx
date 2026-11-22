import { useState } from 'react';
import type { RefObject } from 'react';
import { ModalLayer } from '../../components/ModalLayer';
import { useModalDialog } from '../../components/overlay';

/**
 * A small form dialog that holds its own value and busy state. `onSubmit`
 * resolves true when the work is done and the dialog may close.
 */
function PathFormDialog({ label, title, detail, field, initial, placeholder, action, busyAction, onCancel, onSubmit }: {
	label: string;
	title: string;
	detail: string;
	field: string;
	initial: string;
	placeholder?: string;
	action: string;
	busyAction: string;
	onCancel: () => void;
	onSubmit: (value: string) => Promise<boolean>;
}) {
	const [value, setValue] = useState(initial);
	const [busy, setBusy] = useState(false);
	const dialog = useModalDialog(onCancel, !busy);
	const submit = async () => {
		setBusy(true);
		try {
			if (await onSubmit(value)) onCancel();
		} finally {
			setBusy(false);
		}
	};
	return (
		<ModalLayer><div className="modal-backdrop" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget && !busy) onCancel(); }}>
			<section ref={dialog.ref as RefObject<HTMLElement>} className="modal-panel" role="dialog" aria-modal="true" aria-label={label}>
				<header><h2>{title}</h2></header>
				<div className="metadata-form">
					<p>{detail}</p>
					<label>{field}<input data-autofocus value={value} onChange={(event) => setValue(event.target.value)} placeholder={placeholder} /></label>
				</div>
				<footer className="modal-actions">
					<button className="ghost" type="button" disabled={busy} onClick={onCancel}>Cancel</button>
					<button className="primary" type="button" disabled={busy || !value.trim()} onClick={() => void submit()}>{busy ? busyAction : action}</button>
				</footer>
			</section>
		</div></ModalLayer>
	);
}

/** New file or folder under the selected folder, by a relative path. */
export function CreateItemPathDialog({ kind, parent, onCancel, onCreate }: { kind: 'file' | 'directory'; parent: string; onCancel: () => void; onCreate: (path: string) => Promise<boolean> }) {
	return <PathFormDialog label={`Create new ${kind}`} title={`New ${kind === 'file' ? 'file' : 'folder'}`} detail={`Parent: ${parent || 'item root'}`} field="Relative path" initial="" placeholder={kind === 'file' ? 'schema.json' : 'api'} action="Create" busyAction="Creating..." onCancel={onCancel} onSubmit={onCreate} />;
}

/** Rename the selected file or folder in place. */
export function RenameItemPathDialog({ node, onCancel, onRename }: { node: { path: string; type: 'file' | 'directory' }; onCancel: () => void; onRename: (name: string) => Promise<boolean> }) {
	return <PathFormDialog label="Rename path" title={`Rename ${node.type}`} detail={`Current: ${node.path}`} field="Name" initial={node.path.split('/').at(-1) ?? ''} action="Rename" busyAction="Renaming..." onCancel={onCancel} onSubmit={onRename} />;
}
