<script lang="ts">
import { m } from '../paraglide/messages.js';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each uplink card is its own chunk; the page shell stays small.
const usbTether = lazyPage(() => import('./USBTetherCard.svelte'));
const failover = lazyPage(() => import('./FailoverCard.svelte'));
const interfaces = lazyPage(() => import('./NetworkInterfacesCard.svelte'));
</script>

<div class="grid gap-4">
  <p class="text-muted-foreground">{m.network_intro()}</p>
  <PageHost load={usbTether} />
  <PageHost load={failover} />
  <PageHost load={interfaces} />
</div>
