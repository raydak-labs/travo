<script lang="ts">
import KeyRound from '@lucide/svelte/icons/key-round';
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import { type AuthMessage, passwordMessage } from '../auth-messages';
import { m } from '../paraglide/messages.js';
import { passwordRulesOk } from '../password-rules';
import { session } from '../session.svelte';
import FormMessage from '../ui/FormMessage.svelte';
import { notify } from '../ui/notify';
import PasswordField from '../ui/PasswordField.svelte';

// The admin password, changed in a dialog opened from the Access page header.
let { open = $bindable(false) }: { open?: boolean } = $props();
const formId = $props.id();
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
  open = false;
  notify.success(m.password_changed());
}
</script>

<Button variant="outline" size="sm" type="button" onclick={() => { open = true; }}><KeyRound aria-hidden="true" />{m.password_change_title()}</Button>

<Dialog.Root bind:open onOpenChange={(value) => { if (!value) { current = next = repeat = ''; failure = null; mismatch = false; } }}>
  <Dialog.Content portalProps={{ disabled: typeof window === 'undefined' }}>
    <Dialog.Header>
      <Dialog.Title level={2}>{m.password_change_title()}</Dialog.Title>
      <Dialog.Description>{m.password_change_intro()}</Dialog.Description>
    </Dialog.Header>
    <form id={formId} class="flex flex-col gap-4" onsubmit={submit}>
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
    </form>
    <Dialog.Footer>
      <Button variant="outline" type="button" onclick={() => { open = false; }}>{m.dialog_cancel()}</Button>
      <Button type="submit" form={formId} disabled={pending}>{m.password_submit()}</Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
