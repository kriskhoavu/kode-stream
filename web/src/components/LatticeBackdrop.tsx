import './lattice-backdrop.css';

/**
 * Lattice variant: a blueprint grid with a node network built along it.
 *
 * Grid and network live in one SVG group and are translated together, so they
 * are locked by construction rather than by two animations that happen to
 * share a period. That also fixes an alignment bug: the grid used to be a CSS
 * background at device scale while the network was an SVG scaled to fit the
 * band, so the two only lined up when the band happened to be exactly
 * `fieldWidth` wide.
 *
 * Nodes sit on the grid, so every edge lies on a grid line or its 45 degree
 * diagonal. Highlights land on the nodes where edges actually converge. A walk fades along its length and the whole field fades toward its
 * top and bottom edges, so nothing ends on a hard cut when `slice` crops the
 * field to a band shorter than itself.
 */

/*
 * Field the SVG is drawn in. The height tracks the band's own height so
 * `slice` has little to crop; the fades cover whatever is left at widths where
 * it still crops.
 */
const fieldWidth = 1200;
const fieldHeight = 240;

/** Fine grid the walks step along, and the coarse grid whose nodes are junctions. */
const cell = 24;
const coarseCell = 96;

const walkCount = 12;
const minSteps = 4;
const maxSteps = 10;
const maxStepCells = 4;

/** Distance over which the field fades into its top and bottom edges. */
const edgeFade = 56;

/** Orthogonal and diagonal only: any other angle leaves the grid. */
const directions = [
  [1, 0], [-1, 0], [0, 1], [0, -1],
  [1, 1], [-1, 1], [1, -1], [-1, -1]
] as const;

/*
 * Neon commits its 29 placements as a literal because they are readable as
 * one. A lattice cannot: its meaning is in the walk structure, which a flat
 * list of coordinates hides. So the field is generated from this seed instead,
 * and the geometric properties that make it a lattice are asserted in tests.
 */
const seed = 20260830;

/** Park-Miller LCG. Deterministic, and enough randomness for a decorative walk. */
function generator(from: number) {
  let state = from % 2147483647;
  return () => {
    state = (state * 16807) % 2147483647;
    return state / 2147483647;
  };
}

export type LatticeEdge = { x1: number; y1: number; x2: number; y2: number; opacity: number };
export type LatticeNode = { x: number; y: number; junction: boolean; degree: number; r: number; opacity: number };
export type LatticeField = { edges: LatticeEdge[]; nodes: LatticeNode[] };

const clamp = (v: number, lo: number, hi: number) => Math.max(lo, Math.min(hi, v));
const snap = (value: number, limit: number) => clamp(Math.round(value / cell) * cell, 0, limit);

/** Fades a mark toward the field's top and bottom, so a crop never cuts hard. */
function edgeWeight(y: number) {
  return clamp(Math.min(y, fieldHeight - y) / edgeFade, 0.18, 1);
}

/**
 * Walk the grid, emitting an edge per step and a node at every turn.
 *
 * One walk starts in each vertical band, so coverage reaches the left nav, the
 * page title and the right edge instead of clumping. Walks vary in length: a
 * field of identically long runs reads as a pattern rather than a drawing.
 */
