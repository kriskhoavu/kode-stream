import { cleanup, render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { LatticeBackdrop, buildLattice } from './LatticeBackdrop';

afterEach(cleanup);

function edges(container: HTMLElement) {
  return [...container.querySelectorAll('line')].map((line) => ({
    x1: Number(line.getAttribute('x1')), y1: Number(line.getAttribute('y1')),
    x2: Number(line.getAttribute('x2')), y2: Number(line.getAttribute('y2'))
  }));
}

describe('LatticeBackdrop', () => {
  it('paints a network of edges and nodes', () => {
    const { container } = render(<LatticeBackdrop />);

    expect(container.querySelector('.lattice-grid')).toBeTruthy();
    expect(container.querySelector('.lattice-sweep')).toBeTruthy();
    expect(edges(container).length).toBeGreaterThan(20);
    expect(container.querySelectorAll('.lattice-node').length).toBeGreaterThan(20);
  });

  /*
   * These four are what make the grid and the network read as one field rather
   * than a scatter laid over a grid. Each is a way the generator can still
   * render something plausible while no longer rendering a lattice.
   */
  it('places every node on the fine grid', () => {
    for (const node of buildLattice().nodes) {
      expect(node.x % 24).toBe(0);
      expect(node.y % 24).toBe(0);
    }
  });

  it('draws every edge orthogonally or at exactly 45 degrees', () => {
    for (const edge of buildLattice().edges) {
      const dx = Math.abs(edge.x2 - edge.x1);
      const dy = Math.abs(edge.y2 - edge.y1);
      expect(dx === 0 || dy === 0 || dx === dy).toBe(true);
    }
  });

  it('emits no zero-length edge, even where a walk clamps against the field edge', () => {
    for (const edge of buildLattice().edges) {
      expect(edge.x1 !== edge.x2 || edge.y1 !== edge.y2).toBe(true);
    }
  });

  it('renders the same field twice, so the shell re-rendering never reshuffles it', () => {
    const first = render(<LatticeBackdrop />);
    const before = edges(first.container);
    cleanup();
    const second = render(<LatticeBackdrop />);

    expect(edges(second.container)).toEqual(before);
  });

  it('marks only coarse-grid nodes for the accent pulse', () => {
    const { container } = render(<LatticeBackdrop />);
    const pulses = [...container.querySelectorAll('.lattice-pulse')];

    expect(pulses.length).toBeGreaterThan(0);
    for (const pulse of pulses) {
      expect(Number(pulse.getAttribute('cx')) % 96).toBe(0);
      expect(Number(pulse.getAttribute('cy')) % 96).toBe(0);
    }
  });

  it('spreads walks across the field rather than clumping', () => {
    const xs = buildLattice().nodes.map((node) => node.x);
    expect(Math.min(...xs)).toBeLessThan(200);
    expect(Math.max(...xs)).toBeGreaterThan(1000);
  });

  it('reshapes the field for a different seed', () => {
    expect(buildLattice(1).edges).not.toEqual(buildLattice(2).edges);
  });
});
