<script lang="ts">
import LoadState from '../ui/LoadState.svelte';
import { createLoader, type LoadStatus } from '../ui/loader.svelte';
import type { PageImport } from './lazy';

// Keyed by route in App, so each route gets a fresh loader for its chunk.
let {
  load,
  props = {},
  onStatus,
}: {
  load: PageImport;
  props?: Record<string, unknown>;
  onStatus?: (status: LoadStatus) => void;
} = $props();

const page = createLoader(() => load());
const Page = $derived(page.data);

$effect(() => {
  onStatus?.(page.status);
});
</script>

<LoadState status={page.status} retry={page.retry}>
  {#if Page}
    <Page {...props} />
  {/if}
</LoadState>
