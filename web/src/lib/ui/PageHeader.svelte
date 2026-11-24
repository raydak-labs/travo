<script lang="ts">
import type { Snippet } from 'svelte';

// The page's one h1. Actions sit right of the title from `sm`, below it on phones.
let { title, description, actions }: { title: string; description?: string; actions?: Snippet } =
  $props();
</script>

<header class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between sm:gap-6">
  <div class="grid min-w-0 gap-1">
    <h1 class="text-xl font-semibold">{title}</h1>
    {#if description}<p class="text-sm text-muted-foreground">{description}</p>{/if}
  </div>
  {#if actions}<div class="flex shrink-0 flex-wrap gap-2">{@render actions()}</div>{/if}
</header>
