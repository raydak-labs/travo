<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import logo from '../assets/logo.webp';
import { type AuthMessage, setupMessage } from '../auth-messages';
import { m } from '../paraglide/messages.js';
import { passwordRulesOk } from '../password-rules';
import { session } from '../session.svelte';
import FormMessage from '../ui/FormMessage.svelte';
import PasswordField from '../ui/PasswordField.svelte';
import Section from '../ui/Section.svelte';

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

<div class="mx-auto w-full max-w-md pt-6 md:pt-12">
<div class="mb-6 flex flex-col items-center gap-2">
  <img src={logo} alt="" class="size-20" />
  <span class="text-xl font-semibold">{m.app_name()}</span>
</div>
<Section title={m.setup_title()} description={m.setup_body()}>
  <form class="flex flex-col gap-4" onsubmit={submit}>
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
</Section>
</div>
