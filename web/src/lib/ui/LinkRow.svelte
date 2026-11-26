<script lang="ts">
import ChevronRight from '@lucide/svelte/icons/chevron-right';
import type { Snippet } from 'svelte';

// A settings row that opens another page: the whole row is the link (stretched over the row)
// and ends in a chevron. A control (switch, value) sits above the link, so using it does not
// navigate.
let {
  href,
  label,
  description,
  control,
}: { href: string; label: string; description?: string; control?: Snippet } = $props();
</script>

<div
  data-slot="settings-row"
  class="relative flex items-center gap-4 py-3 hover:bg-muted/40 [[data-slot=settings-row]+&]:border-t [[data-slot=settings-row]+&]:border-border"
>
  <div class="grid min-w-0 flex-1 gap-1">
    <a {href} class="text-sm leading-none font-medium break-words after:absolute after:inset-0 after:rounded-md focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-ring">{label}</a>
    {#if description}<p class="text-sm text-muted-foreground">{description}</p>{/if}
  </div>
  {#if control}<div class="relative z-10 flex shrink-0 items-center gap-2">{@render control()}</div>{/if}
  <ChevronRight aria-hidden="true" size={16} class="shrink-0 text-muted-foreground" />
</div>
