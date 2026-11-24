<script lang="ts">
import ToggleLeft from '@lucide/svelte/icons/toggle-left';
import Button from '$lib/components/ui/button/button.svelte';
import {
  actionLabel,
  type ButtonAction,
  buttonActions,
  confirmText,
  fetchButtons,
  outcomeLabel,
  positionLabel,
  saveButtonAction,
} from '../api/buttons';
import { runTime } from '../api/wifi-schedule';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import EmptyState from '../ui/EmptyState.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import SelectField from '../ui/SelectField.svelte';
import Status from '../ui/Status.svelte';

const loader = createLoader(fetchButtons);
const confirm = createConfirm();
let busy = $state(false);
// Edited action per switch; a switch without an entry shows its saved action.
let picked = $state<Record<string, ButtonAction>>({});

async function save(name: string, action: ButtonAction) {
  if (busy) return;
  const description = confirmText(action);
  if (description) {
    const ok = await confirm.ask({
      title: m.buttons_confirm_title(),
      description,
      action: m.buttons_confirm_action(),
      tone: 'danger',
    });
    if (!ok) return;
  }
  busy = true;
  const result = await saveButtonAction(name, action);
  busy = false;
  if ('error' in result) {
    notify.error(result.error);
    return;
  }
  delete picked[name];
  notify.success(m.buttons_saved());
  void loader.retry();
}
</script>

<Section title={m.buttons_title()}>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      {#if loader.data.buttons.length === 0}
        <EmptyState icon={ToggleLeft} title={m.buttons_none()} />
      {:else}
        <ul class="divide-y divide-border" aria-label={m.buttons_title()}>
          {#each loader.data.buttons as button (button.name)}
            <li class="flex flex-col gap-2 py-2">
              <div class="flex items-center gap-2">
                <span class="min-w-0 flex-1 break-all font-mono text-sm">{button.label || button.name}</span>
                {#if button.configurable}
                  <Status kind={button.action === 'none' ? 'neutral' : 'ok'}>{actionLabel(button.action)}</Status>
                {:else}
                  <Status kind="neutral">{m.buttons_stock()}</Status>
                {/if}
              </div>
              {#if button.configurable}
                {@const choice = picked[button.name] ?? button.action}
                <div class="flex flex-wrap items-center gap-2">
                  <label class="flex items-center gap-2 text-sm">
                    {m.buttons_action()}
                    <SelectField value={choice} disabled={busy} class="w-48"
                      onchange={(v) => (picked[button.name] = v as ButtonAction)}
                      options={buttonActions.map((a) => ({ value: a, label: actionLabel(a) }))} />
                  </label>
                  <Button variant="outline" size="sm" type="button" disabled={busy || choice === button.action}
                    onclick={() => void save(button.name, choice)}>{m.buttons_save()}</Button>
                </div>
                <span class="text-sm text-muted-foreground">{m.buttons_switch_hint()}</span>
                {#if button.last}
                  <span class="text-xs text-muted-foreground">
                    {m.buttons_last({ time: runTime(button.last.at, undefined), position: positionLabel(button.last.position), outcome: outcomeLabel(button.last.outcome) })}
                  </span>
                {/if}
              {:else}
                <span class="text-sm text-muted-foreground">{m.buttons_stock_hint()}</span>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    {/if}
  </LoadState>
</Section>
<ConfirmDialog {confirm} />
