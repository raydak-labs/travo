<script lang="ts">
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const ipv6 = lazyPage(() => import('./IPv6Card.svelte'));
const ddns = lazyPage(() => import('./DDNSCard.svelte'));
</script>

<div class="grid gap-4">
  <PageHost load={ipv6} />
  <PageHost load={ddns} />
</div>
