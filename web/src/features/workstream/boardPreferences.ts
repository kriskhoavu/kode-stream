import { Columns3, ListOrdered, FileText, Ticket } from 'lucide-react';
import { readStringPreference } from '../../shared/preferences/store';
import type { ItemStatus } from '../../lib/types';
import { type ViewSwitcherOption } from '../../components/ViewSwitcher';

export type BoardKind = 'tasks' | 'plans';
export type BoardLayout = 'board' | 'list';
export const boardKinds: ViewSwitcherOption<BoardKind>[] = [{ id: 'tasks', label: 'Tasks', icon: Ticket }, { id: 'plans', label: 'Plans', icon: FileText }];
export const boardLayouts: ViewSwitcherOption<BoardLayout>[] = [{ id: 'board', label: 'Board', icon: Columns3 }, { id: 'list', label: 'List', icon: ListOrdered }];
export const boardKindPreference = 'workstreamBoardKind';
export const layoutPreferences: Record<BoardKind, string> = { tasks: 'workstreamTaskLayout', plans: 'workstreamPlanLayout' };
export const taskColumnLabels: Partial<Record<ItemStatus, string>> = { unsorted: 'To do' };
export const taskStartColumn: ItemStatus = 'draft';
export function readBoardKind(): BoardKind {
  return readStringPreference(boardKindPreference) === 'plans' ? 'plans' : 'tasks';
}
export function readBoardLayout(kind: BoardKind): BoardLayout {
  const value = readStringPreference(layoutPreferences[kind]);
  if (value === 'board' || value === 'list') return value;
  // A stored 'queue' view maps to the Tasks list.
  return kind === 'tasks' && readStringPreference('workstreamBoardView') === 'queue' ? 'list' : 'board';
}
