<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import type { OpenWrtCheck } from '../api/gen';
import {
  attendedSysupgradeURL,
  fetchOpenWrtUpdates,
  runOpenWrtCheck,
} from '../api/openwrt-updates';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';

const last = createLoader(fetchOpenWrtUpdates);
let fresh = $state<OpenWrtCheck>();
let busy = $state(false);
const result = $derived(fresh ?? last.data);
// The link names this host: LuCI listens next to Travo (server rendering has no location).
const luci = typeof window === 'undefined' ? '' : attendedSysupgradeURL(window.location);

async function check(refreshLists: boolean) {
  if (busy) return;
  busy = true;
  const outcome = await runOpenWrtCheck(refreshLists);
  busy = false;
  if ('error' in outcome) {
    notify.error(outcome.error);
    return;
  }
  fresh = outcome.result;
  const r = outcome.result;
  notify.success(r.in_series || r.next_series ? m.osupdate_found() : m.osupdate_current());
}
</script>

<Section title={m.osupdate_title()} description={m.osupdate_intro()}>
  <LoadState status={last.status} retry={last.retry} stale={last.data !== undefined}>
    {#if result?.checked_at}
      <dl class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2">
        <dt class="text-muted-foreground">{m.osupdate_running()}</dt>
        <dd>{result.current} <span class="text-muted-foreground">{result.target}</span></dd>
        <dt class="text-muted-foreground">{m.osupdate_in_series()}</dt>
        <dd>{result.in_series || m.osupdate_none()}</dd>
        <dt class="text-muted-foreground">{m.osupdate_next_series()}</dt>
        <dd>{result.next_series || m.osupdate_none()}</dd>
        <dt class="text-muted-foreground">{m.osupdate_packages()}</dt>
        <dd>
          {#if !result.packages_read}
            {m.osupdate_packages_unread()}
          {:else if result.packages.length === 0}
            {result.lists_refreshed ? m.osupdate_packages_none() : m.osupdate_packages_none_stale()}
          {:else}
            <details>
              <summary class="cursor-pointer">{m.osupdate_packages_count({ count: result.packages.length })}</summary>
              <ul class="text-sm">
                {#each result.packages as p (p.name)}
                  <li class="break-all"><span class="font-medium">{p.name}</span> <span class="text-muted-foreground">{p.installed} → {p.available}</span></li>
                {/each}
              </ul>
            </details>
          {/if}
        </dd>
        <dt class="text-muted-foreground">{m.osupdate_checked()}</dt>
        <dd>{new Date(result.checked_at).toLocaleString()}</dd>
      </dl>
    {:else}
      <p class="text-muted-foreground">{m.osupdate_never()}</p>
    {/if}
  </LoadState>
  <p class="text-sm text-muted-foreground">{m.osupdate_luci_hint()}</p>
  <div class="flex flex-wrap gap-2">
    <Button variant="default" type="button" disabled={busy} onclick={() => check(false)}>{m.osupdate_check()}</Button>
    <Button variant="outline" type="button" disabled={busy} onclick={() => check(true)}>{m.osupdate_check_refresh()}</Button>
    <Button variant="outline" href={luci} target="_blank" rel="noopener noreferrer">{m.osupdate_open_luci()}</Button>
  </div>
</Section>
