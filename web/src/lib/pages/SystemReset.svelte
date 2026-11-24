<script lang="ts">
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const factoryReset = lazyPage(() => import('./SystemFactoryResetCard.svelte'));
</script>

<div class="grid gap-4">
  <PageHost load={factoryReset} />
</div>
