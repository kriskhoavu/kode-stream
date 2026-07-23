import { useEffect, useState } from 'react';
import { GitCompare } from 'lucide-react';
import { api } from '../../lib/api';
import type { E2ERunbookList, KnowledgePageDetail } from '../../lib/types';
import { E2EQualityPanel } from './E2EQualityPanel';

export function KnowledgeE2ESidePanel({ workspaceId, root, detail }: { workspaceId: string; root: string; detail: KnowledgePageDetail }) {
	const [e2e, setE2E] = useState<E2ERunbookList>({ runbooks: [] });
	const refresh = () => void api.knowledgeE2ERunbook(workspaceId, root, detail.slug).then((result) => setE2E(result ?? { runbooks: [] })).catch(() => setE2E({ runbooks: [], diagnostic: 'E2E coverage could not be loaded.' }));
	useEffect(() => { refresh(); }, [workspaceId, root, detail.slug]);
	return <><div className="side-panel-tabs knowledge-side-panel-tabs" role="tablist" aria-label="Knowledge side panel"><button type="button" className="active" aria-selected="true"><GitCompare size={14} /> Quality</button></div><section className="metadata-callout quality-panel"><E2EQualityPanel variant="nested" workspaceId={workspaceId} runbooks={e2e.runbooks} diagnostic={e2e.diagnostic} onRefresh={refresh} /></section></>;
}
