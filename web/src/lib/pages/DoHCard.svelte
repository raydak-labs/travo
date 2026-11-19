<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import { dohPresets, fetchDoH, saveDoH, validDoH } from '../api/doh';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Status from '../ui/Status.svelte';

const loader = createLoader(fetchDoH);
let url = $state('');
let busy = $state(false);
let failure = $state<string>();

async function save(event: SubmitEvent) {
  event.preventDefault();
  if (busy) return;
  const next = url.trim();
  if (!validDoH(next)) {
    failure = m.doh_err_invalid();
    return;
  }
  failure = undefined;
  busy = true;
  // AdGuard tests the server first (up to its 10 s upstream timeout).
  notify.progress(m.doh_testing(), 'doh');
  const result = await saveDoH(next);
  busy = false;
  if ('error' in result) {
    notify.error(result.error, 'doh');
    return;
  }
  notify.success(m.doh_saved(), 'doh');
  url = '';
  void loader.retry();
}
</script>

<Card title={m.doh_title()}>
  <p class="text-muted-foreground">{m.doh_intro()}</p>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      {@const ups = loader.data.upstreams ?? []}
      <div class="mt-3 flex flex-wrap items-center gap-2">
        <Status kind={loader.data.encrypted ? 'ok' : 'warn'}>
          {loader.data.encrypted ? m.doh_state_encrypted() : m.doh_state_plain()}
        </Status>
        <span class="break-all text-sm text-muted-foreground">{ups.join(', ')}</span>
      </div>
      <form class="mt-3 flex flex-col gap-3" onsubmit={save}>
        <div class="flex flex-wrap gap-2">
          {#each dohPresets as [name, preset] (preset)}
            <Button variant="outline" size="sm" type="button" disabled={busy} onclick={() => (url = preset)}>{name}</Button>
          {/each}
        </div>
        <Label class="grid max-w-xl gap-1">
          {m.doh_url()}
          <Input bind:value={url} required maxlength={512} placeholder="https://dns10.quad9.net/dns-query"
            autocomplete="off" spellcheck={false} disabled={busy} />
        </Label>
        {#if failure}<p class="text-sm text-danger" role="alert">{failure}</p>{/if}
        <div>
          <Button variant="default" type="submit" disabled={busy}>{m.doh_save()}</Button>
        </div>
      </form>
    {/if}
  </LoadState>
</Card>
