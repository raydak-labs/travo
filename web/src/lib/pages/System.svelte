<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import '../api/client';
import { downloadSystemBackup, fetchSystemInfo } from '../api/system';
import { type AuthMessage, passwordMessage } from '../auth-messages';
import { formatBytes, formatLoad, formatUptime } from '../format';
import { navigate } from '../navigate';
import { m } from '../paraglide/messages.js';
import { passwordRulesOk } from '../password-rules';
import { restart } from '../restart.svelte';
import { session } from '../session.svelte';
import { theme } from '../theme.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import FormMessage from '../ui/FormMessage.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import PageHeader from '../ui/PageHeader.svelte';
import PasswordField from '../ui/PasswordField.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

const settingsCard = lazyPage(() => import('./SystemSettingsCard.svelte'));
const sshKeysCard = lazyPage(() => import('./SystemSSHKeysCard.svelte'));

const info = createLoader(fetchSystemInfo);
let backupPending = $state(false);
const confirm = createConfirm();

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

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_system()} description={m.system_device_description()} />
  <Section title={m.system_device()}>
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
  </Section>

  <PageHost load={settingsCard} />

  <Section title={m.system_backup()}>
    <div>
      <Button variant="outline"
        type="button"

        disabled={backupPending}
        onclick={downloadBackup}
      >
        {backupPending ? m.system_backup_pending() : m.system_backup_download()}
      </Button>
    </div>
  </Section>

  <PageHost load={sshKeysCard} />

  <Section title={m.reboot_title()} description={m.reboot_confirm()}>
    <div>
      <Button variant="destructive" type="button" disabled={restart.state !== 'idle'} onclick={() => restart.ask(confirm)}>
        {m.reboot_router()}
      </Button>
    </div>
  </Section>

  <Section title={m.system_about()}>
    <SettingsRow label={m.system_theme()}>
      {#snippet control()}
        <Button variant="outline" size="sm" type="button" aria-pressed={theme.value === 'dark'} onclick={() => theme.toggle()}>
          {m.theme_toggle()}
        </Button>
      {/snippet}
    </SettingsRow>
  </Section>

  <Section title={m.password_change_title()}>
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
  </Section>

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
