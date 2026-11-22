import { act, renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { FileContent } from '../../lib/types';
import { useOpenFileTabs } from './useOpenFileTabs';

const file = (id: string, path = `${id}.md`) => ({ id, path, editable: true }) as FileContent;

describe('useOpenFileTabs', () => {
	it('opens a tab once per file and activates it', () => {
		const { result } = renderHook(() => useOpenFileTabs());
		act(() => { result.current.remember(file('a')); result.current.remember(file('b')); result.current.remember(file('a', 'docs/a.md')); });
		expect(result.current.openTabs.map((tab) => [tab.id, tab.name])).toEqual([['a', 'a.md'], ['b', 'b.md']]);
		expect(result.current.activeTabId).toBe('a');
	});

	it('moves to the right-hand neighbour when the active tab closes, else the left-hand one', () => {
		const { result } = renderHook(() => useOpenFileTabs());
		act(() => { ['a', 'b', 'c'].forEach((id) => result.current.remember(file(id))); result.current.activate('b'); });
		let closed = { next: null as { id: string } | null };
		act(() => { closed = result.current.close(result.current.openTabs[1]); });
		expect(closed.next?.id).toBe('c');
		act(() => { closed = result.current.close(result.current.openTabs[1]); });
		expect(closed.next?.id).toBe('a');
		expect(result.current.activeTabId).toBe('a');
	});

	it('keeps the active tab when another tab closes', () => {
		const { result } = renderHook(() => useOpenFileTabs());
		act(() => { ['a', 'b'].forEach((id) => result.current.remember(file(id))); });
		let closed = { next: null as { id: string } | null };
		act(() => { closed = result.current.close(result.current.openTabs[0]); });
		expect(closed.next?.id).toBe('b');
		expect(result.current.openTabs.map((tab) => tab.id)).toEqual(['b']);
	});
});
