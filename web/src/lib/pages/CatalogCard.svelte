<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import {
  type CatalogAction,
  type CatalogItem,
  changeCatalog,
  fetchCatalog,
  luciUrl,
  sizeMiB,
} from '../api/catalog';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Status from '../ui/Status.svelte';

const loader = createLoader(fetchCatalog);
const confirm = createConfirm();
let pending = $state<string>();

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
  await loader.retry();
}
</script>

<Card title={m.catalog_title()}>
  <p class="text-muted-foreground">{m.catalog_intro()}</p>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data?.busy}
      <p class="mt-2"><Status kind="warn">{m.catalog_err_busy()}</Status></p>
    {/if}
    <ul class="mt-3 grid gap-3">
      {#each loader.data?.entries ?? [] as it (it.id)}
        <li class="rounded-md border p-3">
          <div class="flex flex-wrap items-center gap-2">
            <span class="font-semibold">{name(it)}</span>
            {#if !it.installed}
              <Status kind="neutral">{m.catalog_absent()}</Status>
            {:else}
              <Status kind={it.built_in ? 'info' : 'ok'}>{it.built_in ? m.catalog_built_in() : m.catalog_installed()}</Status>
              {#if it.autostart_supported}
                <Status kind={it.autostart ? 'ok' : 'neutral'}>{it.autostart ? m.catalog_autostart_on() : m.catalog_autostart_off()}</Status>
              {/if}
              {#if it.running_supported}
                <Status kind={it.running ? 'ok' : 'warn'}>{it.running ? m.catalog_running() : m.catalog_stopped()}</Status>
              {/if}
            {/if}
          </div>
          <p class="mt-1 text-sm text-muted-foreground">
            {about[it.id]?.()}
            {#if it.installed && it.version}{m.catalog_version({ version: it.version })}{/if}
          </p>
          <div class="mt-2 flex flex-wrap gap-2">
            {#if !it.installed}
              <Button variant="outline" size="sm" type="button" disabled={pending !== undefined}
                onclick={() => run(it, 'install')}>{m.catalog_install_size({ size: sizeMiB(it.size_bytes) })}</Button>
            {:else}
              {#if it.autostart_supported}
                <Button variant="outline" size="sm" type="button" disabled={pending !== undefined}
                  onclick={() => run(it, it.autostart ? 'off' : 'on')}>{it.autostart ? m.catalog_turn_off() : m.catalog_turn_on()}</Button>
              {/if}
              {#if it.luci_path}
                <Button variant="outline" size="sm" href={luciUrl(window.location, it.luci_path)} target="_blank"
                  rel="noopener">{m.catalog_luci()}</Button>
              {/if}
              {#if !it.built_in}
                <Button variant="outline" size="sm" type="button" disabled={pending !== undefined}
                  onclick={() => run(it, 'remove')}>{m.catalog_remove()}</Button>
              {/if}
            {/if}
          </div>
        </li>
      {/each}
    </ul>
  </LoadState>
</Card>
<ConfirmDialog {confirm} />
