<script lang="ts">
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const portForwards = lazyPage(() => import('./PortForwardsCard.svelte'));
</script>

<div class="grid gap-4">
  <PageHost load={portForwards} />
</div>
