<script lang="ts">
import type { Snippet } from 'svelte';

// Flat tiles between PageHeader and the first Section. The hidden h2 keeps headings h1 → h2 → h3.
let { title, cols = 3, children }: { title: string; cols?: 3 | 4; children: Snippet } = $props();
const headingId = $props.id();
</script>

<section class="grid gap-3 {cols === 4 ? 'grid-cols-2 lg:grid-cols-4' : 'sm:grid-cols-3'}" aria-labelledby={headingId}>
  <h2 id={headingId} class="sr-only">{title}</h2>
  {@render children()}
</section>
