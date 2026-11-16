<script lang="ts">
import type { Snippet } from 'svelte';
import Root from '$lib/components/ui/card/card.svelte';
import Content from '$lib/components/ui/card/card-content.svelte';
import Header from '$lib/components/ui/card/card-header.svelte';

let { title, children }: { title?: string; children: Snippet } = $props();
</script>

<Root>
  {#if title}
    <Header>
      <h2 class="text-base leading-normal font-medium">{title}</h2>
    </Header>
  {/if}
  <Content>{@render children()}</Content>
</Root>
