<script lang="ts">
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const lanAddress = lazyPage(() => import('./LANAddressCard.svelte'));
const interfaces = lazyPage(() => import('./NetworkInterfacesCard.svelte'));
</script>

<div class="grid gap-4">
  <PageHost load={lanAddress} />
  <PageHost load={interfaces} />
</div>
