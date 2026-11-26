<script lang="ts">
import Wifi from '@lucide/svelte/icons/wifi';
import { onMount } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { m } from '../paraglide/messages.js';
import { href, wantsConnect } from '../router';
import CardGrid from '../ui/CardGrid.svelte';
import PageHeader from '../ui/PageHeader.svelte';
import DataUsageCard from './DataUsageCard.svelte';
import FailoverCard from './FailoverCard.svelte';
import USBTetherCard from './USBTetherCard.svelte';
import WANCard from './WANCard.svelte';
import WiFi from './WiFi.svelte';

// Internet: every way the router gets online. WiFi uplinks first (the travel case), then the
// Ethernet WAN, USB tethering and failover two per row (the WAN form needs the width), then
// the data usage. Connect to WiFi is a dialog; `#/internet?connect` opens it on arrival.
let connectOpen = $state(false);

onMount(() => {
  if (!wantsConnect(location.hash)) return;
  connectOpen = true;
  history.replaceState(history.state, '', href('internet'));
});
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_internet()} description={m.internet_description()}>
    {#snippet actions()}
      <Button variant="outline" size="sm" type="button" onclick={() => (connectOpen = true)}>
        <Wifi aria-hidden="true" />{m.wifi_connect()}
      </Button>
    {/snippet}
  </PageHeader>
  <WiFi part="uplink" bind:connectOpen />
  <CardGrid cols={2}>
    <WANCard />
    <USBTetherCard />
    <FailoverCard />
  </CardGrid>
  <DataUsageCard />
</div>
