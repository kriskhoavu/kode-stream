import { Fragment, useEffect, useRef, useState } from 'react';
import type { KeyboardEvent } from 'react';
import { hxStatuses, hxSteps } from './vocabulary';
import type { HxStatus } from './vocabulary';
import { groupStatus } from './pipeline';
import type { PipelineModel, StepCard } from './pipeline';
import { LiveTimer } from './LiveTimer';

/**
 * The run as a row of plain steps. One tab stop: arrow keys move between
 * steps (roving tabindex), Enter or a click opens a two-sentence explainer.
 * `orientation="vertical"` is the drawer's compact form.
 */
export function PipelineStepper({ model, label, orientation = 'horizontal' }: { model: PipelineModel; label: string; orientation?: 'horizontal' | 'vertical' }) {
	const cards = model.groups.flatMap((group) => group.cards);
	const initial = Math.max(0, cards.findIndex((card) => card.current));
	const [focusIndex, setFocusIndex] = useState(initial);
	const [open, setOpen] = useState<string | undefined>();
	const refs = useRef<(HTMLButtonElement | null)[]>([]);
	const currentId = model.current?.id;

	useEffect(() => {
		// Follow the run: when the current step moves, the tab stop moves with it.
		// `cards` comes from the same model as `currentId`, so only the id is watched.
		const index = cards.findIndex((card) => card.id === currentId);
		if (index >= 0) setFocusIndex(index);
	}, [currentId]);

	useEffect(() => {
		if (!open) return;
		const close = (event: globalThis.KeyboardEvent) => { if (event.key === 'Escape') setOpen(undefined); };
		window.addEventListener('keydown', close);
		return () => window.removeEventListener('keydown', close);
	}, [open]);

	const move = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
		const keys: Record<string, number> = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 };
		let next = index;
		if (event.key in keys) next = Math.min(cards.length - 1, Math.max(0, index + keys[event.key]));
		else if (event.key === 'Home') next = 0;
		else if (event.key === 'End') next = cards.length - 1;
		else return;
		event.preventDefault();
		setFocusIndex(next);
		refs.current[next]?.focus();
	};

	let index = -1;
	return (
		<div className={`hx-stepper-wrap ${orientation}`}>
			<ol className={`hx-stepper ${orientation}`} aria-label={label}>
				{model.groups.map((group, groupIndex) => {
					const into = groupIndex === model.activeGroup;
					const previousDone = groupIndex > 0 && groupStatus(model.groups[groupIndex - 1]) === 'done';
					return (
						<Fragment key={group.id}>
							{groupIndex > 0 && <li className={['hx-conn', into ? 'flow' : previousDone && groupStatus(group) !== 'idle' ? 'done' : ''].filter(Boolean).join(' ')} aria-hidden="true" />}
							<li className="hx-step-group" role="group" aria-label={group.label}>
								<span className="hx-step-group-label" aria-hidden="true">{group.label}</span>
								{group.cards.map((card) => {
									index += 1;
									const position = index;
									return (
										<StageCard
											key={card.id}
											card={card}
											refCallback={(element) => { refs.current[position] = element; }}
											tabIndex={position === focusIndex ? 0 : -1}
											open={open === card.id}
											onToggle={() => { setFocusIndex(position); setOpen((current) => current === card.id ? undefined : card.id); }}
											onKeyDown={(event) => move(event, position)}
										/>
									);
								})}
							</li>
						</Fragment>
					);
				})}
			</ol>
			{orientation === 'horizontal' && <StepperLegend />}
		</div>
	);
}

function StageCard({ card, refCallback, tabIndex, open, onToggle, onKeyDown }: {
	card: StepCard;
	refCallback: (element: HTMLButtonElement | null) => void;
	tabIndex: number;
	open: boolean;
	onToggle: () => void;
	onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => void;
}) {
	const meta = hxStatuses[card.status];
	const Icon = meta.icon;
	const explainId = `hx-explain-${card.id}`;
	// Draw the check and glow only when a step finishes while someone watches, not on every page load.
	const [justDone, setJustDone] = useState(false);
	const previous = useRef(card.status);
	useEffect(() => {
		if (card.status === 'done' && previous.current !== 'done') setJustDone(true);
		previous.current = card.status;
	}, [card.status]);
	return (
		<div className="hx-stage-slot">
			<button
				ref={refCallback}
				type="button"
				className={`hx-stage hx-tone-${meta.tone} hx-stage-${card.status}${justDone ? ' just-done' : ''}`}
				tabIndex={tabIndex}
				aria-current={card.current ? 'step' : undefined}
				aria-expanded={open}
				aria-controls={open ? explainId : undefined}
				onClick={onToggle}
				onKeyDown={onKeyDown}
				onAnimationEnd={(event) => { if (event.animationName === 'hx-glow') setJustDone(false); }}
			>
				<span className="hx-stage-status"><Icon aria-hidden="true" strokeWidth={2.5} key={card.status} className={justDone ? 'hx-check-draw' : undefined} />{meta.label}</span>
				<span className="hx-stage-name">{card.label}</span>
				<span className="hx-stage-meta">
					{card.since && <LiveTimer since={card.since} label="for" />}
					{card.meta.map((fact) => <span key={fact}>{fact}</span>)}
				</span>
			</button>
			{open && <StageExplainer id={explainId} card={card} />}
		</div>
	);
}

/** Two plain sentences: what the step does, and where it stands. */
export function StageExplainer({ id, card }: { id: string; card: StepCard }) {
	return (
		<div className="hx-explain" id={id} role="note">
			<b>{card.label}</b>
			<span>{hxSteps[card.step].explain}</span>
			<span> {statusSentence(card.status)}</span>
		</div>
	);
}

function statusSentence(status: HxStatus): string {
	switch (status) {
		case 'working': return 'It is running now.';
		case 'waiting': return 'It is waiting for a person to decide.';
		case 'done': return 'It is done.';
		case 'needs_fix': return 'It found something to fix, so it runs again.';
		case 'failed': return "It didn't pass.";
		case 'stopped': return 'It was stopped.';
		case 'skipped': return 'It was not needed this time.';
		case 'manual': return 'A person did it by hand.';
		default: return 'It has not started yet.';
	}
}

export function StepperLegend() {
	const items: HxStatus[] = ['done', 'working', 'waiting', 'stopped', 'not_started', 'manual'];
	return (
		<div className="hx-legend" aria-label="Legend">
			{items.map((status) => (
				<span key={status}><i className={`hx-swatch hx-tone-${hxStatuses[status].tone} ${status === 'not_started' || status === 'manual' ? 'outline' : ''}`} aria-hidden="true" />{hxStatuses[status].label}</span>
			))}
			<span className="hx-legend-hint">Click a step to see what it does.</span>
		</div>
	);
}
