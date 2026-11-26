<script lang="ts">
import { m } from '../paraglide/messages.js';
import CardGrid from '../ui/CardGrid.svelte';
import PageHeader from '../ui/PageHeader.svelte';
import DataUsageCard from './DataUsageCard.svelte';
import FailoverCard from './FailoverCard.svelte';
import USBTetherCard from './USBTetherCard.svelte';
import WANCard from './WANCard.svelte';
import WiFi from './WiFi.svelte';

// Internet: every way the router gets online. WiFi uplinks first (the travel case), then the
// Ethernet WAN, USB tethering and failover two per row (the WAN form needs the width), then
// the data usage.
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_internet()} description={m.internet_description()} />
  <WiFi part="uplink" />
  <CardGrid cols={2}>
    <WANCard />
    <USBTetherCard />
    <FailoverCard />
  </CardGrid>
  <DataUsageCard />
</div>
