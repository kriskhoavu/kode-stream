import { hxStatuses } from './vocabulary';
import type { HxStatus } from './vocabulary';

export function HxStatusBadge({ status, label, size = 'md', className }: { status: HxStatus; label?: string; size?: 'sm' | 'md'; className?: string }) {
	const meta = hxStatuses[status];
	const Icon = meta.icon;
	const classes = ['hx-badge', `hx-tone-${meta.tone}`, `hx-status-${status}`, size === 'sm' ? 'hx-badge-sm' : '', className ?? ''].filter(Boolean).join(' ');
	return (
		<span className={classes} data-status={status}>
			<Icon aria-hidden="true" strokeWidth={2.5} />
			<span>{label ?? meta.label}</span>
		</span>
	);
}
