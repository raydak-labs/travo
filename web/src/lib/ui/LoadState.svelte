<script lang="ts">
import CircleAlert from '@lucide/svelte/icons/circle-alert';
import type { Snippet } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Skeleton from '$lib/components/ui/skeleton/skeleton.svelte';
import { m } from '../paraglide/messages.js';
import type { LoadStatus } from './loader.svelte';
import Status from './Status.svelte';

// stale: a value is already known, so a failed refresh dims it instead of blanking it.
let {
  status,
  retry,
  stale = false,
  children,
}: { status: LoadStatus; retry: () => void; stale?: boolean; children: Snippet } = $props();
</script>

{#if status.state === 'loading' && stale}
  <!-- a retry of a known value: keep it, dimmed, instead of a skeleton -->
  <div class="opacity-60" role="status" aria-live="polite" aria-busy="true">
    <span class="sr-only">{m.load_loading()}</span>
    {@render children()}
  </div>
{:else if status.state === 'loading'}
  <div role="status" aria-live="polite" aria-busy="true">
    <span class="sr-only">{m.load_loading()}</span>
    <Skeleton class="h-5 w-40" />
  </div>
{:else if status.state === 'error'}
  {#if stale}
    <div class="opacity-60">{@render children()}</div>
  {/if}
  <!-- Plain row, not an Alert box: the error already sits inside its Section card. -->
  <div role="alert" class="flex flex-wrap items-center gap-3">
    {#if stale}
      <Status kind="stale">{m.load_stale()}</Status>
    {/if}
    <span class="flex min-w-0 items-start gap-1.5 text-sm text-danger">
      <CircleAlert aria-hidden="true" size={16} class="mt-0.5 shrink-0" />
      <span class="min-w-0">{m.load_error({ cause: status.message })}</span>
    </span>
    <Button variant="outline" size="sm"
      type="button"
      class="text-sm"
      onclick={retry}
    >
      {m.load_retry()}
    </Button>
  </div>
{:else}
  {@render children()}
{/if}
