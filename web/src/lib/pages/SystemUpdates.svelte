<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';
import SystemUpdate from './SystemUpdate.svelte';

// Each card is its own chunk; the page shell stays small.
const openWrt = lazyPage(() => import('./SystemOpenWrtCard.svelte'));
const firmware = lazyPage(() => import('./SystemFirmwareCard.svelte'));
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_system_updates()} description={m.system_updates_description()} />
  <SystemUpdate />
  <PageHost load={openWrt} />
  <PageHost load={firmware} />
</div>
