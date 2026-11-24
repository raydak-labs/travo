<script lang="ts">
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const diagnostics = lazyPage(() => import('./DiagnosticsCard.svelte'));
const wakeOnLan = lazyPage(() => import('./WakeOnLanCard.svelte'));
</script>

<div class="grid gap-4">
  <PageHost load={diagnostics} />
  <PageHost load={wakeOnLan} />
</div>
