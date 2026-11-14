<script lang="ts">
import { onMount } from 'svelte';
import { postSystemReboot } from '../api/gen';
import '../api/client';
import { fetchHealth } from '../api/health';
import { rebootErrorKey, waitForBoot } from '../api/reboot';
import { downloadSystemBackup, fetchSystemInfo } from '../api/system';
import { type AuthMessage, passwordMessage } from '../auth-messages';
import { formatBytes, formatLoad, formatUptime } from '../format';
import { navigate } from '../navigate';
import { m } from '../paraglide/messages.js';
import { passwordRulesOk } from '../password-rules';
import { session } from '../session.svelte';
import { theme } from '../theme.svelte';
import Card from '../ui/Card.svelte';
import FormMessage from '../ui/FormMessage.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import PasswordField from '../ui/PasswordField.svelte';
import SystemUpdate from './SystemUpdate.svelte';

const info = createLoader(fetchSystemInfo);
let backupPending = $state(false);
let backupError = $state(false);

type RebootState =
  | 'idle'
  | 'confirming'
  | 'requesting'
  | 'restarting'
  | 'back'
  | 'timeout'
  | 'failed';
let rebootState = $state<RebootState>('idle');
let rebootFailure = $state<ReturnType<typeof rebootErrorKey>>('generic');
let rebootController: AbortController | undefined;

onMount(() => {
  const controller = new AbortController();
  rebootController = controller;
  return () => controller.abort();
});

async function restartRouter() {
  const controller = rebootController;
  if (!controller || rebootState === 'requesting') return;
  rebootState = 'requesting';
  try {
    const { data, error } = await postSystemReboot({
      body: { confirm: true },
      signal: controller.signal,
    });
    if (controller.signal.aborted) return;
    if (!data) {
      rebootFailure = rebootErrorKey((error as { code?: string } | undefined)?.code);
      rebootState = 'failed';
      return;
    }
    rebootState = 'restarting';
    const result = await waitForBoot(data.boot_id, {
      health: fetchHealth,
      sleep: (ms) => new Promise<void>((resolve) => setTimeout(resolve, ms)),
      now: Date.now,
      signal: controller.signal,
    });
    if (!controller.signal.aborted) rebootState = result === 'restarted' ? 'back' : 'timeout';
  } catch {
    if (!controller.signal.aborted) {
      rebootFailure = 'generic';
      rebootState = 'failed';
    }
  }
}

async function downloadBackup() {
  backupPending = true;
  backupError = false;
  try {
    await downloadSystemBackup();
  } catch {
    backupError = true;
  } finally {
    backupPending = false;
  }
}

let current = $state('');
let next = $state('');
let confirm = $state('');
let pending = $state(false);
let failure = $state<AuthMessage | null>(null);
let mismatch = $state(false);
let changed = $state(false);

async function submit(event?: SubmitEvent) {
  event?.preventDefault();
  failure = null;
  mismatch = false;
  changed = false;
  if (!passwordRulesOk(next)) {
    failure = { text: m.password_rules(), field: 'new' };
    return;
  }
  if (next !== confirm) {
    mismatch = true;
    return;
  }
  pending = true;
  const result = await session.changePassword(current, next);
  pending = false;
  if (!result.ok) {
    failure = passwordMessage(result.code);
    return;
  }
  current = next = confirm = '';
  changed = true;
}

async function logout() {
  // Leave the page first so the guard does not remember System as the place to return to.
  navigate('dashboard');
  await session.logout();
}
</script>

