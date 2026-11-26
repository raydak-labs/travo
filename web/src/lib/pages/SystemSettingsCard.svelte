<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Switch from '$lib/components/ui/switch/switch.svelte';
import Textarea from '$lib/components/ui/textarea/textarea.svelte';
import { fetchDeviceSettings, saveDeviceSettings, validHostname } from '../api/system-settings';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';

const loader = createLoader(fetchDeviceSettings);
let hostname = $state('');
let zonename = $state('');
let ntpEnabled = $state(true);
let servers = $state('');
let filled = false;
let busy = $state(false);
let failure = $state<string>();

// Fill the form once from the first successful load; a later reload must not wipe edits.
$effect(() => {
  const data = loader.data;
  if (!data || filled) return;
  filled = true;
  hostname = data.settings.hostname;
  zonename = data.settings.zonename;
  ntpEnabled = data.settings.ntp.enabled;
  servers = data.settings.ntp.servers.join('\n');
});

async function save(event: SubmitEvent) {
  event.preventDefault();
  const current = loader.data?.settings;
  if (!current || busy) return;
  if (!validHostname(hostname.trim())) {
    failure = m.settings_hostname_invalid();
    return;
  }
  if (!loader.data?.zonenames.includes(zonename) && zonename !== current.zonename) {
    failure = m.settings_zone_invalid();
    return;
  }
  busy = true;
  failure = undefined;
  const result = await saveDeviceSettings(current, { hostname, zonename, ntpEnabled, servers });
  busy = false;
  if ('error' in result) {
    notify.error(result.error);
    return;
  }
  filled = false;
  notify.success(m.settings_saved());
  void loader.retry();
}
const formId = $props.id();
</script>

<Section title={m.settings_title()}>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      <form id={formId} onsubmit={save}>
        <SettingsRow label={m.settings_hostname()} for="{formId}-hostname">
          {#snippet control()}
            <Input id="{formId}-hostname" class="w-64" bind:value={hostname} required maxlength={63} autocomplete="off" spellcheck={false} disabled={busy} />
          {/snippet}
        </SettingsRow>
        <SettingsRow label={m.settings_zone()} for="{formId}-zone" description={m.settings_zone_hint()}>
          {#snippet control()}
            <Input id="{formId}-zone" class="w-64" bind:value={zonename} list="travo-zonenames" required autocomplete="off" spellcheck={false} disabled={busy} />
            <datalist id="travo-zonenames">
              {#each loader.data?.zonenames ?? [] as zone (zone)}<option value={zone}></option>{/each}
            </datalist>
          {/snippet}
        </SettingsRow>
        <SettingsRow label={m.settings_ntp()} for="{formId}-ntp">
          {#snippet control()}
            <Switch id="{formId}-ntp" bind:checked={ntpEnabled} disabled={busy} />
          {/snippet}
        </SettingsRow>
        {#if ntpEnabled}
          <SettingsRow top label={m.settings_ntp_servers()} for="{formId}-servers" description={m.settings_ntp_servers_hint()}>
            {#snippet control()}
              <Textarea id="{formId}-servers" bind:value={servers} rows={3} spellcheck={false} disabled={busy} class="w-64 font-mono text-sm" />
            {/snippet}
          </SettingsRow>
        {/if}
        {#if failure}
          <p class="text-sm text-danger" role="alert">{failure}</p>
        {/if}
      </form>
    {/if}
  </LoadState>
  {#snippet footer()}
    <Button variant="default" type="submit" form={formId} disabled={busy || !loader.data}>{m.settings_save()}</Button>
  {/snippet}
</Section>
