import { useEffect, useRef, useState } from 'react';
import { formatCost } from './vocabulary';

const tweenMs = 600;

function reducedMotion(): boolean {
	return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

export function CostCounter({ value, className }: { value: number; className?: string }) {
	const [shown, setShown] = useState(value);
	const [ghost, setGhost] = useState<{ id: number; delta: number } | undefined>();
	const previous = useRef(value);
	const frame = useRef<number | undefined>(undefined);

	useEffect(() => {
		const from = previous.current;
		previous.current = value;
		if (value === from) return;
		if (value < from || reducedMotion()) {
			setShown(value);
			return;
		}
		setGhost((current) => ({ id: (current?.id ?? 0) + 1, delta: value - from }));
		const start = performance.now();
		const step = () => {
			const progress = Math.min(1, Math.max(0, (performance.now() - start) / tweenMs));
			const eased = 1 - Math.pow(1 - progress, 3);
			setShown(from + (value - from) * eased);
			frame.current = progress < 1 ? window.requestAnimationFrame(step) : undefined;
		};
		frame.current = window.requestAnimationFrame(step);
		return () => { if (frame.current !== undefined) window.cancelAnimationFrame(frame.current); };
	}, [value]);

	return (
		<span className={['hx-cost', className].filter(Boolean).join(' ')}>
			<span aria-hidden="true" className="hx-num">{formatCost(shown)}</span>
			<span className="sr-only">{formatCost(value)}</span>
			{ghost && <span key={ghost.id} className="hx-cost-ghost hx-num" aria-hidden="true" onAnimationEnd={() => setGhost(undefined)}>+{formatCost(ghost.delta)}</span>}
		</span>
	);
}
