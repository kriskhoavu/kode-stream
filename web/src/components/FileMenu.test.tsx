import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { FileNode } from '../lib/types';
import { FileMenu } from './FileMenu';

const tree: FileNode[] = [
  { id: 'd-scenario', name: 'scenario', path: 'scenario', type: 'directory', children: [{ id: 'f-s1', name: 'scenario-01.md', path: 'scenario/scenario-01.md', type: 'file' }] },
  { id: 'f-readme', name: 'README.md', path: 'README.md', type: 'file' }
];
const treeOptions = [{ id: 'f-readme', label: 'README.md' }, { id: 'f-s1', label: 'scenario/scenario-01.md' }];

describe('FileMenu', () => {
  it('supports Escape and roving arrow focus', () => {
    render(<FileMenu value="one" options={[{ id: 'one', label: 'One' }, { id: 'two', label: 'Two' }]} onChange={vi.fn()} />);
    const trigger = screen.getByRole('button', { name: 'Select document' });
    fireEvent.click(trigger);
    const one = screen.getByRole('menuitemradio', { name: 'One' });
    one.focus();
    fireEvent.keyDown(window, { key: 'ArrowDown' });
    expect(screen.getByRole('menuitemradio', { name: 'Two' })).toHaveFocus();
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('menu', { name: 'Documents' })).not.toBeInTheDocument();
  });

  it('shows the files as a tree, keeps folders toggling inside the dropdown, and closes on a pick or Escape', () => {
    const onChange = vi.fn();
    render(<FileMenu value="f-readme" options={treeOptions} tree={tree} onChange={onChange} />);
    const trigger = screen.getByRole('button', { name: 'Select document' });
    expect(trigger).toHaveTextContent('README.md');
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole('button', { name: 'scenario' }));
    expect(screen.queryByRole('button', { name: 'scenario-01.md' })).not.toBeInTheDocument();
    expect(screen.getByRole('tree', { name: 'Item files' })).toBeInTheDocument();
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('tree')).not.toBeInTheDocument();
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole('button', { name: 'scenario-01.md' }));
    expect(onChange).toHaveBeenCalledWith('f-s1');
    expect(screen.queryByRole('tree')).not.toBeInTheDocument();
  });
});
