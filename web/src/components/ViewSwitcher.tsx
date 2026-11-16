import type { LucideIcon } from 'lucide-react';
import './viewSwitcher.css';

export interface ViewSwitcherOption<T extends string> {
	id: T;
	label: string;
	icon: LucideIcon;
	/** Words after the label, such as a count. */
	suffix?: string;
	disabled?: boolean;
}

/**
 * The one way a page switches between its views: Workstream's Tasks · Plans,
 * Agent Space's Sessions · Canvas, Knowledge's Pages · Graph. It sits at the
 * left of the page's title row; `iconOnly` is the compact form beside a
 * toolbar's buttons, named on hover and for screen readers.
 */
export function ViewSwitcher<T extends string>({ label, options, value, onChange, iconOnly = false }: {
	label: string;
	options: ViewSwitcherOption<T>[];
	value: T;
	onChange: (value: T) => void;
	iconOnly?: boolean;
}) {
	return (
		<div className={iconOnly ? 'segmented-control view-switcher icon-only' : 'segmented-control view-switcher'} role="group" aria-label={label}>
			{options.map((option) => {
				const text = option.suffix ? `${option.label} ${option.suffix}` : option.label;
				return (
					<button key={option.id} type="button" className={value === option.id ? 'active' : ''} aria-pressed={value === option.id} disabled={option.disabled} aria-label={iconOnly ? text : undefined} title={iconOnly ? text : undefined} onClick={() => onChange(option.id)}>
						<option.icon size={iconOnly ? 16 : 15} aria-hidden="true" />
						{!iconOnly && text}
					</button>
				);
			})}
		</div>
	);
}
