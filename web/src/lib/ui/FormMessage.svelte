<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import type { AuthMessage } from '../auth-messages';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import Notice from './Notice.svelte';

// Messages that belong to no single input: shown as an alert with the follow-up the code allows.
let { message, retry }: { message: AuthMessage; retry?: () => void } = $props();
</script>

{#snippet followUp()}
  {#if message.action === 'login'}
    <Button variant="link" href={href('login')}>{m.login_title()}</Button>
  {:else if message.action === 'setup'}
    <Button variant="link" href={href('setup')}>{m.setup_title()}</Button>
  {:else if message.action === 'retry' && retry}
    <Button variant="outline" size="sm" type="button" onclick={retry}>{m.load_retry()}</Button>
  {/if}
{/snippet}

<Notice tone="danger" actions={message.action && (message.action !== 'retry' || retry) ? followUp : undefined}>
  {message.text}
</Notice>
