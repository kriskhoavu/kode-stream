import type { Dispatch, SetStateAction } from 'react';
import { StatusMenu } from '../../components/StatusMenu';
import type { ItemDetail, ItemMetadataUpdateInput, ItemStatus } from '../../lib/types';
import { isDocumentationMetadataSource, labels } from '../../lib/vocabulary';

/** The editable metadata of an item, as the form starts from it. */
export function metadataDraftFrom(plan: ItemDetail): ItemMetadataUpdateInput {
	return { title: plan.title, scope: plan.scope, identifier: plan.identifier, status: plan.status, owner: plan.owner ?? '', tags: plan.tags };
}

/** Whether the draft differs from the item it was made from. */
export function isMetadataDirty(draft: ItemMetadataUpdateInput, plan: ItemDetail | null): boolean {
	if (!plan) return false;
	return (draft.title ?? '') !== (plan.title ?? '')
		|| (draft.scope ?? '') !== (plan.scope ?? '')
		|| (draft.identifier ?? '') !== (plan.identifier ?? '')
		|| (draft.status ?? '') !== (plan.status ?? '')
		|| (draft.owner ?? '') !== (plan.owner ?? '')
		|| (draft.tags ?? []).join('\n') !== (plan.tags ?? []).join('\n');
}

/** The item metadata form. The drawer leaves out the scope field; the item page shows it. */
export function ItemMetadataFields({ draft, onChange, showScope = true, className = 'metadata-form' }: {
	draft: ItemMetadataUpdateInput;
	onChange: Dispatch<SetStateAction<ItemMetadataUpdateInput>>;
	showScope?: boolean;
	className?: string;
}) {
	const set = (patch: ItemMetadataUpdateInput) => onChange((current) => ({ ...current, ...patch }));
	return (
		<div className={className}>
			<label>Title<input value={draft.title ?? ''} onChange={(event) => set({ title: event.target.value })} /></label>
			{showScope && <label>{labels.scope}<input value={draft.scope ?? ''} onChange={(event) => set({ scope: event.target.value })} /></label>}
			<label>{labels.identifier}<input value={draft.identifier ?? ''} onChange={(event) => set({ identifier: event.target.value })} /></label>
			<label>Status<StatusMenu value={(draft.status ?? 'draft') as ItemStatus} onChange={(status) => set({ status })} /></label>
			<label>Owner<input value={draft.owner ?? ''} onChange={(event) => set({ owner: event.target.value })} /></label>
			<label>Tags<input value={(draft.tags ?? []).join(', ')} onChange={(event) => set({ tags: event.target.value.split(',').map((tag) => tag.trim()).filter(Boolean) })} /></label>
		</div>
	);
}

/**
 * The editable part of an item's Info tab: the form, its Save button and the item's tags. A
 * documentation folder has no structured metadata, so it shows no form and cannot save.
 */
export function ItemMetadataEditor({ plan, draft, onChange, dirty, saving, onSave, showScope = true, formClassName }: {
	plan: ItemDetail | null;
	draft: ItemMetadataUpdateInput;
	onChange: Dispatch<SetStateAction<ItemMetadataUpdateInput>>;
	dirty: boolean;
	saving: boolean;
	onSave: () => void;
	showScope?: boolean;
	formClassName?: string;
}) {
	const documentation = isDocumentationMetadataSource(plan?.metadataSource);
	return (
		<>
			{!documentation && <ItemMetadataFields draft={draft} onChange={onChange} showScope={showScope} className={formClassName} />}
			<div className="workspace-actions">
				<button className="save-action save-metadata-action" type="button" disabled={!dirty || saving || documentation} onClick={onSave}>{saving ? 'Saving...' : 'Save Metadata'}</button>
			</div>
			{(plan?.tags?.length ?? 0) > 0 && <div className="tags">{plan?.tags.map((tag) => <span key={tag}>{tag}</span>)}</div>}
		</>
	);
}
