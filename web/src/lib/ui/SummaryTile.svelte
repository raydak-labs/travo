<script lang="ts">
import type { Snippet } from 'svelte';
import GroupHeading from './GroupHeading.svelte';

// One tile of a SummaryBand: card surface, GroupHeading label, own content (and own fallback).
let { label, children }: { label: string; children: Snippet } = $props();
</script>

<div data-slot="summary-tile" class="flex flex-col gap-2 rounded-xl bg-card p-4 text-sm ring-1 ring-foreground/10">
  <GroupHeading {label} />
  {@render children()}
</div>
