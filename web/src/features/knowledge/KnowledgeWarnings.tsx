import { AlertTriangle } from 'lucide-react';
import type { KnowledgeWarning } from '../../lib/types';

export function KnowledgeWarnings({ warnings, compact = false }: { warnings: KnowledgeWarning[]; compact?: boolean }) {
	if (!warnings.length) return null;
	const contents = <ul>{warnings.map((warning, index) => <li key={`${warning.code}-${warning.path}-${index}`}><code>{warning.code}</code><span>{warning.path ? `${warning.path}: ` : ''}{warning.message}</span></li>)}</ul>;
	return <aside className={compact ? 'knowledge-warnings compact' : 'knowledge-warnings'} aria-label="Knowledge warnings">{compact ? <details><summary><AlertTriangle size={15} /> {warnings.length} warning{warnings.length === 1 ? '' : 's'}</summary>{contents}</details> : <><strong><AlertTriangle size={15} /> {warnings.length} warning{warnings.length === 1 ? '' : 's'}</strong>{contents}</>}</aside>;
}
