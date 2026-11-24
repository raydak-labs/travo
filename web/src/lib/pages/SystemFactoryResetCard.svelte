<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import { phraseMatches, RESET_PHRASE, requestFactoryReset } from '../api/factory-reset';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';

const confirm = createConfirm();
let typed = $state('');
let busy = $state(false);
let started = $state(false);

async function reset(event: SubmitEvent) {
  event.preventDefault();
  if (busy || !phraseMatches(typed)) return;
  const ok = await confirm.ask({
    title: m.reset_confirm_title(),
    description: m.reset_confirm_body(),
    action: m.reset_action(),
    tone: 'danger',
  });
  if (!ok) return;
  busy = true;
  const result = await requestFactoryReset(typed);
  busy = false;
  if ('error' in result) {
    notify.error(result.error);
    return;
  }
  typed = '';
  started = true;
}
</script>

<Section title={m.reset_section()} description={started ? undefined : m.reset_intro()}>
  {#if started}
    <Alert class="status status-danger" role="status" aria-live="polite">{m.reset_started()}</Alert>
  {:else}
    <form class="flex flex-col gap-3" onsubmit={reset}>
      <Label class="grid gap-1">
        {m.reset_type_phrase({ phrase: RESET_PHRASE })}
        <Input bind:value={typed} autocomplete="off" spellcheck={false} disabled={busy} />
      </Label>
      <div>
        <Button variant="destructive" type="submit" disabled={busy || !phraseMatches(typed)}>{m.reset_action()}</Button>
      </div>
    </form>
  {/if}
</Section>

<ConfirmDialog {confirm} />
