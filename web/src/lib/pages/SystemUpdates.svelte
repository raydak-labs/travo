<script lang="ts">
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';
import SystemUpdate from './SystemUpdate.svelte';

// Each card is its own chunk; the page shell stays small.
const openWrt = lazyPage(() => import('./SystemOpenWrtCard.svelte'));
const firmware = lazyPage(() => import('./SystemFirmwareCard.svelte'));
</script>

<div class="grid gap-4">
  <SystemUpdate />
  <PageHost load={openWrt} />
  <PageHost load={firmware} />
</div>
