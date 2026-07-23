import { useEffect, useState } from 'react';
import { api } from '../../lib/api';
import type { E2ERunbookList, KnowledgePageDetail } from '../../lib/types';
import { E2EQualityPanel } from './E2EQualityPanel';

export function KnowledgeE2ESidePanel({ workspaceId, root, detail }: { workspaceId: string; root: string; detail: KnowledgePageDetail }) {
	const [e2e, setE2E] = useState<E2ERunbookList>({ runbooks: [] });
	const refresh = () => void api.knowledgeE2ERunbook(workspaceId, root, detail.slug).then((result) => setE2E(result ?? { runbooks: [] })).catch(() => setE2E({ runbooks: [], diagnostic: 'E2E coverage could not be loaded.' }));
	useEffect(() => { refresh(); }, [workspaceId, root, detail.slug]);
	return <E2EQualityPanel workspaceId={workspaceId} runbooks={e2e.runbooks} diagnostic={e2e.diagnostic} onRefresh={refresh} />;
}
