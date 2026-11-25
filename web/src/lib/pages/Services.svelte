<script lang="ts">
import EllipsisVertical from '@lucide/svelte/icons/ellipsis-vertical';
import ExternalLink from '@lucide/svelte/icons/external-link';
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
import { fetchAdGuard } from '../api/adguard';
import {
  type CatalogAction,
  type CatalogItem,
  changeCatalog,
  fetchCatalog,
  luciUrl,
  sizeMiB,
} from '../api/catalog';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import PageHeader from '../ui/PageHeader.svelte';
import ServiceTile from '../ui/ServiceTile.svelte';
import Status from '../ui/Status.svelte';

// Every optional service at a glance: installed ones first (state and one action), then the
// ones that can be installed. Services with a page here open it; the others open LuCI.
const adguard = createLoader(fetchAdGuard, 10000);
const catalog = createLoader(fetchCatalog);
const confirm = createConfirm();
let pending = $state<string>();
onDestroy(() => {
  adguard.stop();
  catalog.stop();
});

const names: Record<string, () => string> = {
  sqm: m.catalog_sqm,
  cloudflared: m.catalog_cloudflared,
  vnstat: m.catalog_vnstat,
  watchcat: m.catalog_watchcat,
  speedtest: m.catalog_speedtest,
};
const about: Record<string, () => string> = {
  sqm: m.catalog_sqm_about,
  cloudflared: m.catalog_cloudflared_about,
  vnstat: m.catalog_vnstat_about,
  watchcat: m.catalog_watchcat_about,
  speedtest: m.catalog_speedtest_about,
};
const name = (it: CatalogItem) => names[it.id]?.() ?? it.id;
const installed = $derived((catalog.data?.entries ?? []).filter((it) => it.installed));
const available = $derived((catalog.data?.entries ?? []).filter((it) => !it.installed));

function question(it: CatalogItem, action: CatalogAction) {
  const n = name(it);
  switch (action) {
    case 'install':
      return {
        title: m.catalog_install_title({ name: n }),
        description: m.catalog_install_confirm({
          size: sizeMiB(it.size_bytes),
          packages: (it.packages ?? []).join(', '),
        }),
        action: m.catalog_install(),
      };
    case 'remove':
      return {
        title: m.catalog_remove_title({ name: n }),
        description: m.catalog_remove_confirm(),
        action: m.catalog_remove(),
        tone: 'danger' as const,
      };
    case 'off':
      return {
        title: m.catalog_off_title({ name: n }),
        description: m.catalog_off_confirm(),
        action: m.catalog_turn_off(),
        tone: 'danger' as const,
      };
    default:
      return undefined;
  }
}

async function run(it: CatalogItem, action: CatalogAction) {
  if (pending) return;
  const q = question(it, action);
  if (q && !(await confirm.ask(q))) return;
  pending = it.id;
  notify.progress(
    action === 'install' ? m.catalog_installing({ name: name(it) }) : m.catalog_applying(),
    'catalog',
  );
  const result = await changeCatalog(it.id, action);
  pending = undefined;
  if ('error' in result) notify.error(result.error, 'catalog');
  else notify.success(m.catalog_done({ name: name(it) }), 'catalog');
  await catalog.retry();
}
</script>

