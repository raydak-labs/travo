<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { fetchDeviceSettings, saveDeviceSettings } from '../api/system-settings';
import { m } from '../paraglide/messages.js';
import {
  detectTimezoneMismatch,
  dismissTimezoneMismatch,
  readBrowserTimezone,
  type TimezoneMismatch,
} from '../timezone-mismatch';
import { notify } from './notify';

let { generation }: { generation: number } = $props();

const storage = (() => {
  try {
    return sessionStorage;
  } catch {
    return undefined;
  }
})();

let mismatch = $state<TimezoneMismatch | null>(null);
let busy = $state(false);

$effect(() => {
  void generation;
  mismatch = null;
  const browserZone = readBrowserTimezone();
  if (!browserZone) return;
  const request = new AbortController();
  fetchDeviceSettings(request.signal)
    .then((data) => {
      if (request.signal.aborted) return;
      mismatch = detectTimezoneMismatch({
        routerZone: data.settings.zonename,
        zonenames: data.zonenames,
        browserZone,
        storage,
      });
    })
    .catch(() => {});
  return () => request.abort();
});

async function updateTimezone() {
  const current = mismatch;
  if (!current || busy) return;
  busy = true;
  const toastId = 'timezone-mismatch-update';
  notify.progress(m.timezone_mismatch_updating(), toastId);
  try {
    // Re-read before saving so hostname and NTP edits are not overwritten.
    const latest = await fetchDeviceSettings();
    const result = await saveDeviceSettings(latest.settings, {
      hostname: latest.settings.hostname,
      zonename: current.browserZone,
      ntpEnabled: latest.settings.ntp.enabled,
      servers: latest.settings.ntp.servers.join('\n'),
    });
    if ('error' in result) {
      notify.error(result.error, toastId);
      return;
    }
    mismatch = null;
    notify.success(m.timezone_mismatch_updated({ zone: current.browserZone }), toastId);
  } catch {
    notify.error(m.settings_err_failed(), toastId);
  } finally {
    busy = false;
  }
}

function dismiss() {
  if (!mismatch || busy) return;
  dismissTimezoneMismatch(storage, mismatch.routerZone, mismatch.browserZone);
  mismatch = null;
}
</script>

{#if mismatch}
  <section class="mb-4" aria-label={m.timezone_mismatch_region()}>
    <Alert class="status status-info flex flex-wrap items-center justify-between gap-2">
      <span class="min-w-0 flex-1">
        {m.timezone_mismatch_body({
          router: mismatch.routerZone || 'UTC',
          browser: mismatch.browserZone,
        })}
      </span>
      <div class="flex shrink-0 flex-wrap items-center gap-2">
        <Button variant="outline" size="sm" type="button" disabled={busy} onclick={updateTimezone}>
          {busy ? m.timezone_mismatch_updating() : m.timezone_mismatch_update()}
        </Button>
        <Button variant="ghost" size="sm" type="button" disabled={busy} onclick={dismiss}>
          {m.timezone_mismatch_dismiss()}
        </Button>
      </div>
    </Alert>
  </section>
{/if}
