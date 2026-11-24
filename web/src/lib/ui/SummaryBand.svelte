<script lang="ts">
import type { Snippet } from 'svelte';

// Flat tiles between PageHeader and the first Section. The hidden h2 keeps headings h1 → h2 → h3.
let { title, children }: { title: string; children: Snippet } = $props();
const headingId = $props.id();
</script>

<section class="grid gap-3 sm:grid-cols-3" aria-labelledby={headingId}>
  <h2 id={headingId} class="sr-only">{title}</h2>
  {@render children()}
</section>
