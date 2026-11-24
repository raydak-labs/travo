<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import { type AuthMessage, loginMessage } from '../auth-messages';
import { m } from '../paraglide/messages.js';
import { session } from '../session.svelte';
import FormMessage from '../ui/FormMessage.svelte';
import PasswordField from '../ui/PasswordField.svelte';
import Section from '../ui/Section.svelte';

let password = $state('');
let pending = $state(false);
let failure = $state<AuthMessage | null>(null);

async function submit(event?: SubmitEvent) {
  event?.preventDefault();
  failure = null;
  pending = true;
  const result = await session.login(password);
  pending = false;
  // Success and setup_required change the session state; App's guard effect navigates.
  if (!result.ok && result.code !== 'setup_required') failure = loginMessage(result.code);
}
</script>

<div class="mx-auto w-full max-w-md pt-6 md:pt-12">
<Section title={m.login_title()}>
  <form class="flex flex-col gap-4" onsubmit={submit}>
    <PasswordField
      id="login-password"
      label={m.login_password()}
      autocomplete="current-password"
      bind:value={password}
      error={failure?.field === 'password' ? failure.text : null}
    />
    {#if failure && failure.field === null}
      <FormMessage message={failure} retry={() => submit()} />
    {/if}
    <div>
      <Button variant="default"
        type="submit"
        disabled={pending}

      >
        {m.login_submit()}
      </Button>
    </div>
  </form>
</Section>
</div>
