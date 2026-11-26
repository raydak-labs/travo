<script lang="ts">
import type { StatusKind } from './status';

// A compact square-cornered outline badge for the network map: the colour carries the state
// (green on, grey off, amber or red only for a problem) and the label stays one word. The state
// is spelled out for screen readers and in the tooltip, so colour is never the only signal.
let {
  label,
  state,
  kind,
  href,
}: { label: string; state: string; kind: StatusKind; href?: string } = $props();

const tone: Record<StatusKind, string> = {
  ok: 'border-ok/60 text-ok',
  info: 'border-info/60 text-info',
  warn: 'border-warn/60 text-warn',
  danger: 'border-danger/60 text-danger',
  neutral: 'border-border text-muted-foreground',
  stale: 'border-border text-muted-foreground opacity-60',
};
const cls = $derived(
  `inline-flex items-center rounded-[2px] border px-[5px] py-[3px] text-[11px] leading-none font-medium whitespace-nowrap ${tone[kind]}`,
);
</script>

{#if href}
  <a {href} class="{cls} hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none" title="{label}: {state}" data-kind={kind}>
    {label}<span class="sr-only">: {state}</span>
  </a>
{:else}
  <span class={cls} title="{label}: {state}" data-kind={kind}>{label}<span class="sr-only">: {state}</span></span>
{/if}
