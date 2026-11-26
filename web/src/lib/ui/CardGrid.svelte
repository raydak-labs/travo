<script lang="ts">
import type { Snippet } from 'svelte';

// Related short cards side by side: up to `cols` per row on wide screens, two on medium, one on
// phones. Long lists, tables and long forms stay outside it at full width (docs/web-ui.md).
let { cols = 3, children }: { cols?: 2 | 3; children: Snippet } = $props();
</script>

<div class="grid items-start gap-6 lg:grid-cols-2 {cols === 3 ? 'xl:grid-cols-3' : ''}">
  {@render children()}
</div>
