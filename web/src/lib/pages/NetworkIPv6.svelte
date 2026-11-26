<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';
import DDNSCard from './DDNSCard.svelte';
// Each card is its own chunk; the page shell stays small.
import IPv6Card from './IPv6Card.svelte';
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_ipv6()} description={m.network_ipv6_description()} />
  <IPv6Card />
  <DDNSCard />
</div>
