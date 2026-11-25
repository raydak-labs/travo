<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';

// Each card is its own chunk; the page shell stays small.
import DHCPServerCard from './DHCPServerCard.svelte';
import DNSNamesCard from './DNSNamesCard.svelte';
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_dhcp()} description={m.network_dhcp_description()} />
  <DHCPServerCard />
  <DNSNamesCard />
</div>
