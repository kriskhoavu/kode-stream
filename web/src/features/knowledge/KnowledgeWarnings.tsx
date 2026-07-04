import { AlertTriangle } from 'lucide-react';
import type { KnowledgeWarning } from '../../lib/types';

export function KnowledgeWarnings({ warnings }: { warnings: KnowledgeWarning[] }) {
	if (!warnings.length) return null;
	return <aside className="knowledge-warnings" aria-label="Knowledge warnings"><strong><AlertTriangle size={15} /> {warnings.length} warning{warnings.length === 1 ? '' : 's'}</strong><ul>{warnings.map((warning, index) => <li key={`${warning.code}-${warning.path}-${index}`}><code>{warning.code}</code> {warning.message}</li>)}</ul></aside>;
}
