<script lang="ts">
import { onMount } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
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
import { href } from '../router';
import { session } from '../session.svelte';
import { theme } from '../theme.svelte';
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import FormMessage from '../ui/FormMessage.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import PasswordField from '../ui/PasswordField.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';
import SystemUpdate from './SystemUpdate.svelte';

const settingsCard = lazyPage(() => import('./SystemSettingsCard.svelte'));
const sshKeysCard = lazyPage(() => import('./SystemSSHKeysCard.svelte'));
const openWrtCard = lazyPage(() => import('./SystemOpenWrtCard.svelte'));
const ledsCard = lazyPage(() => import('./SystemLEDsCard.svelte'));
const factoryResetCard = lazyPage(() => import('./SystemFactoryResetCard.svelte'));

const info = createLoader(fetchSystemInfo);
let backupPending = $state(false);
const confirm = createConfirm();

// Progress and outcome are toasts (id 'reboot'); the card only blocks a second request.
type RebootState = 'idle' | 'requesting' | 'restarting' | 'done';
let rebootState = $state<RebootState>('idle');
let rebootController: AbortController | undefined;

onMount(() => {
  const controller = new AbortController();
  rebootController = controller;
  return () => controller.abort();
});

async function askRestart() {
  const ok = await confirm.ask({
    title: m.reboot_confirm_title(),
    description: m.reboot_confirm(),
    action: m.reboot_router(),
    tone: 'danger',
  });
  if (ok) void restartRouter();
}

function rebootFailed(key: ReturnType<typeof rebootErrorKey>) {
  rebootState = 'idle';
  notify.error(m.reboot_error({ key }), 'reboot');
}

async function restartRouter() {
  const controller = rebootController;
  if (!controller || rebootState !== 'idle') return;
  rebootState = 'requesting';
  try {
    const { data, error } = await postSystemReboot({
      body: { confirm: true },
      signal: controller.signal,
    });
    if (controller.signal.aborted) return;
    if (!data) {
      rebootFailed(rebootErrorKey((error as { code?: string } | undefined)?.code));
      return;
    }
    rebootState = 'restarting';
    notify.progress(m.reboot_restarting(), 'reboot');
    const result = await waitForBoot(data.boot_id, {
      health: fetchHealth,
      sleep: (ms) => new Promise<void>((resolve) => setTimeout(resolve, ms)),
      now: Date.now,
      signal: controller.signal,
    });
    if (controller.signal.aborted) return;
    rebootState = 'done';
    if (result === 'restarted') notify.success(m.reboot_back(), 'reboot');
    else notify.error(m.reboot_timeout(), 'reboot');
  } catch {
    if (!controller.signal.aborted) rebootFailed('generic');
  }
}

async function downloadBackup() {
  backupPending = true;
  try {
    await downloadSystemBackup();
  } catch {
    notify.error(m.system_backup_failed());
  } finally {
    backupPending = false;
  }
}

let current = $state('');
let next = $state('');
let repeat = $state('');
let pending = $state(false);
let failure = $state<AuthMessage | null>(null);
let mismatch = $state(false);

async function submit(event?: SubmitEvent) {
  event?.preventDefault();
  failure = null;
  mismatch = false;
  if (!passwordRulesOk(next)) {
    failure = { text: m.password_rules(), field: 'new' };
    return;
  }
  if (next !== repeat) {
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
  current = next = repeat = '';
  notify.success(m.password_changed());
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
          <dt class="text-muted-foreground">{m.system_model()}</dt>
          <dd>{d.model}</dd>
          <dt class="text-muted-foreground">{m.system_hostname()}</dt>
          <dd>{d.hostname}</dd>
          <dt class="text-muted-foreground">{m.system_openwrt()}</dt>
          <dd>{d.openwrt} <span class="text-muted-foreground">{d.revision}</span></dd>
          <dt class="text-muted-foreground">{m.system_kernel()}</dt>
          <dd>{d.kernel}</dd>
          <dt class="text-muted-foreground">{m.system_uptime()}</dt>
          <dd>{formatUptime(d.uptime_seconds)}</dd>
          <dt class="text-muted-foreground">{m.system_load()}</dt>
          <dd>{formatLoad(d.load)}</dd>
          <dt class="text-muted-foreground">{m.system_memory()}</dt>
          <dd>
            {m.system_used_of({
              used: formatBytes(d.memory.total_bytes - d.memory.available_bytes),
              total: formatBytes(d.memory.total_bytes),
            })}
          </dd>
          <dt class="text-muted-foreground">{m.system_storage()}</dt>
          <dd>
            {m.system_used_of({
              used: formatBytes(d.storage.total_bytes - d.storage.available_bytes),
              total: formatBytes(d.storage.total_bytes),
            })}
          </dd>
          <dt class="text-muted-foreground">{m.system_app_version()}</dt>
          <dd>{d.app.version}</dd>
          <dt class="text-muted-foreground">{m.system_build_time()}</dt>
          <dd>{d.app.build_time || '—'}</dd>
        </dl>
      {/if}
    </LoadState>
  </Card>

  <PageHost load={settingsCard} />

  <PageHost load={ledsCard} />

  <SystemUpdate />

  <PageHost load={openWrtCard} />

  <Card title={m.system_backup()}>
    <div>
      <Button variant="outline"
        type="button"

        disabled={backupPending}
        onclick={downloadBackup}
      >
        {backupPending ? m.system_backup_pending() : m.system_backup_download()}
      </Button>
    </div>
  </Card>

  <Card title={m.tokens_title()}>
    <p class="text-muted-foreground">{m.tokens_intro()}</p>
    <div class="mt-3">
      <Button variant="outline" href={href('system-tokens')}>{m.tokens_open()}</Button>
    </div>
  </Card>

  <PageHost load={sshKeysCard} />

  <Card title={m.logs_title()}>
    <p class="text-muted-foreground">{m.logs_intro()}</p>
    <div class="mt-3">
      <Button variant="outline" href={href('system-logs')}>{m.logs_open()}</Button>
    </div>
  </Card>

  <Card title={m.reboot_title()}>
    <p class="text-muted-foreground">{m.reboot_confirm()}</p>
    <div>
      <Button variant="outline" type="button" disabled={rebootState !== 'idle'} onclick={askRestart}>
        {m.reboot_router()}
      </Button>
    </div>
  </Card>

  <Card title={m.system_about()}>
    <div class="flex items-center justify-between gap-4">
      <span>{m.system_theme()}</span>
      <Button variant="outline" size="sm"
        type="button"
        class="text-sm"
        aria-pressed={theme.value === 'dark'}
        onclick={() => theme.toggle()}
      >
        {m.theme_toggle()}
      </Button>
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
        bind:value={repeat}
        error={mismatch ? m.password_mismatch() : null}
      />
      {#if failure && failure.field === null}
        <FormMessage message={failure} retry={() => submit()} />
      {/if}
      <div>
        <Button variant="default"
          type="submit"
          disabled={pending}
        >
          {m.password_submit()}
        </Button>
      </div>
    </form>
  </Card>

  <PageHost load={factoryResetCard} />

  <div>
    <Button variant="outline"
      type="button"

      onclick={logout}
    >
      {m.logout()}
    </Button>
  </div>
</div>

<ConfirmDialog {confirm} />
