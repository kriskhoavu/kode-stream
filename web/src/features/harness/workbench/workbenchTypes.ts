import { Info, ListChecks, MessagesSquare, Ticket } from 'lucide-react';
import type { StepCard } from '../pipeline';

export type Mode = 'plan' | 'changes' | 'agents';
export type Side = 'info' | 'jira' | 'checker' | 'steps';
export type View = 'preview' | 'source' | 'diff';
export type WorkbenchKey = `plan:${string}` | `file:${string}` | `agent:${string}` | `stage:${string}`;
export interface WorkbenchRequest { card: StepCard; n: number }

export const planPrefix = 'plans/';
export const statusLetter: Record<string, string> = { added: 'A', modified: 'M', deleted: 'D', A: 'A', M: 'M', D: 'D' };
export const statusClass: Record<string, string> = { A: 'added', M: 'modified', D: 'deleted' };

export const modeLabels: Record<Mode, string> = { plan: 'Plan', changes: 'Changes', agents: 'Agents' };
export const sideLabels: Record<Side, string> = { info: 'Info', jira: 'Jira', checker: 'Checker', steps: 'Steps' };
export const sideIcons = { info: Info, jira: Ticket, checker: ListChecks, steps: MessagesSquare };

export interface Loaded<T> { value?: T; error: string }

export interface TreeDir { name: string; dirs: TreeDir[]; files: string[] }
