<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Badge from '$lib/components/ui/badge/badge.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import { addDNSName, type DnsName, fetchDNSNames, removeDNSName } from '../api/dhcp';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';

const names = createLoader(fetchDNSNames);
const confirm = createConfirm();
let name = $state('');
let ip = $state('');
let busy = $state(false);
let invalid = $state(false);

function label(entry: DnsName): string {
  return entry.names.join(' ');
}

async function add(event: SubmitEvent) {
  event.preventDefault();
  if (busy) return;
  invalid = name.trim() === '' || ip.trim() === '';
  if (invalid) return;
  busy = true;
  const result = await addDNSName(name, ip);
  busy = false;
  if ('error' in result) {
    notify.error(result.error);
    return;
  }
  name = '';
  ip = '';
  notify.success(m.dns_names_added({ name: label(result.entry), ip: result.entry.ip }));
  void names.retry();
}

async function remove(entry: DnsName) {
  const ok = await confirm.ask({
    title: m.dns_names_remove_title({ name: label(entry) }),
    description: m.dns_names_remove_body(),
    action: m.dns_names_remove(),
    tone: 'danger',
  });
  if (!ok) return;
  busy = true;
  const result = await removeDNSName(entry.id);
  busy = false;
  if ('error' in result) notify.error(result.error);
  else notify.success(m.dns_names_removed({ name: label(entry) }));
  void names.retry();
}
</script>

<Card title={m.dns_names_title()}>
  <p class="text-muted-foreground">{m.dns_names_intro()}</p>
  <LoadState status={names.status} retry={names.retry} stale={names.data !== undefined}>
    {#if names.data}
      {#if names.data.names.length === 0}
        <p class="mt-3 text-muted-foreground">{m.dns_names_empty()}</p>
      {:else}
        <ul class="mt-3 divide-y divide-border" aria-label={m.dns_names_title()}>
          {#each names.data.names as entry (entry.id)}
            <li class="flex flex-wrap items-center gap-2 py-2">
              <span class="min-w-0 flex-1">
                <span class="block break-all font-medium">{label(entry)}</span>
                <span class="block break-all font-mono text-xs text-muted-foreground">{entry.ip}</span>
              </span>
              {#if entry.owned}
                <Button variant="outline" size="sm" type="button" disabled={busy}
                  aria-label={m.dns_names_remove_name({ name: label(entry) })}
                  onclick={() => remove(entry)}>{m.dns_names_remove()}</Button>
              {:else}
                <Badge variant="outline">{m.dns_names_luci()}</Badge>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    {/if}
  </LoadState>
  <form class="mt-4 flex flex-wrap items-end gap-3" onsubmit={add}>
    <Label class="grid min-w-40 flex-1 gap-1">
      {m.dns_names_name()}
      <Input bind:value={name} maxlength={253} autocomplete="off" autocapitalize="off" spellcheck={false}
        disabled={busy} placeholder="nas" aria-invalid={invalid} />
    </Label>
    <Label class="grid min-w-40 flex-1 gap-1">
      {m.dns_names_ip()}
      <Input bind:value={ip} maxlength={45} autocomplete="off" autocapitalize="off" spellcheck={false}
        disabled={busy} placeholder="192.168.1.20" class="font-mono" aria-invalid={invalid} />
    </Label>
    <Button variant="outline" type="submit" disabled={busy}>{m.dns_names_add()}</Button>
  </form>
  {#if invalid}
    <Alert class="status status-danger mt-2 self-start" role="alert">{m.dns_names_err_input()}</Alert>
  {/if}
</Card>

<ConfirmDialog {confirm} />
