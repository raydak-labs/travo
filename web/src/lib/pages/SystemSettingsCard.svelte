<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import Textarea from '$lib/components/ui/textarea/textarea.svelte';
import { fetchDeviceSettings, saveDeviceSettings, validHostname } from '../api/system-settings';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';

const loader = createLoader(fetchDeviceSettings);
let hostname = $state('');
let zonename = $state('');
let ntpEnabled = $state(true);
let servers = $state('');
let filled = false;
let busy = $state(false);
let failure = $state<string>();
let saved = $state(false);

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
  saved = false;
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
    failure = result.error;
    return;
  }
  filled = false;
  saved = true;
  void loader.retry();
}
</script>

<Card title={m.settings_title()}>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      <form class="flex flex-col gap-4" onsubmit={save}>
        <Label class="grid gap-1">
          {m.settings_hostname()}
          <Input bind:value={hostname} required maxlength={63} autocomplete="off" spellcheck={false} disabled={busy} />
        </Label>
        <Label class="grid gap-1">
          {m.settings_zone()}
          <Input bind:value={zonename} list="travo-zonenames" required autocomplete="off" spellcheck={false} disabled={busy} />
          <datalist id="travo-zonenames">
            {#each loader.data.zonenames as zone (zone)}<option value={zone}></option>{/each}
          </datalist>
          <span class="text-sm text-muted-foreground">{m.settings_zone_hint()}</span>
        </Label>
        <label class="flex items-center gap-2">
          <input type="checkbox" bind:checked={ntpEnabled} disabled={busy} />
          {m.settings_ntp()}
        </label>
        <Label class="grid gap-1">
          {m.settings_ntp_servers()}
          <Textarea bind:value={servers} rows={4} spellcheck={false} disabled={busy} class="font-mono text-sm" />
          <span class="text-sm text-muted-foreground">{m.settings_ntp_servers_hint()}</span>
        </Label>
        {#if failure}
          <Alert class="status status-danger self-start" role="alert">{failure}</Alert>
        {/if}
        {#if saved}
          <Status kind="ok">{m.settings_saved()}</Status>
        {/if}
        <div>
          <Button variant="default" type="submit" disabled={busy}>{m.settings_save()}</Button>
        </div>
      </form>
    {/if}
  </LoadState>
</Card>
