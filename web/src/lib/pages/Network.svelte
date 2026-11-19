<script lang="ts">
import { m } from '../paraglide/messages.js';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each uplink card is its own chunk; the page shell stays small.
const wan = lazyPage(() => import('./WANCard.svelte'));
const usbTether = lazyPage(() => import('./USBTetherCard.svelte'));
const failover = lazyPage(() => import('./FailoverCard.svelte'));
const interfaces = lazyPage(() => import('./NetworkInterfacesCard.svelte'));
const diagnostics = lazyPage(() => import('./DiagnosticsCard.svelte'));
const wakeOnLan = lazyPage(() => import('./WakeOnLanCard.svelte'));
const dhcpServer = lazyPage(() => import('./DHCPServerCard.svelte'));
const dnsNames = lazyPage(() => import('./DNSNamesCard.svelte'));
const portForwards = lazyPage(() => import('./PortForwardsCard.svelte'));
const lanAddress = lazyPage(() => import('./LANAddressCard.svelte'));
const ipv6 = lazyPage(() => import('./IPv6Card.svelte'));
const ddns = lazyPage(() => import('./DDNSCard.svelte'));
</script>

<div class="grid gap-4">
  <p class="text-muted-foreground">{m.network_intro()}</p>
  <PageHost load={wan} />
  <PageHost load={usbTether} />
  <PageHost load={failover} />
  <PageHost load={ipv6} />
  <PageHost load={ddns} />
  <PageHost load={interfaces} />
  <PageHost load={lanAddress} />
  <PageHost load={dhcpServer} />
  <PageHost load={dnsNames} />
  <PageHost load={portForwards} />
  <PageHost load={diagnostics} />
  <PageHost load={wakeOnLan} />
</div>
