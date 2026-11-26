<script lang="ts">
import { m } from '../paraglide/messages.js';
import CardGrid from '../ui/CardGrid.svelte';
import PageHeader from '../ui/PageHeader.svelte';
import DiagnosticsCard from './DiagnosticsCard.svelte';
import NetworkInterfacesCard from './NetworkInterfacesCard.svelte';
import WakeOnLanCard from './WakeOnLanCard.svelte';
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_diagnostics()} description={m.network_diagnostics_description()} />
  <CardGrid cols={2}>
    <DiagnosticsCard />
    <WakeOnLanCard />
  </CardGrid>
  <NetworkInterfacesCard />
</div>
