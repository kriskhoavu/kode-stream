import { Bot, Cog } from 'lucide-react';
import type { HarnessEngineId } from '../../lib/types';
import { workerLabel } from './vocabulary';

/** Who is doing the work: "Claude · Opus 5.5", or a role with no model ("Tester · no model"). The raw id stays in the tooltip. */
export function WorkerChip({ engine, model, role }: { engine?: HarnessEngineId; model?: string; role?: string }) {
	const text = engine ? workerLabel(engine, model) : `${role ?? 'Harness'} · no model`;
	const title = engine ? [role, engine, model].filter(Boolean).join(' · ') : undefined;
	const Icon = engine ? Bot : Cog;
	return <span className="hx-chip" title={title}><Icon aria-hidden="true" size={12} />{text}</span>;
}
