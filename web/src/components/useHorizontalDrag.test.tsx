import { renderHook } from '@testing-library/react';
import type { PointerEvent as ReactPointerEvent } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { useHorizontalDrag } from './useHorizontalDrag';

function pointerDown(clientX: number) {
	return { clientX, preventDefault: vi.fn() } as unknown as ReactPointerEvent;
}

function pointer(type: string, clientX: number) {
	const event = new Event(type) as Event & { clientX: number };
	event.clientX = clientX;
	window.dispatchEvent(event);
}

describe('useHorizontalDrag', () => {
	it('reports the distance moved and ends once when the pointer is released', () => {
		const { result } = renderHook(() => useHorizontalDrag());
		const move = vi.fn();
		const end = vi.fn();
		result.current(pointerDown(100), { move, end });
		expect(document.body.classList.contains('is-resizing-panel')).toBe(true);
		pointer('pointermove', 140);
		pointer('pointerup', 140);
		pointer('pointermove', 200);
		expect(move.mock.calls).toEqual([[40]]);
		expect(end).toHaveBeenCalledTimes(1);
		expect(document.body.classList.contains('is-resizing-panel')).toBe(false);
	});

	it('cancels a drag without ending it when a new drag starts or the owner unmounts', () => {
		const { result, unmount } = renderHook(() => useHorizontalDrag('is-resizing-drawer'));
		const first = { move: vi.fn(), end: vi.fn() };
		const second = { move: vi.fn(), end: vi.fn() };
		result.current(pointerDown(0), first);
		result.current(pointerDown(0), second);
		pointer('pointermove', 10);
		expect(first.move).not.toHaveBeenCalled();
		expect(second.move).toHaveBeenCalledWith(10);
		unmount();
		pointer('pointerup', 10);
		expect(first.end).not.toHaveBeenCalled();
		expect(second.end).not.toHaveBeenCalled();
		expect(document.body.classList.contains('is-resizing-drawer')).toBe(false);
	});
});
