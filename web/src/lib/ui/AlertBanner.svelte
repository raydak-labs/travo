<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { alertMessage, alertRoute, bannerAlerts } from '../alerts';
import type { AlertList } from '../api/gen';
import { m } from '../paraglide/messages.js';
import { href } from '../router';

let { list }: { list: AlertList | undefined } = $props();
const shown = $derived(bannerAlerts(list));
</script>

{#if shown.length > 0}
  <section class="mb-4 flex flex-col gap-2" aria-label={m.alerts_region()}>
    {#each shown as alert (alert.id)}
      <Alert class="status {alert.severity === 'info' ? 'status-ok' : 'status-danger'} flex flex-wrap items-center justify-between gap-2">
        <span class="min-w-0 flex-1">{alertMessage(alert)}</span>
        <Button variant="outline" size="sm" href={href(alertRoute(alert))}>{m.alert_open()}</Button>
      </Alert>
    {/each}
  </section>
{/if}
