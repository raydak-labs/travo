<script lang="ts">
import type { Snippet } from 'svelte';
import Root from '$lib/components/ui/card/card.svelte';
import Content from '$lib/components/ui/card/card-content.svelte';
import Header from '$lib/components/ui/card/card-header.svelte';

let { title, children }: { title?: string; children: Snippet } = $props();
</script>

<!-- Phones get the compact card spacing so content keeps most of the 390 px width. -->
<Root class="max-sm:[--card-spacing:--spacing(4)]">
  {#if title}
    <Header>
      <h2 class="text-base leading-normal font-semibold">{title}</h2>
    </Header>
  {/if}
  <Content>{@render children()}</Content>
</Root>
