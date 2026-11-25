<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';
import DataUsageCard from './DataUsageCard.svelte';
import FailoverCard from './FailoverCard.svelte';
// Each card is its own chunk; the page shell stays small.
import USBTetherCard from './USBTetherCard.svelte';
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_internet()} description={m.network_intro()} />
  <USBTetherCard />
  <FailoverCard />
  <DataUsageCard />
</div>
