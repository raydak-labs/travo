<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Switch from '$lib/components/ui/switch/switch.svelte';
import { type DdnsRequest, type DdnsView, fetchDDNS, installDDNS, saveDDNS } from '../api/ddns';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import SelectField from '../ui/SelectField.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
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
const formId = $props.id();
</script>

<Section title={m.ddns_title()} description={m.ddns_intro()}>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data && !loader.data.installed}
      <div class="flex flex-wrap items-center gap-3">
        <Status kind="neutral">{m.ddns_state_absent()}</Status>
        <Button variant="outline" size="sm" type="button" disabled={busy} onclick={() => run(installDDNS, m.ddns_installed())}>
          {m.ddns_install()}
        </Button>
      </div>
      <p class="text-sm text-muted-foreground">{m.ddns_install_note()}</p>
    {:else if loader.data}
      {@const data = loader.data}
      <form id={formId} onsubmit={save}>
        <SettingsRow label={m.ddns_enabled()} for="{formId}-on"
          description={data.registered_ip ? m.ddns_registered({ ip: data.registered_ip }) : undefined}>
          {#snippet control()}<Switch id="{formId}-on" bind:checked={form.enabled} disabled={busy} />{/snippet}
        </SettingsRow>
        <SettingsRow label={m.ddns_service()}>
          {#snippet control()}
            <SelectField bind:value={form.service} disabled={busy} class="w-64"
              options={(data.services ?? []).map((s) => ({ value: s, label: s }))} />
          {/snippet}
        </SettingsRow>
        <SettingsRow label={m.ddns_domain()} for="{formId}-domain">
          {#snippet control()}<Input id="{formId}-domain" class="w-64" bind:value={form.domain} required maxlength={253} placeholder="trip.duckdns.org" autocomplete="off" spellcheck={false} disabled={busy} />{/snippet}
        </SettingsRow>
        <SettingsRow label={m.ddns_username()} for="{formId}-user">
          {#snippet control()}<Input id="{formId}-user" class="w-64" bind:value={form.username} maxlength={128} autocomplete="off" spellcheck={false} disabled={busy} />{/snippet}
        </SettingsRow>
        <SettingsRow label={m.ddns_password()} for="{formId}-pass">
          {#snippet control()}<Input id="{formId}-pass" class="w-64" type="password" bind:value={form.password} maxlength={128} autocomplete="new-password" required={!data.password_set} disabled={busy} placeholder={data.password_set ? m.wan_password_keep() : ''} />{/snippet}
        </SettingsRow>
      </form>
    {/if}
  </LoadState>
  {#snippet footer()}
    {#if loader.data?.installed}
      <Button variant="default" type="submit" form={formId} disabled={busy}>{m.ddns_save()}</Button>
    {/if}
  {/snippet}
</Section>
