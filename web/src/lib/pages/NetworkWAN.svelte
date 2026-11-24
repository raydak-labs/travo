<script lang="ts">
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// Each card is its own chunk; the page shell stays small.
const wan = lazyPage(() => import('./WANCard.svelte'));
</script>

<div class="grid gap-4">
  <PageHost load={wan} />
</div>
