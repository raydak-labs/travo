<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const portForwards = lazyPage(() => import('./PortForwardsCard.svelte'));
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_port_forwards()} description={m.network_port_forwards_description()} />
  <PageHost load={portForwards} />
</div>
