<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const lanAddress = lazyPage(() => import('./LANAddressCard.svelte'));
const interfaces = lazyPage(() => import('./NetworkInterfacesCard.svelte'));
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_lan()} description={m.network_lan_description()} />
  <PageHost load={lanAddress} />
  <PageHost load={interfaces} />
</div>
