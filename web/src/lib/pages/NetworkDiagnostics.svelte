<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const diagnostics = lazyPage(() => import('./DiagnosticsCard.svelte'));
const wakeOnLan = lazyPage(() => import('./WakeOnLanCard.svelte'));
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_network_diagnostics()} description={m.network_diagnostics_description()} />
  <PageHost load={diagnostics} />
  <PageHost load={wakeOnLan} />
</div>
