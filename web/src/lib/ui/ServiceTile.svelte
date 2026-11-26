<script lang="ts">
import type { Snippet } from 'svelte';

// One service on the Services overview: name and state on top, what it does, and its actions
// pinned to the bottom so tiles in a row line up.
let {
  title,
  description,
  status,
  actions,
}: { title: string; description?: string; status?: Snippet; actions?: Snippet } = $props();
const headingId = $props.id();
</script>

<li
  aria-labelledby={headingId}
  class="flex flex-col gap-2 rounded-xl border border-border bg-card p-4 text-card-foreground shadow-xs"
>
  <div class="flex items-start justify-between gap-2">
    <h3 id={headingId} class="text-sm font-semibold">{title}</h3>
    {#if status}<div class="flex shrink-0 flex-wrap justify-end gap-1">{@render status()}</div>{/if}
  </div>
  {#if description}<p class="flex-1 text-sm text-muted-foreground">{description}</p>{/if}
  {#if actions}<div class="mt-auto flex flex-wrap items-center gap-2 pt-2">{@render actions()}</div>{/if}
</li>
