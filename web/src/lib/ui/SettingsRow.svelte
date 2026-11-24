<script lang="ts">
import type { Snippet } from 'svelte';
import Label from '$lib/components/ui/label/label.svelte';

// Label and description left, control right from `sm`; stacked on phones. Consecutive rows get
// a divider, so a section never wraps them by hand.
let {
  label,
  description,
  for: forId,
  control,
}: { label: string; description?: string; for?: string; control: Snippet } = $props();
</script>

<div
  data-slot="settings-row"
  class="flex flex-col gap-2 py-3 sm:flex-row sm:items-center sm:justify-between sm:gap-6 [[data-slot=settings-row]+&]:border-t [[data-slot=settings-row]+&]:border-border"
>
  <div class="grid min-w-0 gap-1">
    {#if forId}
      <Label for={forId} class="break-words">{label}</Label>
    {:else}
      <span class="text-sm leading-none font-medium break-words">{label}</span>
    {/if}
    {#if description}<p class="text-sm text-muted-foreground">{description}</p>{/if}
  </div>
  <div class="flex flex-wrap items-center gap-2 sm:shrink-0 sm:justify-end">{@render control()}</div>
</div>
