<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
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
import { href } from '../router';
import Card from '../ui/Card.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';
import TokenSecretDialog from './TokenSecretDialog.svelte';

const tokens = createLoader(fetchTokens);
let name = $state('');
let scope = $state<TokenScope>('read');
let busy = $state(false);
let failure = $state<string>();
let created = $state<ApiTokenCreated>();
let revoking = $state<Token>();

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
    failure = result.error;
    return;
  }
  name = '';
  scope = 'read';
  created = result.created;
  void tokens.retry();
}

async function revoke(token: Token) {
  revoking = undefined;
  busy = true;
  failure = undefined;
  const result = await revokeToken(token.id);
  busy = false;
  if ('error' in result) failure = result.error;
  void tokens.retry();
}
</script>

<div class="flex flex-col gap-4">
  <div>
    <Button variant="link" class="px-0" href={href('system')}>{m.tokens_back()}</Button>
  </div>

  <Card title={m.tokens_title()}>
    <p class="text-muted-foreground">{m.tokens_intro()}</p>
    <LoadState status={tokens.status} retry={tokens.retry} stale={tokens.data !== undefined}>
      {#if tokens.data?.length === 0}
        <p class="mt-3 text-muted-foreground">{m.tokens_empty()}</p>
      {:else if tokens.data}
        <ul class="mt-3 divide-y divide-border" aria-label={m.tokens_title()}>
          {#each tokens.data as token (token.id)}
            <li class="flex flex-wrap items-center gap-2 py-2">
              <span class="break-all font-medium">{token.name}</span>
              <Status kind={token.scope === 'admin' ? 'warn' : 'neutral'}>{scopeLabel(token.scope)}</Status>
              <span class="text-sm text-muted-foreground">
                {m.tokens_created_at({ date: createdLabel(token.created_at) })}
              </span>
              <Button variant="outline" size="sm" class="ml-auto" type="button" disabled={busy}
                aria-label={m.tokens_revoke_name({ name: token.name })}
                onclick={() => { revoking = token; }}>{m.tokens_revoke()}</Button>
            </li>
          {/each}
        </ul>
      {/if}
    </LoadState>
  </Card>

  <Card title={m.tokens_new()}>
    <form class="flex flex-col gap-4" onsubmit={create}>
      <Label class="grid gap-1">
        {m.tokens_name()}
        <Input bind:value={name} required maxlength={256} autocomplete="off" disabled={busy}
          placeholder={m.tokens_name_hint()} />
      </Label>
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
        <Alert class="status status-danger self-start" role="alert">{failure}</Alert>
      {/if}
      <div>
        <Button variant="default" type="submit" disabled={busy}>
          {busy ? m.tokens_creating() : m.tokens_create()}
        </Button>
      </div>
    </form>
  </Card>
</div>

<TokenSecretDialog {created} close={() => { created = undefined; }} />

<Dialog.Root open={revoking !== undefined} onOpenChange={(open) => { if (!open) revoking = undefined; }}>
  <Dialog.Content portalProps={{ disabled: typeof window === 'undefined' }} showCloseButton={false}
    class="w-[min(90vw,26rem)] rounded-lg border border-border bg-surface p-5 text-fg">
    <Dialog.Title level={2} class="text-lg font-semibold">
      {m.tokens_revoke_title({ name: revoking?.name ?? '' })}
    </Dialog.Title>
    <Dialog.Description>{m.tokens_revoke_body()}</Dialog.Description>
    <div class="flex justify-end gap-2">
      <Button variant="outline" type="button" onclick={() => { revoking = undefined; }}>{m.tokens_cancel()}</Button>
      <Button variant="destructive" type="button" disabled={busy}
        onclick={() => revoking && revoke(revoking)}>{m.tokens_revoke()}</Button>
    </div>
  </Dialog.Content>
</Dialog.Root>
