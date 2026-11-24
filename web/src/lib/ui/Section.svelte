<script lang="ts">
import type { Snippet } from 'svelte';
import Root from '$lib/components/ui/card/card.svelte';
import Action from '$lib/components/ui/card/card-action.svelte';
import Content from '$lib/components/ui/card/card-content.svelte';
import Description from '$lib/components/ui/card/card-description.svelte';
import Header from '$lib/components/ui/card/card-header.svelte';

// One card surface per page section; its title is the section's h2 (PageHeader owns the h1).
let {
  title,
  description,
  actions,
  children,
}: { title: string; description?: string; actions?: Snippet; children?: Snippet } = $props();
const headingId = $props.id();
</script>

<!-- Phones get the compact card spacing so content keeps most of the 390 px width. -->
<Root role="region" aria-labelledby={headingId} class="gap-4 max-sm:[--card-spacing:--spacing(4)]">
  <Header>
    <h2 id={headingId} class="text-base leading-normal font-semibold">{title}</h2>
    {#if description}<Description>{description}</Description>{/if}
    {#if actions}<Action class="flex flex-wrap gap-2">{@render actions()}</Action>{/if}
  </Header>
  {#if children}<Content class="gap-4">{@render children()}</Content>{/if}
</Root>
