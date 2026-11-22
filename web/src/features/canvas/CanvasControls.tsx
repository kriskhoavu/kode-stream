import { useRef } from 'react';
import { useDismiss } from '../../components/useDismiss';
import { useReactFlow } from '@xyflow/react';
import { ChevronDown, Expand, LayoutGrid, Maximize, Minimize2, Minus, Plus, RotateCcw, Search, WandSparkles } from 'lucide-react';

export function CanvasNodeLegend() {
	return <aside className="canvas-node-legend" aria-label="Node type legend"><span className="workspace"><i />Workspace group</span><span className="service"><i />Service group</span><span className="plan"><i />Plan</span><span className="session"><i />Terminal session</span></aside>;
}

export function CanvasGraphControls({ layoutAvailable, layoutMessage, onArrange, serviceGridColumns, onReset, isFullscreen, onToggleFullscreen }: { layoutAvailable: boolean; layoutMessage?: string; onArrange: (grouping: 'status' | 'service' | 'service_status' | 'service_grid') => void; serviceGridColumns: number; onReset: () => void; isFullscreen: boolean; onToggleFullscreen: () => void }) {
	const { fitView, zoomIn, zoomOut } = useReactFlow();
	const unavailableTitle = !layoutAvailable ? layoutMessage : undefined;
	return <div className="canvas-graph-controls-panel" aria-label="Canvas graph controls">
		<button type="button" className="canvas-graph-control-button" aria-label="Zoom in" title="Zoom in" onClick={() => void zoomIn()}><Plus size={15} /></button>
		<button type="button" className="canvas-graph-control-button" aria-label="Zoom out" title="Zoom out" onClick={() => void zoomOut()}><Minus size={15} /></button>
		<button type="button" className="canvas-graph-control-button" aria-label="Fit Canvas to view" title="Fit Canvas to view" onClick={() => void fitView({ padding: 0.08, maxZoom: 1.35 })}><Maximize size={15} /></button>
		<button type="button" className="canvas-graph-control-button" aria-label={isFullscreen ? 'Exit fullscreen Canvas' : 'Open fullscreen Canvas'} title={isFullscreen ? 'Exit fullscreen' : 'Open fullscreen'} onClick={onToggleFullscreen}>{isFullscreen ? <Minimize2 size={15} /> : <Expand size={15} />}</button>
		<button type="button" className="canvas-graph-control-button" aria-label="Beautify Canvas layout" title="Beautify Canvas layout" disabled={!layoutAvailable} onClick={() => onArrange('status')}><WandSparkles size={15} /></button>
		<button type="button" className="canvas-graph-control-button" aria-label={`Arrange service plans in ${serviceGridColumns === 4 ? 1 : serviceGridColumns + 1} columns`} title={`Cycle service grid: ${serviceGridColumns === 4 ? 1 : serviceGridColumns + 1} columns`} disabled={!layoutAvailable} onClick={() => onArrange('service_grid')}><LayoutGrid size={15} /></button>
		<button type="button" className="canvas-graph-control-button" aria-label="Reset layout" title={unavailableTitle || 'Reset layout'} disabled={!layoutAvailable} onClick={onReset}><RotateCcw size={15} /></button>
	</div>;
}

export function CanvasSearch({ query, onQueryChange }: { query: string; onQueryChange: (value: string) => void }) {
	return <label className="filter-input plan-search canvas-search"><Search size={15} /><input aria-label="Search Canvas nodes" value={query} onChange={(event) => onQueryChange(event.target.value)} placeholder="Search items..." /></label>;
}

export function CanvasFacetMenu({ title, options, selected, open, onOpen, onClose, onToggle, onClear }: { title: string; options: Array<{ value: string; label: string }>; selected: string[]; open: boolean; onOpen: () => void; onClose: () => void; onToggle: (value: string) => void; onClear: () => void }) {
	const menuRef = useRef<HTMLElement>(null);
	useDismiss(menuRef, open, onClose);
	if (options.length === 0) return null;
	return <section className="facet-menu" ref={menuRef}><button type="button" className={selected.length ? 'facet-trigger active' : 'facet-trigger'} aria-expanded={open} onClick={onOpen}><span>{title}</span><span className="facet-trigger-right">{selected.length > 0 && <strong>{selected.length}</strong>}<ChevronDown className={open ? 'facet-chevron open' : 'facet-chevron'} size={15} /></span></button>{open && <div className="facet-popover"><div className="facet-popover-header"><strong>{title}</strong><button type="button" onClick={onClear} disabled={selected.length === 0}>Clear</button></div><div className="facet-option-list">{options.map((option) => <label className="facet-option" key={option.value}><input type="checkbox" checked={selected.includes(option.value)} onChange={() => onToggle(option.value)} /><span>{option.label}</span></label>)}</div></div>}</section>;
}

export function CanvasArrangeMenu({ open, disabled, onOpen, onClose, onArrange }: { open: boolean; disabled: boolean; onOpen: () => void; onClose: () => void; onArrange: (grouping: 'status' | 'service' | 'service_status') => void }) {
	const menuRef = useRef<HTMLElement>(null);
	useDismiss(menuRef, open, onClose);
	const choose = (value: 'status' | 'service' | 'service_status') => { onArrange(value); onClose(); };
	return <section className="facet-menu canvas-arrange-menu" ref={menuRef}><button type="button" className="facet-trigger" aria-expanded={open} disabled={disabled} onClick={onOpen}><span>Group</span><ChevronDown className={open ? 'facet-chevron open' : 'facet-chevron'} size={15} /></button>{open && <div className="facet-popover"><div className="facet-popover-header"><strong>Arrange plans by</strong></div><div className="facet-option-list"><button type="button" onClick={() => choose('status')}>Status</button><button type="button" onClick={() => choose('service')}>Service</button><button type="button" onClick={() => choose('service_status')}>Service + status</button></div></div>}</section>;
}
