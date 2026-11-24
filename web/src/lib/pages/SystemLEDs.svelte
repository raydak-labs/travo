<script lang="ts">
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const leds = lazyPage(() => import('./SystemLEDsCard.svelte'));
const buttons = lazyPage(() => import('./SystemButtonsCard.svelte'));
</script>

<div class="grid gap-4">
  <PageHost load={leds} />
  <PageHost load={buttons} />
</div>
