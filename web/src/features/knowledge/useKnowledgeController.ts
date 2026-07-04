import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { KnowledgeLocation } from '../../app/router';
import { api } from '../../lib/api';
import type { KnowledgeActionResult, KnowledgePage, KnowledgePageDetail, KnowledgeWarning, KnowledgeWiki, WorkspaceConfig } from '../../lib/types';

export function useKnowledgeController(workspaces: WorkspaceConfig[], location: KnowledgeLocation | undefined, onLocationChange: (location: KnowledgeLocation) => void) {
	const [wikis, setWikis] = useState<KnowledgeWiki[]>([]);
	const [pages, setPages] = useState<KnowledgePage[]>([]);
	const [warnings, setWarnings] = useState<KnowledgeWarning[]>([]);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const [notice, setNotice] = useState('');
	const [actionResult, setActionResult] = useState<KnowledgeActionResult | null>(null);
	const [actionBusy, setActionBusy] = useState(false);
	const [detail, setDetail] = useState<KnowledgePageDetail | null>(null);
	const [detailLoading, setDetailLoading] = useState(false);
	const requestVersion = useRef(0);
	const detailVersion = useRef(0);
	const locationRef = useRef(location);
	const onLocationChangeRef = useRef(onLocationChange);
	locationRef.current = location;
	onLocationChangeRef.current = onLocationChange;
	const workspace = workspaces.find((candidate) => candidate.id === location?.workspaceId) ?? workspaces[0];
	const wiki = wikis.find((candidate) => candidate.root === location?.root) ?? wikis[0];
	const page = pages.find((candidate) => candidate.slug === location?.slug);

	const updateLocation = useCallback((patch: Partial<KnowledgeLocation>) => {
		onLocationChangeRef.current({ ...locationRef.current, ...patch });
	}, []);

	const load = useCallback(async () => {
		const version = ++requestVersion.current;
		setError(''); setNotice(''); setLoading(true);
		if (!workspace) { setWikis([]); setPages([]); setWarnings([]); setLoading(false); return; }
		try {
			const currentLocation = locationRef.current;
			const loadedWikis = await api.knowledgeWikis(workspace.id);
			if (version !== requestVersion.current) return;
			setWikis(loadedWikis);
			const selectedWiki = loadedWikis.find((candidate) => candidate.root === currentLocation?.root) ?? loadedWikis[0];
			if (!selectedWiki) { setPages([]); setWarnings([]); setLoading(false); if (currentLocation?.workspaceId !== workspace.id || currentLocation?.root) onLocationChangeRef.current({ workspaceId: workspace.id, view: 'browse' }); return; }
			const response = await api.knowledgePages(workspace.id, selectedWiki.root);
			if (version !== requestVersion.current) return;
			setPages(response.pages); setWarnings(response.warnings);
			const selectedPage = response.pages.find((candidate) => candidate.slug === currentLocation?.slug);
			const next: KnowledgeLocation = { workspaceId: workspace.id, root: selectedWiki.root, view: currentLocation?.view ?? 'browse' };
			if (selectedPage) next.slug = selectedPage.slug;
			else if (currentLocation?.slug) setNotice('The selected page is no longer available.');
			if (!sameLocation(currentLocation, next)) onLocationChangeRef.current(next);
		} catch (requestError) {
			if (version === requestVersion.current) { setError(requestError instanceof Error ? requestError.message : 'Knowledge could not be loaded.'); setPages([]); setWarnings([]); }
		} finally { if (version === requestVersion.current) setLoading(false); }
	}, [workspace, location?.workspaceId, location?.root, location?.slug, location?.view]);

	useEffect(() => { void load(); return () => { requestVersion.current++; }; }, [load]);

	useEffect(() => {
		const version = ++detailVersion.current;
		if (!workspace || !wiki || !location?.slug || location.view !== 'read') { setDetail(null); setDetailLoading(false); return; }
		setDetailLoading(true);
		void api.knowledgePage(workspace.id, wiki.root, location.slug).then((loaded) => {
			if (version === detailVersion.current) setDetail(loaded);
		}).catch(() => {
			if (version !== detailVersion.current) return;
			setDetail(null); setNotice('The selected page could not be loaded. It may have been removed.');
			onLocationChangeRef.current({ workspaceId: workspace.id, root: wiki.root, view: 'browse' });
		}).finally(() => { if (version === detailVersion.current) setDetailLoading(false); });
		return () => { detailVersion.current++; };
	}, [workspace, wiki, location?.slug, location?.view]);

	const runAction = useCallback(async (operation: 'rescan' | 'sync' | 'enrich', confirm = false) => {
		if (!workspace || (operation === 'rescan' && !wiki)) return null;
		setActionBusy(true); setError('');
		try {
			const result = operation === 'rescan' ? await api.rescanKnowledge(workspace.id, wiki!.root)
				: operation === 'sync' ? await api.syncKnowledge(workspace.id, confirm) : await api.enrichKnowledge(workspace.id, confirm);
			setActionResult(result); await load(); return result;
		} catch (actionError) { setError(actionError instanceof Error ? actionError.message : 'Knowledge action failed.'); return null; }
		finally { setActionBusy(false); }
	}, [load, wiki, workspace]);

	return useMemo(() => ({ workspace, wiki, page, detail, detailLoading, wikis, pages, warnings, loading, error, notice, actionBusy, actionResult, updateLocation, reload: load, runAction }), [workspace, wiki, page, detail, detailLoading, wikis, pages, warnings, loading, error, notice, actionBusy, actionResult, updateLocation, load, runAction]);
}

function sameLocation(left: KnowledgeLocation | undefined, right: KnowledgeLocation): boolean {
	return left?.workspaceId === right.workspaceId && left?.root === right.root && left?.slug === right.slug && left?.view === right.view;
}
