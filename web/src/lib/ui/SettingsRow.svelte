<script lang="ts">
import type { Snippet } from 'svelte';
import Label from '$lib/components/ui/label/label.svelte';

// Label and description left, control right; a control too wide for the row wraps below it. Consecutive rows get
// a divider, so a section never wraps them by hand.
let {
  label,
  description,
  for: forId,
  control,
  below,
  top = false,
}: {
  label: string;
  description?: string;
  for?: string;
  control: Snippet;
  /** Under the description, full width of the text column: a load error with its Retry. */
  below?: Snippet;
  /** Top-align label and control, for a control taller than one line (textarea). */
  top?: boolean;
} = $props();
</script>

<div
  data-slot="settings-row"
  class="flex flex-wrap justify-between gap-x-6 gap-y-2 py-3 {top ? 'items-start' : 'items-center'} [[data-slot=settings-row]+&]:border-t [[data-slot=settings-row]+&]:border-border"
>
  <!-- Phones: a small control (switch, chip) stays on the right; a wide one wraps below. -->
  <div class="grid min-w-0 flex-1 basis-48 gap-1">
    {#if forId}
      <Label for={forId} class="break-words">{label}</Label>
    {:else}
      <span class="text-sm leading-none font-medium break-words">{label}</span>
    {/if}
    {#if description}<p class="text-sm text-muted-foreground">{description}</p>{/if}
    {#if below}{@render below()}{/if}
  </div>
  <div class="flex flex-wrap items-center gap-2">{@render control()}</div>
</div>
