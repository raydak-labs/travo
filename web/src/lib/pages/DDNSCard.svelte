<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import { type DdnsRequest, type DdnsView, fetchDDNS, installDDNS, saveDDNS } from '../api/ddns';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import Status from '../ui/Status.svelte';

const loader = createLoader(fetchDDNS);
let form = $state<DdnsRequest>({
  enabled: false,
  service: '',
  domain: '',
  username: '',
  password: '',
});
let filled = false;
let busy = $state(false);

// Fill the form once per load; a later reload must not wipe edits. The token stays empty.
$effect(() => {
  const data = loader.data;
  if (!data?.installed || filled) return;
  filled = true;
  form = {
    enabled: data.enabled,
    service: data.service || (data.services ?? [])[0] || '',
    domain: data.domain,
    username: data.username,
    password: '',
  };
});

async function run(action: () => Promise<{ view: DdnsView } | { error: string }>, done: string) {
  if (busy) return;
  busy = true;
  notify.progress(m.ddns_applying(), 'ddns');
  const result = await action();
  busy = false;
  if ('error' in result) {
    notify.error(result.error, 'ddns');
    return;
  }
  notify.success(done, 'ddns');
  filled = false;
  void loader.retry();
}

function save(event: SubmitEvent) {
  event.preventDefault();
  void run(() => saveDDNS(form), m.ddns_saved());
}
</script>

<Section title={m.ddns_title()} description={m.ddns_intro()}>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data && !loader.data.installed}
      <div class="flex flex-wrap items-center gap-3">
        <Status kind="neutral">{m.ddns_state_absent()}</Status>
        <Button variant="outline" type="button" disabled={busy} onclick={() => run(installDDNS, m.ddns_installed())}>
          {m.ddns_install()}
        </Button>
      </div>
      <p class="text-sm text-muted-foreground">{m.ddns_install_note()}</p>
    {:else if loader.data}
      {@const data = loader.data}
      <div class="flex flex-wrap items-center gap-2">
        <Status kind={data.enabled ? 'ok' : 'neutral'}>{data.enabled ? m.ddns_state_on() : m.ddns_state_off()}</Status>
        {#if data.registered_ip}
          <span class="text-sm text-muted-foreground">{m.ddns_registered({ ip: data.registered_ip })}</span>
        {/if}
      </div>
      <form class="flex flex-col gap-4" onsubmit={save}>
        <label class="flex items-center gap-2">
          <input type="checkbox" bind:checked={form.enabled} disabled={busy} />
          {m.ddns_enabled()}
        </label>
        <div class="flex flex-wrap gap-3">
          <Label class="grid w-56 gap-1">
            {m.ddns_service()}
            <select bind:value={form.service} disabled={busy} required
              class="h-9 rounded-md border border-input bg-transparent px-2 text-sm">
              {#each data.services ?? [] as s (s)}
                <option value={s}>{s}</option>
              {/each}
            </select>
          </Label>
          <Label class="grid w-64 gap-1">
            {m.ddns_domain()}
            <Input bind:value={form.domain} required maxlength={253} placeholder="trip.duckdns.org"
              autocomplete="off" spellcheck={false} disabled={busy} />
          </Label>
          <Label class="grid w-48 gap-1">
            {m.ddns_username()}
            <Input bind:value={form.username} maxlength={128} autocomplete="off" spellcheck={false} disabled={busy} />
          </Label>
          <Label class="grid w-56 gap-1">
            {m.ddns_password()}
            <Input type="password" bind:value={form.password} maxlength={128} autocomplete="new-password"
              required={!data.password_set} disabled={busy}
              placeholder={data.password_set ? m.wan_password_keep() : ''} />
          </Label>
        </div>
        <div>
          <Button variant="default" type="submit" disabled={busy}>{m.ddns_save()}</Button>
        </div>
      </form>
    {/if}
  </LoadState>
</Section>
