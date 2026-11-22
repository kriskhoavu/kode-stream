import { act, renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { useFileTabs } from './useFileTabs';

type Tab = { id: string; name: string };
const keyOf = (tab: Tab) => tab.id;
const tab = (id: string, name = `${id}.md`): Tab => ({ id, name });

describe('useFileTabs', () => {
	it('opens a tab once per key, refreshes it in place and activates it', () => {
		const { result } = renderHook(() => useFileTabs(keyOf));
		act(() => { result.current.remember(tab('a')); result.current.remember(tab('b')); result.current.remember(tab('a', 'renamed.md')); });
		expect(result.current.openTabs).toEqual([tab('a', 'renamed.md'), tab('b')]);
		expect(result.current.activeTab).toEqual(tab('a', 'renamed.md'));
	});

	it('offers the right-hand neighbour when the active tab closes, else the left-hand one', () => {
		const { result } = renderHook(() => useFileTabs(keyOf));
		act(() => { ['a', 'b', 'c'].forEach((id) => result.current.remember(tab(id))); result.current.activate('b'); });
		let closed = { next: null as Tab | null };
		act(() => { closed = result.current.close(tab('b')); });
		expect(closed.next?.id).toBe('c');
		act(() => { result.current.activate('c'); });
		act(() => { closed = result.current.close(tab('c')); });
		expect(closed.next?.id).toBe('a');
		act(() => { closed = result.current.close(tab('a')); });
		expect(closed.next).toBeNull();
		expect(result.current.activeKey).toBe('');
	});

	it('offers the active tab when another tab closes', () => {
		const { result } = renderHook(() => useFileTabs(keyOf));
		act(() => { ['a', 'b'].forEach((id) => result.current.remember(tab(id))); });
		let closed = { next: null as Tab | null };
		act(() => { closed = result.current.close(tab('a')); });
		expect(closed.next?.id).toBe('b');
		expect(result.current.openTabs).toEqual([tab('b')]);
	});

	it('moves to the first tab left when retain drops the active one', () => {
		const { result } = renderHook(() => useFileTabs(keyOf));
		act(() => { ['a', 'b', 'c'].forEach((id) => result.current.remember(tab(id))); });
		act(() => { result.current.retain((candidate) => candidate.id !== 'c'); });
		expect(result.current.activeKey).toBe('a');
		act(() => { result.current.activate('b'); result.current.retain((candidate) => candidate.id !== 'a'); });
		expect(result.current.activeKey).toBe('b');
	});
});
