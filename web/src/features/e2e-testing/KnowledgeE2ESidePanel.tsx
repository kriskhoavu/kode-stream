import { useCallback, useEffect, useRef, useState } from 'react';
import { GitCompare } from 'lucide-react';
import { api } from '../../shared/api';
import type { E2ERunbookList, KnowledgePageDetail } from '../../lib/types';
import { E2EQualityPanel } from './E2EQualityPanel';

export function KnowledgeE2ESidePanel({ workspaceId, root, detail }: { workspaceId: string; root: string; detail: KnowledgePageDetail }) {
	const [e2e, setE2E] = useState<E2ERunbookList>({ runbooks: [] });
	const abortRef = useRef<AbortController | null>(null);
	const refresh = useCallback(() => {
		abortRef.current?.abort();
		const controller = new AbortController();
		abortRef.current = controller;
		void api.knowledgeE2ERunbook(workspaceId, root, detail.slug, controller.signal)
			.then((result) => {
				if (!controller.signal.aborted) setE2E(result ?? { runbooks: [] });
			})
			.catch(() => {
				if (!controller.signal.aborted) setE2E({ runbooks: [], diagnostic: 'E2E coverage could not be loaded.' });
			});
	}, [detail.slug, root, workspaceId]);
	useEffect(() => {
		refresh();
		return () => abortRef.current?.abort();
	}, [refresh]);
	return <><div className="side-panel-tabs knowledge-side-panel-tabs" role="tablist" aria-label="Knowledge side panel"><button type="button" className="active" aria-selected="true"><GitCompare size={14} /> Quality</button></div><section className="metadata-callout quality-panel"><E2EQualityPanel variant="nested" workspaceId={workspaceId} runbooks={e2e.runbooks} diagnostic={e2e.diagnostic} onRefresh={refresh} /></section></>;
}
