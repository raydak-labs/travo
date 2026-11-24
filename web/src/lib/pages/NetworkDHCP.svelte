<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const dhcpServer = lazyPage(() => import('./DHCPServerCard.svelte'));
const dnsNames = lazyPage(() => import('./DNSNamesCard.svelte'));
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_dhcp()} description={m.network_dhcp_description()} />
  <PageHost load={dhcpServer} />
  <PageHost load={dnsNames} />
</div>
