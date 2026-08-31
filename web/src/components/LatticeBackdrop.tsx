import './lattice-backdrop.css';

/**
 * Lattice variant: a blueprint grid with a node network built along it.
 *
 * The two are one field rather than two stacked decorations, and three rules
 * are what make them read that way. Nodes sit on the grid, so every edge lies
 * on a grid line or its 45 degree diagonal. The network drifts as one rigid
 * body over the same period and distance as the grid's own pan, so no edge
 * parts from its endpoints. A single light sweep crosses both, and the accent
 * pulse runs on the sweep's period rather than one of its own.
 */

/** Field the SVG is drawn in. The band scales it with `preserveAspectRatio`. */
const fieldWidth = 1200;
const fieldHeight = 360;

/** Fine grid the walks step along, and the coarse grid whose nodes pulse. */
const cell = 24;
const coarseCell = 96;

const walkCount = 10;
const stepsPerWalk = 7;
const maxStepCells = 4;

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
 * Change the seed to reshape the field; the generator is deterministic, so the
 * same seed always yields the same field.
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

export type LatticeEdge = { x1: number; y1: number; x2: number; y2: number };
export type LatticeNode = { x: number; y: number; coarse: boolean };
export type LatticeField = { edges: LatticeEdge[]; nodes: LatticeNode[] };

const snap = (value: number, limit: number) =>
  Math.max(0, Math.min(limit, Math.round(value / cell) * cell));

/**
 * Walk the grid, emitting an edge per step and a node at every turn.
 *
 * One walk starts in each vertical band of the field, so coverage reaches the
 * left nav, the page title and the right edge instead of clumping where the
 * generator happened to land.
 */
export function buildLattice(from = seed): LatticeField {
  const next = generator(from);
  const edges: LatticeEdge[] = [];
  const points = new Map<string, LatticeNode>();

  const remember = (x: number, y: number) => {
    const key = `${x},${y}`;
    if (!points.has(key)) points.set(key, { x, y, coarse: x % coarseCell === 0 && y % coarseCell === 0 });
  };

  const band = fieldWidth / walkCount;
  for (let walk = 0; walk < walkCount; walk += 1) {
    let x = snap(walk * band + next() * band, fieldWidth);
    let y = snap(next() * fieldHeight, fieldHeight);
    remember(x, y);

    for (let step = 0; step < stepsPerWalk; step += 1) {
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

      edges.push({ x1: x, y1: y, x2: toX, y2: toY });
      remember(toX, toY);
      x = toX;
      y = toY;
    }
  }

  return { edges, nodes: [...points.values()] };
}

/*
 * Built once, at module load. Generating per render would reshuffle the field
 * on every shell update — the stability Neon's literal array gives for free.
 */
const lattice = buildLattice();

export function LatticeBackdrop() {
  return (
    <>
      <div className="lattice-grid" />
      <svg
        className="lattice-network"
        viewBox={`0 0 ${fieldWidth} ${fieldHeight}`}
        preserveAspectRatio="xMidYMid slice"
      >
        {lattice.edges.map((edge, index) => (
          <line key={index} x1={edge.x1} y1={edge.y1} x2={edge.x2} y2={edge.y2} />
        ))}
        {/*
          * Nodes are grouped so their halo is one filter pass over the whole
          * set rather than one per circle. Coarse junctions carry their own,
          * brighter filter on top of it.
          */}
        <g className="lattice-nodes">
          {lattice.nodes.map((node, index) => (
            <circle
              className={node.coarse ? 'lattice-node lattice-pulse' : 'lattice-node'}
              key={index}
              cx={node.x}
              cy={node.y}
              r={node.coarse ? 3.4 : 2.2}
              style={node.coarse ? { animationDelay: `-${(index * 0.7).toFixed(1)}s` } : undefined}
            />
          ))}
        </g>
      </svg>
      <div className="lattice-sweep" />
    </>
  );
}
