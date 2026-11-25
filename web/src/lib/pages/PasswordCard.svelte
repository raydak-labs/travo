<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import { type AuthMessage, passwordMessage } from '../auth-messages';
import { m } from '../paraglide/messages.js';
import { passwordRulesOk } from '../password-rules';
import { session } from '../session.svelte';
import FormMessage from '../ui/FormMessage.svelte';
import { notify } from '../ui/notify';
import PasswordField from '../ui/PasswordField.svelte';
import Section from '../ui/Section.svelte';

// The admin password; a credential form keeps its fields stacked at a fixed width.
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
</script>

<Section title={m.password_change_title()}>
  <form class="flex max-w-sm flex-col gap-4" onsubmit={submit}>
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