{#snippet catalogStatus(it: CatalogItem)}
  {#if it.running_supported}
    <Status kind={it.running ? 'ok' : 'neutral'}>{it.running ? m.catalog_running() : m.catalog_stopped()}</Status>
  {:else if it.autostart_supported}
    <Status kind={it.autostart ? 'ok' : 'neutral'}>{it.autostart ? m.catalog_autostart_on() : m.catalog_autostart_off()}</Status>
  {:else}
    <Status kind="ok">{it.built_in ? m.catalog_built_in() : m.catalog_installed()}</Status>
  {/if}
{/snippet}

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_services()} description={m.services_description()} />

  <section class="flex flex-col gap-3" aria-labelledby="services-installed">
    <h2 id="services-installed" class="text-base font-semibold">{m.services_installed()}</h2>
    <ul class="grid gap-4 sm:grid-cols-2">
      <ServiceTile title={m.adguard_title()} description={m.adguard_about()}>
        {#snippet status()}
          <LoadState status={adguard.status} retry={adguard.retry} stale={adguard.data !== undefined}>
            {#if adguard.data}
              {#if !adguard.data.installed}
                <Status kind="neutral">{m.adguard_state_absent()}</Status>
              {:else if adguard.data.running}
                <Status kind="ok">{m.adguard_state_running()}</Status>
              {:else}
                <Status kind="warn">{m.adguard_state_stopped()}</Status>
              {/if}
            {/if}
          </LoadState>
        {/snippet}
        {#snippet actions()}
          <Button variant="outline" size="sm" href={href('services-adguard')}>{adguard.data && !adguard.data.installed ? m.adguard_install() : m.services_configure()}</Button>
        {/snippet}
      </ServiceTile>
      {#each installed as it (it.id)}
        <ServiceTile title={name(it)} description={about[it.id]?.()}>
          {#snippet status()}{@render catalogStatus(it)}{/snippet}
          {#snippet actions()}
            {#if it.luci_path}
              <Button variant="outline" size="sm" href={luciUrl(window.location, it.luci_path)} target="_blank" rel="noopener">{m.catalog_luci()}<ExternalLink aria-hidden="true" /></Button>
            {/if}
            {#if it.autostart_supported || !it.built_in}
              <DropdownMenu.Root>
                <DropdownMenu.Trigger>
                  {#snippet child({ props })}
                    <Button {...props} variant="ghost" size="icon" class="ml-auto" aria-label={m.table_row_menu({ name: name(it) })} disabled={pending !== undefined}>
                      <EllipsisVertical aria-hidden="true" size={16} />
                    </Button>
                  {/snippet}
                </DropdownMenu.Trigger>
                <DropdownMenu.Content align="end">
                  {#if it.autostart_supported}
                    <DropdownMenu.Item onSelect={() => run(it, it.autostart ? 'off' : 'on')}>{it.autostart ? m.catalog_turn_off() : m.catalog_turn_on()}</DropdownMenu.Item>
                  {/if}
                  {#if !it.built_in}
                    <DropdownMenu.Separator />
                    <DropdownMenu.Item variant="destructive" onSelect={() => run(it, 'remove')}>{m.catalog_remove()}</DropdownMenu.Item>
                  {/if}
                </DropdownMenu.Content>
              </DropdownMenu.Root>
            {/if}
          {/snippet}
        </ServiceTile>
      {/each}
    </ul>
  </section>

  <section class="flex flex-col gap-3" aria-labelledby="services-available">
    <div>
      <h2 id="services-available" class="text-base font-semibold">{m.services_available()}</h2>
      <p class="text-sm text-muted-foreground">{m.catalog_intro()}</p>
    </div>
    <LoadState status={catalog.status} retry={catalog.retry} stale={catalog.data !== undefined}>
      {#if catalog.data?.busy}<Status kind="warn">{m.catalog_err_busy()}</Status>{/if}
      {#if catalog.data && available.length === 0}
        <p class="text-sm text-muted-foreground">{m.services_all_installed()}</p>
      {/if}
      <ul class="grid gap-4 sm:grid-cols-2">
        {#each available as it (it.id)}
          <ServiceTile title={name(it)} description={about[it.id]?.()}>
            {#snippet actions()}
              <Button variant="outline" size="sm" type="button" disabled={pending !== undefined} onclick={() => run(it, 'install')}>{m.catalog_install_size({ size: sizeMiB(it.size_bytes) })}</Button>
            {/snippet}
          </ServiceTile>
        {/each}
      </ul>
    </LoadState>
  </section>
</div>

<ConfirmDialog {confirm} />
