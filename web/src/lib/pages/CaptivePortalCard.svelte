<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import type { Status as Connectivity } from '../api/gen';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import Status from '../ui/Status.svelte';

// Shown only when an action exists (invariants §12); the caller decides from the same data.
let { info }: { info: Connectivity } = $props();
</script>

<Card title={m.captive_title()}>
  <div class="flex flex-col items-start gap-3">
    <Status kind="warn">{m.dashboard_internet_captive()}</Status>
    <p>{m.captive_login_needed()}</p>
    {#if info.portal_url}
      <Button size="sm" href={info.portal_url} target="_blank" rel="noopener noreferrer">
        {m.captive_open_login()}
      </Button>
    {/if}
  </div>
</Card>
