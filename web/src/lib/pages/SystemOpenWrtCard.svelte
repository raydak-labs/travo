<script lang="ts">
import ExternalLink from '@lucide/svelte/icons/external-link';
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
import SettingsRow from '../ui/SettingsRow.svelte';

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

{#snippet fact(label: string, value: string)}
  <SettingsRow {label}>{#snippet control()}<span class="text-sm">{value}</span>{/snippet}</SettingsRow>
{/snippet}

<Section title={m.osupdate_title()} description={m.osupdate_intro()}>
  {#snippet actions()}
    <Button variant="ghost" size="sm" href={luci} target="_blank" rel="noopener noreferrer">{m.osupdate_open_luci()}<ExternalLink aria-hidden="true" /></Button>
  {/snippet}
  <LoadState status={last.status} retry={last.retry} stale={last.data !== undefined}>
    {#if result?.checked_at}
      <div class="border-t border-border">
        {@render fact(m.osupdate_running(), `${result.current} ${result.target}`)}
        {@render fact(m.osupdate_in_series(), result.in_series || m.osupdate_none())}
        {@render fact(m.osupdate_next_series(), result.next_series || m.osupdate_none())}
        <SettingsRow label={m.osupdate_packages()}>
          {#snippet control()}
            <span class="text-sm">
              {#if !result?.packages_read}
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
            </span>
          {/snippet}
        </SettingsRow>
        {@render fact(m.osupdate_checked(), new Date(result.checked_at).toLocaleString())}
      </div>
    {:else}
      <div class="border-t border-border">{@render fact(m.osupdate_checked(), m.osupdate_never_short())}</div>
    {/if}
  </LoadState>
  {#snippet footer()}
    <Button variant="outline" type="button" disabled={busy} onclick={() => check(true)}>{m.osupdate_check_refresh()}</Button>
    <Button variant="default" type="button" disabled={busy} onclick={() => check(false)}>{m.osupdate_check()}</Button>
  {/snippet}
</Section>
