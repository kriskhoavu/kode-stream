import { Columns3, ListOrdered, FileText, Ticket } from 'lucide-react';
import { readStringPreference } from '../../shared/preferences/store';
import type { ItemStatus } from '../../lib/types';
import { type ViewSwitcherOption } from '../../components/ViewSwitcher';

/** What the board shows: the harness's tasks, whose columns follow their runs, or the plans on the branch, moved by hand. */
export type BoardKind = 'tasks' | 'plans';
/** How it shows them: cards in status columns, or rows. */
export type BoardLayout = 'board' | 'list';
export const boardKinds: ViewSwitcherOption<BoardKind>[] = [{ id: 'tasks', label: 'Tasks', icon: Ticket }, { id: 'plans', label: 'Plans', icon: FileText }];
export const boardLayouts: ViewSwitcherOption<BoardLayout>[] = [{ id: 'board', label: 'Board', icon: Columns3 }, { id: 'list', label: 'List', icon: ListOrdered }];
export const boardKindPreference = 'workstreamBoardKind';
export const layoutPreferences: Record<BoardKind, string> = { tasks: 'workstreamTaskLayout', plans: 'workstreamPlanLayout' };
/** The Tasks board's first column: a task nobody has started. */
export const taskColumnLabels: Partial<Record<ItemStatus, string>> = { unsorted: 'To do' };
/** The one column a task may be dragged to: dropping a task that has not started there starts it. */
export const taskStartColumn: ItemStatus = 'draft';
export function readBoardKind(): BoardKind {
  return readStringPreference(boardKindPreference) === 'plans' ? 'plans' : 'tasks';
}
export function readBoardLayout(kind: BoardKind): BoardLayout {
  const value = readStringPreference(layoutPreferences[kind]);
  if (value === 'board' || value === 'list') return value;
  // The Task Queue view the board had before is the Tasks list.
  return kind === 'tasks' && readStringPreference('workstreamBoardView') === 'queue' ? 'list' : 'board';
}
