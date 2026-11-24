import { useId } from 'react';
import { CircleHelp } from 'lucide-react';
import type { HarnessEngineId, HarnessStatus } from '../../lib/types';
import { engineLabel, modelLabel } from './vocabulary';

export const modelSuggestions = ['opus', 'sonnet', 'haiku'];

export function Override({ label, hint, value, fallback, onChange }: { label: string; hint: string; value?: boolean | null; fallback: boolean; onChange: (value: boolean) => void }) {
	const name = useId();
	const on = value === undefined || value === null ? fallback : value;
	return (
		<div className="settings-toggle-row">
			<span><strong>{label}</strong><small>{hint}</small></span>
			<div className="hx-choice-cards" role="radiogroup" aria-label={label}>
				{[true, false].map((choice) => (
					<label key={String(choice)} className={`hx-choice-card${on === choice ? ' on' : ''}`}>
						<input type="radio" name={name} value={choice ? 'on' : 'off'} checked={on === choice} onChange={() => onChange(choice)} />
						<strong>{choice ? 'On' : 'Off'}</strong>
					</label>
				))}
			</div>
		</div>
	);
}

export function Toggle({ label, hint, checked, onChange }: { label: string; hint: string; checked: boolean; onChange: (checked: boolean) => void }) {
	return (
		<label className="settings-toggle-row">
			<span><strong>{label}</strong><small>{hint}</small></span>
			<input type="checkbox" role="switch" checked={checked} onChange={(event) => onChange(event.target.checked)} />
		</label>
	);
}

export function EngineSelect({ value, engines, label, onChange }: { value: HarnessEngineId; engines: HarnessStatus['engines']; label: string; onChange: (engine: HarnessEngineId) => void }) {
	const ids: HarnessEngineId[] = engines.length ? engines.map((engine) => engine.id) : [value];
	return (
		<select className="hx-select" aria-label={label} value={value} onChange={(event) => onChange(event.target.value as HarnessEngineId)}>
			{ids.map((id) => {
				const engine = engines.find((candidate) => candidate.id === id);
				const unavailable = engine && !engine.available;
				return <option key={id} value={id} disabled={unavailable && id !== value}>{engineLabel(id)}{unavailable ? ' (not installed)' : ''}</option>;
			})}
		</select>
	);
}

export function ModelInput({ label, value, placeholder, hideLabel = false, hint, onChange }: { label: string; value: string; placeholder?: string; hideLabel?: boolean; hint?: string; onChange: (model: string) => void }) {
	const listId = useId();
	const inputId = useId();
	const hintId = useId();
	return (
		<div className="hx-model-input">
			<span className={hideLabel ? 'sr-only' : 'hx-model-label'}><label htmlFor={inputId}>{label}</label>{hint && (
				<span className="hx-hint">
					<button type="button" className="hx-hint-button" aria-label={`What ${label} means`} aria-describedby={hintId}><CircleHelp size={13} aria-hidden="true" /></button>
					<span id={hintId} role="tooltip" className="hx-hint-text">{hint}</span>
				</span>
			)}</span>
			<input id={inputId} aria-describedby={hint ? hintId : undefined} value={value} list={listId} title={value} placeholder={placeholder} onChange={(event) => onChange(event.target.value.trim())} />
			<datalist id={listId}>{modelSuggestions.map((model) => <option key={model} value={model}>{modelLabel(model)}</option>)}</datalist>
		</div>
	);
}
