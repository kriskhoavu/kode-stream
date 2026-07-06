import { AlertTriangle } from 'lucide-react';
import type { KnowledgeWarning } from '../../lib/types';

export function KnowledgeWarnings({ warnings, compact = false, indexDiagnostics = false }: { warnings: KnowledgeWarning[]; compact?: boolean; indexDiagnostics?: boolean }) {
	if (!warnings.length) return null;
	const contents = <ul>{warnings.map((warning, index) => <li key={`${warning.code}-${warning.path}-${index}`}><code>{warning.code}</code><span>{warning.path ? `${warning.path}: ` : ''}{warning.message}</span></li>)}</ul>;
	const label = indexDiagnostics ? `Index diagnostics (${warnings.length})` : `${warnings.length} warning${warnings.length === 1 ? '' : 's'}`;
	return <aside className={compact ? 'knowledge-warnings compact' : 'knowledge-warnings'} aria-label={indexDiagnostics ? 'Index diagnostics' : 'Knowledge warnings'}>{compact ? <details><summary><AlertTriangle size={15} /> {label}</summary>{indexDiagnostics && <p className="knowledge-warning-help">Files skipped while building this Wiki index. These are not errors in the selected page.</p>}{contents}</details> : <><strong><AlertTriangle size={15} /> {label}</strong>{contents}</>}</aside>;
}
