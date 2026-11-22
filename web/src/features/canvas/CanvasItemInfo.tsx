import { useEffect, useRef, useState } from 'react';
import { ItemMetadataFields, isMetadataDirty, metadataDraftFrom } from '../item-metadata/itemMetadata';
import { api } from '../../shared/api';
import type { CanvasNode, ItemDetail, ItemMetadataUpdateInput } from '../../lib/types';

export function CanvasItemInfo({ itemId, node, workspace, onSaved }: { itemId: string; node: CanvasNode; workspace?: CanvasNode['workspace']; onSaved: () => void }) {
	const [detail, setDetail] = useState<ItemDetail>();
	const [draft, setDraft] = useState<ItemMetadataUpdateInput>({});
	const [saving, setSaving] = useState(false);
	const [error, setError] = useState('');
	const generationRef = useRef(0);
	useEffect(() => {
		const generation = ++generationRef.current;
		const controller = new AbortController();
		api.item(itemId, controller.signal).then((next) => {
			if (controller.signal.aborted) return;
			setDetail(next);
			setDraft(metadataDraftFrom(next));
		}).catch((caught) => !controller.signal.aborted && setError(caught instanceof Error ? caught.message : 'Item details could not be loaded.'));
		return () => {
			controller.abort();
			if (generationRef.current === generation) generationRef.current += 1;
		};
	}, [itemId]);
	const canvasPlan = node.plan;
	if (!canvasPlan) return null;
	const dirty = isMetadataDirty(draft, detail ?? null);
	const save = async () => {
		const generation = generationRef.current;
		const isCurrent = () => generationRef.current === generation;
		setSaving(true); setError('');
		try {
			const result = await api.saveMetadata(itemId, { ...draft, expectedRevision: detail?.metadataRevision });
			if (!isCurrent()) return;
			setDetail(result.item);
			setDraft(metadataDraftFrom(result.item));
			onSaved();
		} catch (caught) { if (isCurrent()) setError(caught instanceof Error ? caught.message : 'Metadata could not be saved.'); } finally { if (isCurrent()) setSaving(false); }
	};
	const loaded = detail;
	return <div className="canvas-item-tab-content"><dl><dt>Workspace</dt><dd>{loaded?.workspaceName || workspace?.name || 'Unavailable'}</dd><dt>Source</dt><dd>{loaded?.scope || canvasPlan.service || 'Other'}</dd><dt>Item</dt><dd>{loaded?.identifier || canvasPlan.identifier}</dd><dt>Branch</dt><dd>{loaded?.branch || canvasPlan.branch || node.entityRef.branchKey || 'Unavailable'}</dd><dt>Status</dt><dd><span className={`canvas-item-status status-${loaded?.status || canvasPlan.status}`}>{(loaded?.status || canvasPlan.status).replaceAll('_', ' ')}</span></dd><dt>Metadata</dt><dd>{loaded?.metadataSource || 'Item'}</dd><dt>Author</dt><dd>{loaded?.author || loaded?.owner || 'Unknown'}</dd><dt>Files</dt><dd>{loaded?.counts.files ?? '—'}</dd></dl>{loaded && <ItemMetadataFields draft={draft} onChange={setDraft} className="metadata-form canvas-metadata-form"><button className="save-action save-metadata-action" type="button" disabled={!dirty || saving} onClick={() => void save()}>{saving ? 'Saving...' : 'Save Metadata'}</button><div className="tags">{loaded.tags.map((tag) => <span key={tag}>{tag}</span>)}</div></ItemMetadataFields>}{error && <p className="error" role="alert">{error}</p>}</div>;
}
