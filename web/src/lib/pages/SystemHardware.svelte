<script lang="ts">
import { m } from '../paraglide/messages.js';
import CardGrid from '../ui/CardGrid.svelte';
import PageHeader from '../ui/PageHeader.svelte';
import SystemButtonsCard from './SystemButtonsCard.svelte';
// Each card is its own chunk; the page shell stays small.
import SystemLEDsCard from './SystemLEDsCard.svelte';
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_system_hardware()} description={m.system_leds_description()} />
  <CardGrid cols={3}>
  <SystemLEDsCard />
  <SystemButtonsCard />
  </CardGrid>
</div>
