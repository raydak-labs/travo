<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const ipv6 = lazyPage(() => import('./IPv6Card.svelte'));
const ddns = lazyPage(() => import('./DDNSCard.svelte'));
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_ipv6()} description={m.network_ipv6_description()} />
  <PageHost load={ipv6} />
  <PageHost load={ddns} />
</div>
