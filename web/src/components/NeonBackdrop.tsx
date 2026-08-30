import { Bot, Boxes, Database, FileCode2, GitBranch, KanbanSquare, Key, Layers, Plug, Sparkles, Terminal, Workflow, Wrench, BookOpen } from 'lucide-react';
import './neon-backdrop.css';

type NeonIcon = {
  Icon: typeof GitBranch;
  /** Horizontal placement across the band, in percent. */
  x: number;
  /** Vertical placement within the band, in percent. */
  y: number;
  size: number;
  /** HSL hue for the neon stroke and glow. */
  hue: number;
  /** Drift cycle length in seconds. */
  duration: number;
  delay: number;
};

/*
 * Placements are literal rather than random so the field is stable across
 * renders and reviewable in a diff. Density leans on the outer thirds; the
 * middle keeps only a few small icons so the page title, global search and
 * filter controls stay legible on top of it.
 */
const icons: NeonIcon[] = [
  /*
   * Left-nav band (x under ~18%): the sidebar is only 232px wide and densely
   * packed, so these dodge the brand row and the three nav buttons vertically
   * and sit in the gaps between them.
   */
  { Icon: Sparkles, x: 4, y: 4, size: 18, hue: 176, duration: 14, delay: 1.2 },
  { Icon: Layers, x: 3, y: 20, size: 26, hue: 190, duration: 13, delay: 0 },
  { Icon: Terminal, x: 13, y: 17, size: 20, hue: 284, duration: 16, delay: 4.1 },
  { Icon: Wrench, x: 11, y: 34, size: 20, hue: 338, duration: 11, delay: 1.1 },
  { Icon: Key, x: 5, y: 45, size: 19, hue: 48, duration: 18, delay: 2.9 },
  { Icon: GitBranch, x: 14, y: 52, size: 22, hue: 152, duration: 17, delay: 2.4 },
  { Icon: Boxes, x: 4, y: 68, size: 21, hue: 210, duration: 12, delay: 0.6 },
  { Icon: Bot, x: 12, y: 78, size: 19, hue: 316, duration: 19, delay: 3.7 },
  { Icon: Sparkles, x: 9, y: 90, size: 18, hue: 96, duration: 15, delay: 5.2 },

  { Icon: Plug, x: 16, y: 8, size: 24, hue: 268, duration: 15, delay: 0.7 },
  { Icon: Database, x: 19, y: 55, size: 19, hue: 205, duration: 12, delay: 4.2 },
  { Icon: Key, x: 23, y: 88, size: 21, hue: 96, duration: 18, delay: 1.9 },
  { Icon: Terminal, x: 27, y: 24, size: 23, hue: 24, duration: 14, delay: 3.1 },
  { Icon: FileCode2, x: 31, y: 70, size: 18, hue: 172, duration: 20, delay: 0.4 },
  { Icon: Boxes, x: 38, y: 6, size: 20, hue: 300, duration: 16, delay: 2.8 },
  { Icon: KanbanSquare, x: 43, y: 91, size: 17, hue: 214, duration: 13, delay: 5.0 },
  { Icon: Bot, x: 49, y: 15, size: 16, hue: 132, duration: 18, delay: 1.5 },
  { Icon: Workflow, x: 55, y: 89, size: 17, hue: 350, duration: 15, delay: 3.9 },
  { Icon: BookOpen, x: 61, y: 9, size: 20, hue: 52, duration: 12, delay: 0.2 },
  { Icon: Sparkles, x: 67, y: 66, size: 18, hue: 256, duration: 19, delay: 4.6 },
  { Icon: Plug, x: 72, y: 28, size: 22, hue: 186, duration: 14, delay: 2.1 },
  { Icon: Database, x: 76, y: 84, size: 19, hue: 320, duration: 17, delay: 0.9 },
  { Icon: Key, x: 80, y: 47, size: 21, hue: 40, duration: 11, delay: 3.4 },
  { Icon: GitBranch, x: 84, y: 13, size: 24, hue: 160, duration: 20, delay: 1.7 },
  { Icon: Terminal, x: 87, y: 74, size: 20, hue: 278, duration: 13, delay: 4.9 },
  { Icon: Wrench, x: 91, y: 38, size: 22, hue: 200, duration: 16, delay: 2.6 },
  { Icon: Boxes, x: 93, y: 89, size: 19, hue: 108, duration: 18, delay: 0.5 },
  { Icon: Layers, x: 96, y: 20, size: 25, hue: 344, duration: 12, delay: 3.3 },
  { Icon: FileCode2, x: 95, y: 58, size: 18, hue: 30, duration: 19, delay: 1.3 }
];

/**
 * Neon variant: a scattered icon field, each icon carrying its own hue, drift
 * and glow pulse. The band box, its height and its fade mask belong to
 * `HeaderBackdrop`; this component paints contents only.
 */
export function NeonBackdrop() {
  return (
    <>
      {icons.map(({ Icon, x, y, size, hue, duration, delay }, index) => (
        <span
          className="neon-icon"
          key={index}
          style={{
            left: `${x}%`,
            top: `${y}%`,
            animationDuration: `${duration}s, ${(duration * 0.7).toFixed(1)}s`,
            animationDelay: `${delay}s, ${(delay * 0.6).toFixed(1)}s`,
            ['--neon-hue' as string]: `${hue}`
          }}
        >
          <Icon size={size} strokeWidth={2} />
        </span>
      ))}
    </>
  );
}
