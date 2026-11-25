<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';

// Each card is its own chunk; the page shell stays small.
import LANAddressCard from './LANAddressCard.svelte';
import NetworkInterfacesCard from './NetworkInterfacesCard.svelte';
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_lan()} description={m.network_lan_description()} />
  <LANAddressCard />
  <NetworkInterfacesCard />
</div>
