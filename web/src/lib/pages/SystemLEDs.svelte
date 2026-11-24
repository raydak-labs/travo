<script lang="ts">
import { m } from '../paraglide/messages.js';
import PageHeader from '../ui/PageHeader.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const leds = lazyPage(() => import('./SystemLEDsCard.svelte'));
const buttons = lazyPage(() => import('./SystemButtonsCard.svelte'));
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_system_leds()} description={m.system_leds_description()} />
  <PageHost load={leds} />
  <PageHost load={buttons} />
</div>
