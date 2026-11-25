<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';

// Each card is its own chunk; the page shell stays small.
import DiagnosticsCard from './DiagnosticsCard.svelte';
import WakeOnLanCard from './WakeOnLanCard.svelte';
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_diagnostics()} description={m.network_diagnostics_description()} />
  <DiagnosticsCard />
  <WakeOnLanCard />
</div>
