<script lang="ts">
import type { Snippet } from 'svelte';
import Alert from '$lib/components/ui/alert/alert.svelte';
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
  <Alert class="flex flex-wrap items-center gap-3">
    {#if stale}
      <Status kind="stale">{m.load_stale()}</Status>
    {/if}
    <Status kind="danger">{m.load_error({ cause: status.message })}</Status>
    <Button variant="outline" size="sm"
      type="button"
      class="text-sm"
      onclick={retry}
    >
      {m.load_retry()}
    </Button>
  </Alert>
{:else}
  {@render children()}
{/if}
