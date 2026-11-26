<script lang="ts">
import KeyRound from '@lucide/svelte/icons/key-round';
import Plus from '@lucide/svelte/icons/plus';
import Trash2 from '@lucide/svelte/icons/trash-2';
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import * as Table from '$lib/components/ui/table';
import type { ApiTokenCreated, Token } from '../api/gen';
import {
  createToken,
  fetchTokens,
  revokeToken,
  type TokenScope,
  tokenError,
  tokenName,
} from '../api/tokens';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import EmptyState from '../ui/EmptyState.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import Status from '../ui/Status.svelte';
import PasswordDialog from './PasswordDialog.svelte';
import SystemSSHKeysCard from './SystemSSHKeysCard.svelte';
import TokenSecretDialog from './TokenSecretDialog.svelte';

const tokens = createLoader(fetchTokens);
let name = $state('');
let scope = $state<TokenScope>('read');
let busy = $state(false);
let failure = $state<string>();
let created = $state<ApiTokenCreated>();
const confirm = createConfirm();
const formId = $props.id();
let creating = $state(false);

function openCreate() {
  name = '';
  scope = 'read';
  failure = undefined;
  creating = true;
}

const scopes: { value: TokenScope; label: () => string; hint: () => string }[] = [
  { value: 'read', label: () => m.tokens_scope_read(), hint: () => m.tokens_scope_read_hint() },
  { value: 'admin', label: () => m.tokens_scope_admin(), hint: () => m.tokens_scope_admin_hint() },
];

function scopeLabel(value: TokenScope): string {
  return value === 'admin' ? m.tokens_scope_admin() : m.tokens_scope_read();
}

function createdLabel(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime())
    ? iso
    : date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

async function create(event: SubmitEvent) {
  event.preventDefault();
  if (busy) return;
  const valid = tokenName(name);
  if (!valid) {
    failure = tokenError('invalid_request');
    return;
  }
  busy = true;
  failure = undefined;
  const result = await createToken(valid, scope);
  busy = false;
  if ('error' in result) {
    notify.error(result.error);
    return;
  }
  name = '';
  scope = 'read';
  creating = false;
  created = result.created;
  void tokens.retry();
}

async function revoke(token: Token) {
  const ok = await confirm.ask({
    title: m.tokens_revoke_title({ name: token.name }),
    description: m.tokens_revoke_body(),
    action: m.tokens_revoke(),
    tone: 'danger',
  });
  if (!ok) return;
  busy = true;
  const result = await revokeToken(token.id);
  busy = false;
  if ('error' in result) notify.error(result.error);
  else notify.success(m.tokens_revoked({ name: token.name }));
  void tokens.retry();
}
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_system_access()} description={m.system_access_description()}>
    {#snippet actions()}<PasswordDialog />{/snippet}
  </PageHeader>

  <Section title={m.tokens_yours()} description={m.tokens_intro()}>
    {#snippet actions()}
      <Button variant="outline" size="sm" type="button" disabled={busy} onclick={openCreate}><Plus aria-hidden="true" />{m.tokens_create()}</Button>
    {/snippet}
    <LoadState status={tokens.status} retry={tokens.retry} stale={tokens.data !== undefined}>
      {#if tokens.data?.length === 0}
        <EmptyState icon={KeyRound} title={m.tokens_empty()} />
      {:else if tokens.data}
        <Table.Root aria-label={m.tokens_title()}>
          <Table.Header>
            <Table.Row>
              <Table.Head>{m.tokens_name()}</Table.Head>
              <Table.Head>{m.tokens_scope_label()}</Table.Head>
              <Table.Head class="max-sm:hidden">{m.tokens_created()}</Table.Head>
              <Table.Head class="w-12"><span class="sr-only">{m.table_actions()}</span></Table.Head>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {#each tokens.data as token (token.id)}
              <Table.Row>
                <Table.Cell class="font-medium break-all whitespace-normal">{token.name}</Table.Cell>
                <Table.Cell><Status kind={token.scope === 'admin' ? 'warn' : 'neutral'}>{scopeLabel(token.scope)}</Status></Table.Cell>
                <Table.Cell class="text-muted-foreground max-sm:hidden">{createdLabel(token.created_at)}</Table.Cell>
                <Table.Cell class="text-right">
                  <Button variant="ghost" size="icon" type="button" disabled={busy}
                    aria-label={m.tokens_revoke_name({ name: token.name })}
                    onclick={() => revoke(token)}><Trash2 aria-hidden="true" size={16} /></Button>
                </Table.Cell>
              </Table.Row>
            {/each}
          </Table.Body>
        </Table.Root>
      {/if}
    </LoadState>
  </Section>

  <SystemSSHKeysCard />
</div>

<Dialog.Root bind:open={creating}>
  <Dialog.Content portalProps={{ disabled: typeof window === 'undefined' }}>
    <Dialog.Header>
      <Dialog.Title level={2}>{m.tokens_new()}</Dialog.Title>
      <Dialog.Description>{m.tokens_intro()}</Dialog.Description>
    </Dialog.Header>
    <form id={formId} class="flex flex-col gap-4" onsubmit={create}>
      <div class="grid gap-2">
        <Label for="{formId}-name">{m.tokens_name()}</Label>
        <Input id="{formId}-name" bind:value={name} required maxlength={256} autocomplete="off" disabled={busy}
          placeholder={m.tokens_name_hint()} />
      </div>
      <fieldset class="grid gap-2" disabled={busy}>
        <legend class="mb-1 text-sm font-medium">{m.tokens_scope_label()}</legend>
        {#each scopes as option (option.value)}
          <label class="flex items-start gap-2">
            <input type="radio" name="token-scope" class="mt-1" value={option.value} bind:group={scope} />
            <span>
              <span class="font-medium">{option.label()}</span>
              <span class="block text-sm text-muted-foreground">{option.hint()}</span>
            </span>
          </label>
        {/each}
      </fieldset>
      {#if failure}
        <p class="text-sm text-danger" role="alert">{failure}</p>
      {/if}
    </form>
    <Dialog.Footer>
      <Button variant="outline" type="button" onclick={() => { creating = false; }}>{m.dialog_cancel()}</Button>
      <Button type="submit" form={formId} disabled={busy}>{busy ? m.tokens_creating() : m.tokens_create()}</Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<TokenSecretDialog {created} close={() => { created = undefined; }} />

<ConfirmDialog {confirm} />
