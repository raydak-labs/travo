<script lang="ts">
import type { Component, Snippet } from 'svelte';

// "Nothing here yet" content. A failed load is never empty: LoadState shows that.
let {
  icon: Icon,
  title,
  text,
  action,
}: {
  icon: Component<{ size?: number; class?: string; 'aria-hidden'?: 'true' }>;
  title: string;
  text?: string;
  action?: Snippet;
} = $props();
</script>

<div data-slot="empty-state" class="flex items-start gap-3 py-2">
  <Icon aria-hidden="true" size={18} class="mt-0.5 shrink-0 text-muted-foreground" />
  <div class="grid gap-1">
    <p class="max-w-prose text-sm font-medium">{title}</p>
    {#if text}<p class="max-w-prose text-sm text-muted-foreground">{text}</p>{/if}
    {#if action}<div class="mt-1">{@render action()}</div>{/if}
  </div>
</div>
