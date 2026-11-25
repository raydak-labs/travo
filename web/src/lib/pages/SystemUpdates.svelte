<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';
import SystemFirmwareCard from './SystemFirmwareCard.svelte';

// Each card is its own chunk; the page shell stays small.
import SystemOpenWrtCard from './SystemOpenWrtCard.svelte';
import SystemUpdate from './SystemUpdate.svelte';
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_system_updates()} description={m.system_updates_description()} />
  <SystemUpdate />
  <SystemOpenWrtCard />
  <SystemFirmwareCard />
</div>
