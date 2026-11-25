<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';

// Each card is its own chunk; the page shell stays small.
import WANCard from './WANCard.svelte';
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_wan()} description={m.network_wan_description()} />
  <WANCard />
</div>
