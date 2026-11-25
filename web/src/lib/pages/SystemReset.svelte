<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';

// Each card is its own chunk; the page shell stays small.
import SystemFactoryResetCard from './SystemFactoryResetCard.svelte';
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_system_reset()} description={m.system_reset_description()} />
  <SystemFactoryResetCard />
</div>
