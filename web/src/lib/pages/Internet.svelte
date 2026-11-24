<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const usbTether = lazyPage(() => import('./USBTetherCard.svelte'));
const failover = lazyPage(() => import('./FailoverCard.svelte'));
const dataUsage = lazyPage(() => import('./DataUsageCard.svelte'));
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_internet()} description={m.network_intro()} />
  <PageHost load={usbTether} />
  <PageHost load={failover} />
  <PageHost load={dataUsage} />
</div>
