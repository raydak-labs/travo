<script lang="ts">
import { m } from '../paraglide/messages.js';
import CardGrid from '../ui/CardGrid.svelte';
import GroupHeading from '../ui/GroupHeading.svelte';
import PageHeader from '../ui/PageHeader.svelte';
import DiagnosticsCard from './DiagnosticsCard.svelte';
import NetworkInterfacesCard from './NetworkInterfacesCard.svelte';
import WakeOnLanCard from './WakeOnLanCard.svelte';
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_diagnostics()} description={m.network_diagnostics_description()} />
  <div class="flex flex-col gap-3">
    <GroupHeading level={2} label={m.network_diagnostics_group_tools()} />
    <CardGrid cols={2}>
      <DiagnosticsCard />
      <WakeOnLanCard />
    </CardGrid>
  </div>
  <div class="flex flex-col gap-3">
    <GroupHeading level={2} label={m.network_diagnostics_group_state()} />
    <div class="flex flex-col gap-6"><NetworkInterfacesCard /></div>
  </div>
</div>
