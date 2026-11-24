<script lang="ts">
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const dhcpServer = lazyPage(() => import('./DHCPServerCard.svelte'));
const dnsNames = lazyPage(() => import('./DNSNamesCard.svelte'));
</script>

<div class="grid gap-4">
  <PageHost load={dhcpServer} />
  <PageHost load={dnsNames} />
</div>
