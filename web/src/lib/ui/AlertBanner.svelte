<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import { alertMessage, alertRoute, bannerAlerts } from '../alerts';
import type { AlertList } from '../api/gen';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import Notice from './Notice.svelte';

let { list }: { list: AlertList | undefined } = $props();
const shown = $derived(bannerAlerts(list));
</script>

{#if shown.length > 0}
  <section class="mb-4 flex flex-col gap-2" aria-label={m.alerts_region()}>
    {#each shown as alert (alert.id)}
      <Notice tone={alert.severity === 'info' ? 'ok' : 'danger'}>
        {alertMessage(alert)}
        {#snippet actions()}
          <Button variant="outline" size="sm" href={href(alertRoute(alert))}>{m.alert_open()}</Button>
        {/snippet}
      </Notice>
    {/each}
  </section>
{/if}
