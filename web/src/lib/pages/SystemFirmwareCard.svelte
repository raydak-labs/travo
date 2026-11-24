<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import {
  applyFirmware,
  boardMatches,
  FIRMWARE_PHRASE,
  fetchFirmware,
  phraseMatches,
  uploadFirmware,
} from '../api/firmware';
import type { FirmwareImage } from '../api/gen';
import { formatBytes } from '../format';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import Status from '../ui/Status.svelte';

const status = createLoader(fetchFirmware);
const confirm = createConfirm();
let files = $state<FileList>();
let fileInput = $state<HTMLInputElement | null>(null);
let uploaded = $state<FirmwareImage>();
let keepSettings = $state(true);
let typed = $state('');
let busy = $state(false);
let started = $state(false);
const image = $derived(uploaded ?? status.data?.staged);
const maxBytes = $derived(status.data?.max_image_bytes ?? 0);
const canKeep = $derived(image?.validation.allow_backup === true);
const keep = $derived(keepSettings && canKeep);

async function upload(event: SubmitEvent) {
  event.preventDefault();
  const file = files?.[0];
  if (busy || !file) return;
  // The daemon refuses an image that does not fit before reading it; say so up front.
  if (file.size > maxBytes) {
    notify.error(m.firmware_err_no_room());
    return;
  }
  busy = true;
  const result = await uploadFirmware(file);
  busy = false;
  if ('error' in result) {
    // The daemon dropped the previous image before taking this one: show what it holds now.
    uploaded = undefined;
    status.retry();
    notify.error(result.error);
    return;
  }
  uploaded = result.image;
  typed = '';
}

async function flash(event: SubmitEvent) {
  event.preventDefault();
  if (busy || !image || !image.validation.valid || !phraseMatches(typed)) return;
  const ok = await confirm.ask({
    title: m.firmware_confirm_title(),
    description: keep ? m.firmware_confirm_keep() : m.firmware_confirm_wipe(),
    action: m.firmware_action(),
    tone: 'danger',
  });
  if (!ok) return;
  busy = true;
  const result = await applyFirmware(image.sha256, keep, typed);
  busy = false;
  if ('error' in result) {
    notify.error(result.error);
    return;
  }
  started = true;
}
</script>

<Section title={m.firmware_title()}>
  {#if started || status.data?.pending}
    <Alert class="status status-danger" role="status" aria-live="polite">{m.firmware_started()}</Alert>
  {:else}
    <p class="text-muted-foreground">{m.firmware_intro()}</p>
    <LoadState status={status.status} retry={status.retry} stale={status.data !== undefined}>
      <form class="flex flex-col gap-3" onsubmit={upload}>
        <!-- The native picker shows the browser's language; the button and file name are ours. -->
        <div class="grid gap-1">
          <Label for="firmware-file">{m.firmware_file({ max: formatBytes(maxBytes) })}</Label>
          <input id="firmware-file" type="file" accept=".bin,.img,.itb" class="sr-only" bind:this={fileInput} bind:files disabled={busy} />
          <div class="flex flex-wrap items-center gap-2">
            <Button variant="outline" type="button" disabled={busy} onclick={() => fileInput?.click()}>{m.firmware_choose()}</Button>
            <span class="text-sm break-all text-muted-foreground">{files?.[0]?.name ?? m.firmware_no_file()}</span>
          </div>
        </div>
        <div>
          <Button variant="outline" type="submit" disabled={busy || !files?.length}>
            {busy && !image ? m.firmware_uploading() : m.firmware_upload()}
          </Button>
        </div>
      </form>
      {#if image}
        <dl class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2">
          <dt class="text-muted-foreground">{m.firmware_validation()}</dt>
          <dd>
            <Status kind={image.validation.valid ? 'ok' : 'danger'}>
              {image.validation.valid ? m.firmware_valid() : m.firmware_invalid()}
            </Status>
          </dd>
          <dt class="text-muted-foreground">{m.firmware_board()}</dt>
          <dd>
            <Status kind={boardMatches(image) ? 'ok' : 'danger'}>
              {boardMatches(image) ? m.firmware_board_match({ board: image.board }) : m.firmware_board_mismatch({ board: image.board })}
            </Status>
          </dd>
          <dt class="text-muted-foreground">{m.firmware_size()}</dt>
          <dd>{formatBytes(image.size)}</dd>
          <dt class="text-muted-foreground">{m.firmware_sha()}</dt>
          <dd class="break-all font-mono text-sm">{image.sha256}</dd>
        </dl>
        <Alert class="status status-warn" role="note">
          {#if !image.packages_known}
            {m.firmware_packages_unknown()}
          {:else if image.dropped_packages.length === 0}
            {m.firmware_packages_none()}
          {:else}
            {m.firmware_packages_dropped({ count: image.dropped_packages.length })}
            <ul class="mt-1 text-sm">
              {#each image.dropped_packages as name (name)}<li class="break-all">{name}</li>{/each}
            </ul>
          {/if}
        </Alert>
        {#if image.validation.valid}
          <form class="flex flex-col gap-3" onsubmit={flash}>
            <label class="flex items-center gap-2">
              <input
                type="checkbox"
                checked={keep}
                onchange={(e) => (keepSettings = e.currentTarget.checked)}
                disabled={busy || !canKeep}
              />
              {m.firmware_keep()}
            </label>
            {#if !canKeep}<Status kind="warn">{m.firmware_no_backup()}</Status>{/if}
            {#if !keep}<Status kind="danger">{m.firmware_wipe_warning()}</Status>{/if}
            <Label class="grid gap-1">
              {m.firmware_type_phrase({ phrase: FIRMWARE_PHRASE })}
              <Input bind:value={typed} autocomplete="off" spellcheck={false} disabled={busy} />
            </Label>
            <div>
              <Button variant="destructive" type="submit" disabled={busy || !phraseMatches(typed)}>{m.firmware_action()}</Button>
            </div>
          </form>
        {:else}
          <p class="text-muted-foreground">{m.firmware_invalid_hint()}</p>
        {/if}
      {/if}
    </LoadState>
  {/if}
</Section>

<ConfirmDialog {confirm} />
