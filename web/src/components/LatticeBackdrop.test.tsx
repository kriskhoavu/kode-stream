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

  /*
   * A highlight has to mean something. Marking coarse-grid coincidences alone
   * lit three nodes across the whole field, most of them where no edges met.
   */
  it('highlights nodes where edges converge, or coarse-grid landmarks', () => {
    const { nodes } = buildLattice();
    const junctions = nodes.filter((node) => node.junction);

    expect(junctions.length).toBeGreaterThan(5);
    for (const node of junctions) {
      const onCoarse = node.x % 96 === 0 && node.y % 96 === 0;
      expect(node.degree >= 3 || onCoarse).toBe(true);
    }
    // At least some are real convergences rather than grid coincidences.
    expect(junctions.some((node) => node.degree >= 3)).toBe(true);
  });

  it('sizes a highlight by how many edges meet on it', () => {
    const { nodes } = buildLattice();
    const plain = nodes.filter((n) => !n.junction);
    const junctions = nodes.filter((n) => n.junction);

    expect(Math.max(...plain.map((n) => n.r))).toBeLessThan(Math.min(...junctions.map((n) => n.r)));
  });

  it('renders a highlight class for every junction', () => {
    const { container } = render(<LatticeBackdrop />);
    const expected = buildLattice().nodes.filter((n) => n.junction).length;
    expect(container.querySelectorAll('.lattice-pulse').length).toBe(expected);
  });

  it('spreads walks across the field rather than clumping', () => {
    const xs = buildLattice().nodes.map((node) => node.x);
    expect(Math.min(...xs)).toBeLessThan(200);
    expect(Math.max(...xs)).toBeGreaterThan(1000);
  });

  /*
   * `slice` crops the field to a band shorter than itself, so anything near the
   * top or bottom must already be faint or it ends on a hard cut — which is
   * exactly how this looked before the fades.
   */
  it('fades marks toward the top and bottom of the field', () => {
    const { nodes } = buildLattice();
    const edgeBand = nodes.filter((n) => Math.min(n.y, 240 - n.y) <= 24);
    const middle = nodes.filter((n) => Math.min(n.y, 240 - n.y) >= 96);

    expect(edgeBand.length).toBeGreaterThan(0);
    expect(middle.length).toBeGreaterThan(0);
    expect(Math.max(...edgeBand.map((n) => n.opacity)))
      .toBeLessThan(Math.max(...middle.map((n) => n.opacity)));
  });

  it('keeps every mark within a usable opacity range', () => {
    const { edges, nodes } = buildLattice();
    for (const mark of [...edges, ...nodes]) {
      expect(mark.opacity).toBeGreaterThan(0);
      expect(mark.opacity).toBeLessThanOrEqual(1);
    }
  });

  it('varies walk length, so the field does not read as a repeated pattern', () => {
    // Walks are the only source of edges, so a spread of per-walk counts shows
    // up as a spread of distinct edge opacities along each trail.
    const opacities = new Set(buildLattice().edges.map((e) => e.opacity.toFixed(3)));
    expect(opacities.size).toBeGreaterThan(20);
  });

  it('reshapes the field for a different seed', () => {
    expect(buildLattice(1).edges).not.toEqual(buildLattice(2).edges);
  });
});
