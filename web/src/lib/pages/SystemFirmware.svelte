<script lang="ts">
import { m } from '../paraglide/messages.js';
import CardGrid from '../ui/CardGrid.svelte';
import PageHeader from '../ui/PageHeader.svelte';
import SystemFactoryResetCard from './SystemFactoryResetCard.svelte';
import SystemFirmwareCard from './SystemFirmwareCard.svelte';
import SystemOpenWrtCard from './SystemOpenWrtCard.svelte';
import SystemUpdate from './SystemUpdate.svelte';

// Firmware: Travo updates, OpenWrt updates, flashing an image and, last, the factory reset.
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_system_firmware()} description={m.system_firmware_description()} />
  <CardGrid cols={2}>
    <SystemUpdate />
    <SystemOpenWrtCard />
    <SystemFirmwareCard />
    <SystemFactoryResetCard />
  </CardGrid>
</div>