<div class="flex flex-col gap-4">
  <Card title={m.system_device()}>
    <LoadState status={info.status} retry={info.retry} stale={info.data !== undefined}>
      {#if info.data}
        {@const d = info.data}
        <dl class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2">
          <dt class="text-muted">{m.system_model()}</dt>
          <dd>{d.model}</dd>
          <dt class="text-muted">{m.system_hostname()}</dt>
          <dd>{d.hostname}</dd>
          <dt class="text-muted">{m.system_openwrt()}</dt>
          <dd>{d.openwrt} <span class="text-muted">{d.revision}</span></dd>
          <dt class="text-muted">{m.system_kernel()}</dt>
          <dd>{d.kernel}</dd>
          <dt class="text-muted">{m.system_uptime()}</dt>
          <dd>{formatUptime(d.uptime_seconds)}</dd>
          <dt class="text-muted">{m.system_load()}</dt>
          <dd>{formatLoad(d.load)}</dd>
          <dt class="text-muted">{m.system_memory()}</dt>
          <dd>
            {m.system_used_of({
              used: formatBytes(d.memory.total_bytes - d.memory.available_bytes),
              total: formatBytes(d.memory.total_bytes),
            })}
          </dd>
          <dt class="text-muted">{m.system_storage()}</dt>
          <dd>
            {m.system_used_of({
              used: formatBytes(d.storage.total_bytes - d.storage.available_bytes),
              total: formatBytes(d.storage.total_bytes),
            })}
          </dd>
          <dt class="text-muted">{m.system_app_version()}</dt>
          <dd>{d.app.version}</dd>
          <dt class="text-muted">{m.system_build_time()}</dt>
          <dd>{d.app.build_time || '—'}</dd>
        </dl>
      {/if}
    </LoadState>
  </Card>

  <SystemUpdate />

  <Card title={m.system_backup()}>
    <div>
      <button
        type="button"
        class="rounded-md border border-border px-3 py-2 hover:bg-bg disabled:opacity-60"
        disabled={backupPending}
        onclick={downloadBackup}
      >
        {backupPending ? m.system_backup_pending() : m.system_backup_download()}
      </button>
    </div>
    {#if backupError}
      <p role="alert" class="status status-danger self-start">{m.system_backup_failed()}</p>
    {/if}
  </Card>

  <Card title={m.reboot_title()}>
    {#if rebootState === 'confirming' || rebootState === 'requesting'}
      <p>{m.reboot_confirm()}</p>
      <div class="mt-3 flex flex-wrap gap-2">
        <button
          type="button"
          class="rounded-md border border-border px-3 py-2 hover:bg-bg disabled:opacity-60"
          disabled={rebootState === 'requesting'}
          onclick={restartRouter}
        >
          {m.reboot_title()}
        </button>
        <button
          type="button"
          class="rounded-md border border-border px-3 py-2 hover:bg-bg disabled:opacity-60"
          disabled={rebootState === 'requesting'}
          onclick={() => { rebootState = 'idle'; }}
        >
          {m.update_cancel()}
        </button>
      </div>
    {:else if rebootState === 'restarting'}
      <p aria-live="polite">{m.reboot_restarting()}</p>
    {:else if rebootState === 'back'}
      <p aria-live="polite">{m.reboot_back()}</p>
    {:else if rebootState === 'timeout'}
      <p role="alert" class="status status-danger self-start">{m.reboot_timeout()}</p>
    {:else}
      <div>
        <button
          type="button"
          class="rounded-md border border-border px-3 py-2 hover:bg-bg"
          onclick={() => { rebootState = 'confirming'; }}
        >
          {m.reboot_router()}
        </button>
      </div>
      {#if rebootState === 'failed'}
        <p role="alert" class="status status-danger mt-3 self-start">
          {m.reboot_error({ key: rebootFailure })}
        </p>
      {/if}
    {/if}
  </Card>

  <Card title={m.system_about()}>
    <div class="flex items-center justify-between gap-4">
      <span>{m.system_theme()}</span>
      <button
        type="button"
        class="rounded-md border border-border px-3 py-1 text-sm hover:bg-bg"
        aria-pressed={theme.value === 'dark'}
        onclick={() => theme.toggle()}
      >
        {m.theme_toggle()}
      </button>
    </div>
  </Card>

  <Card title={m.password_change_title()}>
    <form class="flex flex-col gap-4" onsubmit={submit}>
      <PasswordField
        id="password-current"
        label={m.password_current()}
        autocomplete="current-password"
        bind:value={current}
        error={failure?.field === 'current' ? failure.text : null}
      />
      <PasswordField
        id="password-new"
        label={m.password_new()}
        autocomplete="new-password"
        bind:value={next}
        error={failure?.field === 'new' ? failure.text : null}
      />
      <PasswordField
        id="password-confirm"
        label={m.setup_confirm()}
        autocomplete="new-password"
        bind:value={confirm}
        error={mismatch ? m.password_mismatch() : null}
      />
      {#if failure && failure.field === null}
        <FormMessage message={failure} retry={() => submit()} />
      {/if}
      {#if changed}
        <p role="status" class="status status-ok self-start">{m.password_changed()}</p>
      {/if}
      <div>
        <button
          type="submit"
          disabled={pending}
          class="rounded-md bg-accent px-4 py-2 text-accent-contrast disabled:opacity-60"
        >
          {m.password_submit()}
        </button>
      </div>
    </form>
  </Card>

  <div>
    <button
      type="button"
      class="rounded-md border border-border px-3 py-2 hover:bg-bg"
      onclick={logout}
    >
      {m.logout()}
    </button>
  </div>
</div>
