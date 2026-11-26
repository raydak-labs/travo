<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';

// Each card is its own chunk; the page shell stays small.
import PortForwardsCard from './PortForwardsCard.svelte';
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_port_forwards()} description={m.network_port_forwards_description()} />
  <PortForwardsCard />
</div>
