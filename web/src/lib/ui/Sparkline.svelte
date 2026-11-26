<script lang="ts">
import { type Frame, type Sample, sparklinePath } from '../stats-history';

// Hand-written on purpose (#232): the shadcn-svelte chart (layerchart) measured 74 KB gzip
// against the 16 KB chunk budget. If richer charts (axes, tooltips, stacking) are wanted
// later, switch to a chart library (shadcn-svelte chart / layerchart, or uPlot) and revisit
// the size budget then instead of growing this component.
let {
  samples,
  frame,
  label,
}: {
  samples: readonly Sample[];
  frame: Omit<Frame, 'width' | 'height'>;
  label: string;
} = $props();

const WIDTH = 300;
const HEIGHT = 40;
const d = $derived(sparklinePath(samples, { ...frame, width: WIDTH, height: HEIGHT }));
</script>

<svg
  viewBox="-2 -2 {WIDTH + 4} {HEIGHT + 4}"
  preserveAspectRatio="none"
  role="img"
  aria-label={label}
  class="h-10 w-full text-primary"
>
  <title>{label}</title>
  <line
    x1="0"
    y1={HEIGHT}
    x2={WIDTH}
    y2={HEIGHT}
    class="stroke-border"
    vector-effect="non-scaling-stroke"
  />
  <path
    {d}
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    vector-effect="non-scaling-stroke"
  />
</svg>
