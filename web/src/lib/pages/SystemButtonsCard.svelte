<script lang="ts">
import ToggleLeft from '@lucide/svelte/icons/toggle-left';
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
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';

const loader = createLoader(fetchButtons);
const confirm = createConfirm();
let busy = $state(false);
// Edited action per switch; a switch without an entry shows its saved action.

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
  notify.success(m.buttons_saved());
  void loader.retry();
}
// Device-tree labels are lowercase kernel names (`reset`, `switch`); show them as words.
const buttonName = (name: string) => name.charAt(0).toUpperCase() + name.slice(1);
</script>

<Section title={m.buttons_title()}>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      {#if loader.data.buttons.length === 0}
        <EmptyState icon={ToggleLeft} title={m.buttons_none()} />
      {:else}
        <div role="group" aria-label={m.buttons_title()}>
          {#each loader.data.buttons as button (button.name)}
            {@const last = button.last ? m.buttons_last({ time: runTime(button.last.at, undefined), position: positionLabel(button.last.position), outcome: outcomeLabel(button.last.outcome) }) : ''}
            <SettingsRow
              label={buttonName(button.label || button.name)}
              description={button.configurable ? [m.buttons_switch_hint(), last].filter(Boolean).join(' ') : m.buttons_stock_hint()}
            >
              {#snippet control()}
                {#if button.configurable}
                  <SelectField value={button.action} disabled={busy} class="w-48"
                    onchange={(v) => void save(button.name, v as ButtonAction)}
                    options={buttonActions.map((a) => ({ value: a, label: actionLabel(a) }))} />
                {:else}
                  <Status kind="neutral">{m.buttons_stock()}</Status>
                {/if}
              {/snippet}
            </SettingsRow>
          {/each}
        </div>
      {/if}
    {/if}
  </LoadState>
</Section>
<ConfirmDialog {confirm} />
