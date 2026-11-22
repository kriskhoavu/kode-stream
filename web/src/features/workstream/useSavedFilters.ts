import { useEffect, useState } from 'react';
import type { SavedFilter } from '../../lib/types';
import { api } from '../../shared/api';
import { emptyFilters, type Filters } from './filtering';

/** The board's saved views: loads the workspace's, saves the current filters as one, applies and deletes them. */
export function useSavedFilters({ workspaceId, filters, query, setFilters, setQuery }: {
	workspaceId?: string;
	filters: Filters;
	query: string;
	setFilters: (filters: Filters) => void;
	setQuery: (query: string) => void;
}) {
	const [savedFilters, setSavedFilters] = useState<SavedFilter[]>([]);
	const [saveFilterOpen, setSaveFilterOpen] = useState(false);
	const [saveFilterName, setSaveFilterName] = useState('');

	useEffect(() => {
		const controller = new AbortController();
		api.savedFilters(controller.signal)
			.then((saved) => {
				if (!controller.signal.aborted) setSavedFilters(saved.filter((filter) => !filter.workspaceId || filter.workspaceId === workspaceId));
			})
			.catch(() => {
				if (!controller.signal.aborted) setSavedFilters([]);
			});
		return () => controller.abort();
	}, [workspaceId]);

	const saveCurrentFilter = async () => {
		if (!saveFilterName.trim()) return;
		const saved = await api.saveFilter({
			name: saveFilterName.trim(),
			route: '/workstream',
			workspaceId,
			filters: { filters: { ...filters, branches: [] }, query }
		});
		setSavedFilters((current) => [saved, ...current]);
		setSaveFilterName('');
		setSaveFilterOpen(false);
	};

	const applySavedFilter = (saved: SavedFilter) => {
		const value = saved.filters as { filters?: Partial<Filters>; query?: string };
		setFilters({ ...emptyFilters, ...(value.filters ?? {}), branches: [] });
		setQuery(value.query ?? '');
	};

	const deleteSavedFilter = async (id: string) => {
		await api.deleteFilter(id);
		setSavedFilters((current) => current.filter((filter) => filter.id !== id));
	};

	return { savedFilters, saveFilterOpen, setSaveFilterOpen, saveFilterName, setSaveFilterName, saveCurrentFilter, applySavedFilter, deleteSavedFilter };
}
