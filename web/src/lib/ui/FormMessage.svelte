<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import type { AuthMessage } from '../auth-messages';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import Status from './Status.svelte';

// Messages that belong to no single input: shown as an alert with the follow-up the code allows.
let { message, retry }: { message: AuthMessage; retry?: () => void } = $props();
</script>

<Alert class="flex flex-wrap items-center gap-3">
  <Status kind="danger">{message.text}</Status>
  {#if message.action === 'login'}
    <Button variant="link" href={href('login')}>{m.login_title()}</Button>
  {:else if message.action === 'setup'}
    <Button variant="link" href={href('setup')}>{m.setup_title()}</Button>
  {:else if message.action === 'retry' && retry}
    <Button variant="outline" size="sm"
      type="button"
      class="text-sm"
      onclick={retry}
    >
      {m.load_retry()}
    </Button>
  {/if}
</Alert>
