<script lang="ts">
import { m } from '../paraglide/messages.js';
import CardGrid from '../ui/CardGrid.svelte';
import PageHeader from '../ui/PageHeader.svelte';
import DHCPServerCard from './DHCPServerCard.svelte';
import DNSNamesCard from './DNSNamesCard.svelte';
import LANAddressCard from './LANAddressCard.svelte';

// LAN: the router's address, the DHCP pool inside it and the local DNS names, on one page.
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_lan()} description={m.network_lan_description()} />
  <CardGrid cols={2}>
    <LANAddressCard />
    <DHCPServerCard />
  </CardGrid>
  <DNSNamesCard />
</div>
