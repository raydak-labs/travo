<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import { type AuthMessage, setupMessage } from '../auth-messages';
import { m } from '../paraglide/messages.js';
import { passwordRulesOk } from '../password-rules';
import { session } from '../session.svelte';
import Card from '../ui/Card.svelte';
import FormMessage from '../ui/FormMessage.svelte';
import PasswordField from '../ui/PasswordField.svelte';

let password = $state('');
let confirm = $state('');
let pending = $state(false);
let failure = $state<AuthMessage | null>(null);
let mismatch = $state(false);

async function submit(event?: SubmitEvent) {
  event?.preventDefault();
  failure = null;
  mismatch = false;
  if (!passwordRulesOk(password)) {
    failure = { text: m.password_rules(), field: 'password' };
    return;
  }
  if (password !== confirm) {
    mismatch = true;
    return;
  }
  pending = true;
  const result = await session.setup(password);
  pending = false;
  // Success and already_set change the session state; App's guard effect navigates.
  if (!result.ok && result.code !== 'already_set') failure = setupMessage(result.code);
}
</script>

<Card title={m.setup_title()}>
  <form class="flex flex-col gap-4" onsubmit={submit}>
    <p class="text-muted">{m.setup_body()}</p>
    <PasswordField
      id="setup-password"
      label={m.setup_password()}
      autocomplete="new-password"
      bind:value={password}
      error={failure?.field === 'password' ? failure.text : null}
    />
    <PasswordField
      id="setup-confirm"
      label={m.setup_confirm()}
      autocomplete="new-password"
      bind:value={confirm}
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
        {m.setup_submit()}
      </Button>
    </div>
  </form>
</Card>
