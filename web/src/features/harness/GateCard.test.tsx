import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { GateCard } from './GateCard';
import { approval, task } from './testData';

afterEach(cleanup);

describe('GateCard', () => {
	it('says what approving does and approves in one step', () => {
		const onApprove = vi.fn();
		render(<GateCard approval={approval('a1', 'task-PM-02', { estimate: { minMinutes: 5, maxMinutes: 10, costUsd: 3 }, context: { verdict: 'approve', blocking: 0, advisory: 4, rounds: 2, costUsd: 4.47, durationMs: 600_000 } })} task={task('PM-02', { title: 'Add login form' })} onApprove={onApprove} onReject={vi.fn()} />);
		expect(screen.getByRole('heading', { name: 'Waiting on you: approve the plan for PM-02 · Add login form' })).toBeInTheDocument();
		expect(screen.getByText('Approving lets the Builder start writing code. That usually takes about 5–10 minutes and around $3.00.')).toBeInTheDocument();
		expect(screen.getByText('0 must fix · 4 suggestions · 2 rounds · 10 min · $4.47')).toBeInTheDocument();
		const buttons = screen.getAllByRole('button');
		expect(buttons[0]).toHaveTextContent('Approve');
		fireEvent.click(buttons[0]);
		expect(onApprove).toHaveBeenCalledWith();
	});

	it('asks for a reason before asking for changes, and sends it with Ctrl+Enter', () => {
		const onReject = vi.fn();
		render(<GateCard approval={approval('a1', 'task-A')} task={task('A')} onApprove={vi.fn()} onReject={onReject} />);
		fireEvent.click(screen.getByRole('button', { name: 'Ask for changes' }));
		const dialog = screen.getByRole('dialog', { name: 'What should change?' });
		const reason = screen.getByRole('textbox', { name: 'What should change?' });
		expect(reason).toHaveFocus();
		fireEvent.click(screen.getByRole('button', { name: 'Send to Planner' }));
		expect(screen.getByRole('alert')).toHaveTextContent('Write a few words first');
		expect(onReject).not.toHaveBeenCalled();
		fireEvent.change(reason, { target: { value: 'Name the rate limit' } });
		fireEvent.keyDown(reason, { key: 'Enter', ctrlKey: true });
		expect(onReject).toHaveBeenCalledWith('Name the rate limit');
		expect(dialog).not.toBeInTheDocument();
	});

	it('shows Sending… while the decision is in flight', () => {
		render(<GateCard approval={approval('a1', 'task-A')} task={task('A')} sent={{ approvalId: 'a1', decision: 'approve', state: 'sending' }} onApprove={vi.fn()} onReject={vi.fn()} />);
		expect(screen.getByRole('button', { name: 'Sending…' })).toBeDisabled();
	});

	it('says who decided it and where when the decision came from elsewhere', () => {
		render(<GateCard approval={approval('a1', 'task-A', { state: 'approved', channel: 'webex', decidedBy: { actorId: 'anna', displayName: 'Anna' }, decidedAt: new Date(Date.now() - 180_000).toISOString() })} task={task('A')} onApprove={vi.fn()} onReject={vi.fn()} />);
		expect(screen.getByText(/Approved via Webex by Anna · 3 min ago/)).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Approve' })).not.toBeInTheDocument();
	});

	it('lists what the agent asks, takes the answers, and sends them as the approval', () => {
		const onApprove = vi.fn();
		const onReject = vi.fn();
		render(<GateCard approval={approval('a1', 'task-S', { gate: 'question', summary: 'The Builder stopped to ask you before going on.', context: { role: 'coder', questions: ['Keep the old endpoint too?', 'Which log level for the retry?'] } })} task={task('S')} onApprove={onApprove} onReject={onReject} />);
		expect(screen.getByRole('heading', { name: /the Builder asks about/ })).toBeInTheDocument();
		expect(screen.getByRole('list', { name: 'What the Builder asks' })).toHaveTextContent('Keep the old endpoint too?');
		fireEvent.click(screen.getByRole('button', { name: 'Answer and continue' }));
		expect(screen.getByRole('list', { name: 'The questions' })).toHaveTextContent('Which log level for the retry?');
		fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Keep it. Warn.' } });
		fireEvent.click(screen.getByRole('button', { name: 'Send and continue' }));
		expect(onApprove).toHaveBeenCalledWith('Keep it. Warn.');
		expect(onReject).not.toHaveBeenCalled();
	});

	it('shows the open questions a Planner went on without, on the plan approval', () => {
		render(<GateCard approval={approval('a2', 'task-S', { gate: 'plan', context: { verdict: 'approve', questions: ['Archived offers too?'] } })} task={task('S')} onApprove={vi.fn()} onReject={vi.fn()} />);
		expect(screen.getByText(/went on with its own best guess/)).toBeInTheDocument();
		expect(screen.getByRole('list', { name: 'Open questions from the Planner' })).toHaveTextContent('Archived offers too?');
	});

	it('takes the answer first when a stuck task is retried', () => {
		const onApprove = vi.fn();
		render(<GateCard approval={approval('a1', 'task-S', { gate: 'resume', summary: 'The Planner cannot decide that alone.' })} task={task('S')} onApprove={onApprove} onReject={vi.fn()} />);
		expect(screen.getByText('Stopped')).toBeInTheDocument();
		fireEvent.click(screen.getByRole('button', { name: 'Answer and retry' }));
		fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Keep web clients connected' } });
		fireEvent.click(screen.getByRole('button', { name: 'Send and retry' }));
		expect(onApprove).toHaveBeenCalledWith('Keep web clients connected');
	});
});
