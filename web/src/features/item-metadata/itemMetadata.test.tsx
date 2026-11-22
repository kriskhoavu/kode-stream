import { fireEvent, render, screen } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import type { ItemDetail, ItemMetadataUpdateInput } from '../../lib/types';
import { ItemMetadataEditor, ItemMetadataFields, isMetadataDirty, metadataDraftFrom } from './itemMetadata';

const plan = { title: 'Plan', scope: 'api', identifier: 'PM-1', status: 'draft', owner: undefined, tags: ['a', 'b'] } as unknown as ItemDetail;

describe('item metadata', () => {
	it('starts clean from the item and turns dirty on any change', () => {
		const draft = metadataDraftFrom(plan);
		expect(isMetadataDirty(draft, plan)).toBe(false);
		expect(isMetadataDirty({ ...draft, owner: 'kim' }, plan)).toBe(true);
		expect(isMetadataDirty({ ...draft, tags: ['a'] }, plan)).toBe(true);
		expect(isMetadataDirty(draft, null)).toBe(false);
	});

	it('edits the draft and hides the scope field when asked', () => {
		let latest: ItemMetadataUpdateInput = {};
		function Harness({ showScope }: { showScope: boolean }) {
			const [draft, setDraft] = useState(metadataDraftFrom(plan));
			latest = draft;
			return <ItemMetadataFields draft={draft} onChange={setDraft} showScope={showScope} />;
		}
		const { rerender } = render(<Harness showScope />);
		expect(screen.getAllByRole('textbox')).toHaveLength(5);
		fireEvent.change(screen.getByDisplayValue('a, b'), { target: { value: 'x, , y ' } });
		expect(latest.tags).toEqual(['x', 'y']);
		rerender(<Harness showScope={false} />);
		expect(screen.getAllByRole('textbox')).toHaveLength(4);
	});

	it('shows no form and cannot save a documentation folder', () => {
		const documentation = { ...plan, metadataSource: 'docs' } as unknown as ItemDetail;
		render(<ItemMetadataEditor plan={documentation} draft={metadataDraftFrom(plan)} onChange={() => undefined} dirty saving={false} onSave={() => undefined} />);
		expect(screen.queryAllByRole('textbox')).toHaveLength(0);
		expect(screen.getByRole('button', { name: 'Save Metadata' })).toBeDisabled();
		expect(screen.getByText('a')).toBeInTheDocument();
	});
});
