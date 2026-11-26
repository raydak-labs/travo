<script lang="ts">
import type { Snippet } from 'svelte';
import Root from '$lib/components/ui/card/card.svelte';
import Action from '$lib/components/ui/card/card-action.svelte';
import Content from '$lib/components/ui/card/card-content.svelte';
import Description from '$lib/components/ui/card/card-description.svelte';
import Footer from '$lib/components/ui/card/card-footer.svelte';
import Header from '$lib/components/ui/card/card-header.svelte';

// One card surface per page section; its title is the section's h2 (PageHeader owns the h1).
let {
  title,
  description,
  actions,
  footer,
  children,
}: {
  title: string;
  description?: string;
  actions?: Snippet;
  /** Bottom right of the card: the one Save of an edit form. */
  footer?: Snippet;
  children?: Snippet;
} = $props();
const headingId = $props.id();
</script>

<!-- Phones get the compact card spacing so content keeps most of the 390 px width. -->
<Root role="region" aria-labelledby={headingId} class="gap-4 max-sm:[--card-spacing:--spacing(4)]">
  <Header>
    <h2 id={headingId} class="text-base leading-normal font-semibold">{title}</h2>
    {#if description}<Description>{description}</Description>{/if}
    {#if actions}<Action class="flex flex-wrap gap-2">{@render actions()}</Action>{/if}
  </Header>
  <!-- Empty blocks (closed aria-live regions) take no gap; content that renders nothing hides. -->
  {#if children}<Content class="gap-4 [&:not(:has(>:not(:empty)))]:hidden [&>:empty]:hidden">{@render children()}</Content>{/if}
  {#if footer}<Footer class="justify-end gap-2 [&:not(:has(>*))]:hidden">{@render footer()}</Footer>{/if}
</Root>