export function buildLattice(from = seed): LatticeField {
  const next = generator(from);
  const edges: LatticeEdge[] = [];
  const points = new Map<string, { x: number; y: number; degree: number; opacity: number }>();

  const touch = (x: number, y: number, along: number) => {
    const key = `${x},${y}`;
    // A walk fades along its length, so it trails off rather than stopping.
    const opacity = (1 - along * 0.55) * edgeWeight(y);
    const existing = points.get(key);
    if (existing) {
      existing.degree += 1;
      // Where walks cross, the brighter claim wins.
      existing.opacity = Math.max(existing.opacity, opacity);
    } else {
      points.set(key, { x, y, degree: 1, opacity });
    }
  };

  const band = fieldWidth / walkCount;
  for (let walk = 0; walk < walkCount; walk += 1) {
    let x = snap(walk * band + next() * band, fieldWidth);
    let y = snap(edgeFade + next() * (fieldHeight - edgeFade * 2), fieldHeight);
    const steps = minSteps + Math.floor(next() * (maxSteps - minSteps + 1));

    for (let step = 0; step < steps; step += 1) {
      const [dx, dy] = directions[Math.floor(next() * directions.length)];

      /*
       * Shorten the step to fit the field rather than clamping each axis on
       * its own: clamping one axis of a diagonal leaves an edge at an
       * arbitrary angle, which is exactly what puts the network off the grid.
       */
      let cells = 1 + Math.floor(next() * maxStepCells);
      if (dx > 0) cells = Math.min(cells, (fieldWidth - x) / cell);
      if (dx < 0) cells = Math.min(cells, x / cell);
      if (dy > 0) cells = Math.min(cells, (fieldHeight - y) / cell);
      if (dy < 0) cells = Math.min(cells, y / cell);
      // No room left in this direction; try another rather than emitting nothing.
      if (cells < 1) continue;

      const toX = x + dx * cells * cell;
      const toY = y + dy * cells * cell;
      const along = (step + 1) / steps;

      edges.push({
        x1: x, y1: y, x2: toX, y2: toY,
        opacity: (1 - along * 0.6) * edgeWeight((y + toY) / 2)
      });
      touch(x, y, along);
      touch(toX, toY, along);
      x = toX;
      y = toY;
    }
  }

  /*
   * Highlights land where the drawing is actually dense: a node three or more
   * edges meet, or one sitting on a coarse-grid intersection where the network
   * touches the blueprint's major rules. Both are structural. Marking arbitrary
   * grid coincidences alone gave three highlights across the whole field, most
   * of them where nothing was happening.
   */
  const nodes: LatticeNode[] = [...points.values()].map((point) => {
    const onCoarse = point.x % coarseCell === 0 && point.y % coarseCell === 0;
    const junction = point.degree >= 3 || onCoarse;
    return {
      x: point.x,
      y: point.y,
      degree: point.degree,
      junction,
      // Bigger where more edges converge, so weight follows the structure.
      r: junction ? 2.9 + Math.min(point.degree, 5) * 0.16 : 2.1,
      opacity: point.opacity
    };
  });

  return { edges, nodes };
}

/*
 * Built once, at module load. Generating per render would reshuffle the field
 * on every shell update — the stability Neon's literal array gives for free.
 */
const lattice = buildLattice();

const gridSpan = { x: -coarseCell * 2, y: -coarseCell * 2, w: fieldWidth + coarseCell * 4, h: fieldHeight + coarseCell * 4 };

export function LatticeBackdrop() {
  return (
    <>
      <svg
        className="lattice-field"
        viewBox={`0 0 ${fieldWidth} ${fieldHeight}`}
        preserveAspectRatio="xMidYMid slice"
      >
        <defs>
          <pattern id="lattice-fine" width={cell} height={cell} patternUnits="userSpaceOnUse">
            <path className="lattice-rule-fine" d={`M ${cell} 0 H 0 V ${cell}`} fill="none" />
          </pattern>
          <pattern id="lattice-coarse" width={coarseCell} height={coarseCell} patternUnits="userSpaceOnUse">
            <rect width={coarseCell} height={coarseCell} fill="url(#lattice-fine)" />
            <path className="lattice-rule-coarse" d={`M ${coarseCell} 0 H 0 V ${coarseCell}`} fill="none" />
          </pattern>
        </defs>

        {/*
          * Grid and network in one translated group, so they cannot drift apart.
          * The grid rect overhangs the field by two coarse cells, which is what
          * keeps the pan seamless as the group travels.
          */}
        <g className="lattice-drift">
          <rect
            className="lattice-grid"
            x={gridSpan.x}
            y={gridSpan.y}
            width={gridSpan.w}
            height={gridSpan.h}
            fill="url(#lattice-coarse)"
          />
          <g className="lattice-edges">
            {lattice.edges.map((edge, index) => (
              <line key={index} x1={edge.x1} y1={edge.y1} x2={edge.x2} y2={edge.y2} opacity={edge.opacity.toFixed(3)} />
            ))}
          </g>
          {/*
            * Nodes are grouped so their halo is one filter pass over the whole
            * set rather than one per circle. Junctions carry their own,
            * brighter filter on top of it.
            */}
          <g className="lattice-nodes">
            {lattice.nodes.map((node, index) => (
              <circle
                className={node.junction ? 'lattice-node lattice-pulse' : 'lattice-node'}
                key={index}
                cx={node.x}
                cy={node.y}
                r={node.r.toFixed(2)}
                opacity={node.opacity.toFixed(3)}
                style={node.junction ? { animationDelay: `-${(index * 0.7).toFixed(1)}s` } : undefined}
              />
            ))}
          </g>
        </g>
      </svg>
      <div className="lattice-sweep" />
    </>
  );
}
