<script lang="ts">
import RefreshCw from '@lucide/svelte/icons/refresh-cw';
import { onMount } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import {
  getSystemUpdateApply,
  getSystemUpdateStage,
  postSystemUpdateApply,
  postSystemUpdateCheck,
  postSystemUpdateRecover,
  postSystemUpdateStage,
} from '../api/gen';
import '../api/client';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import Section from '../ui/Section.svelte';
import Status from '../ui/Status.svelte';
import { disabledMessage, showOperation } from '../update-check';

type Update = NonNullable<Awaited<ReturnType<typeof postSystemUpdateCheck>>['data']>;
let result = $state<Update>();
type Stage = NonNullable<Awaited<ReturnType<typeof getSystemUpdateStage>>['data']>;
let stage = $state<Stage>();
let verifying = $state(false);
let verifyFailed = $state(false);
let stageRequest = 0;
type Operation = NonNullable<Awaited<ReturnType<typeof getSystemUpdateApply>>['data']>;
let operation = $state<Operation>();
const confirm = createConfirm();
let updating = $state(false);
let applyFailed = $state(false);
let operationRequest = 0;
let readingOperation = false;
// The daemon keeps the last operation's journal: an old "0.1.0 installed" stays there after a
// later install.sh. Show a finished result only when this page started it or it matches what runs.
let startedHere = $state(false);
const shownOperation = $derived(
  showOperation(operation, result?.running_version, startedHere) ? operation : undefined,
);

async function readOperation(request: AbortController) {
  if (readingOperation) return;
  readingOperation = true;
  const generation = ++operationRequest;
  try {
    const response = await getSystemUpdateApply({ signal: request.signal });
    if (!request.signal.aborted && generation === operationRequest && response.data) {
      operation = response.data;
    }
  } catch {
    // The listener can be briefly absent during procd restart.
  } finally {
    readingOperation = false;
  }
}

async function changeOperation(request: AbortController, version?: string) {
  updating = true;
  applyFailed = false;
  startedHere = true;
  operationRequest++;
  try {
    const options = { signal: request.signal };
    const response = version
      ? await postSystemUpdateApply({ ...options, body: { version } })
      : await postSystemUpdateRecover(options);
    if (!request.signal.aborted) {
      if (response.data) operation = response.data;
      else applyFailed = true;
    }
  } catch {
    if (!request.signal.aborted) applyFailed = true;
  } finally {
    if (!request.signal.aborted) {
      updating = false;
      void readOperation(request);
    }
  }
}
async function askApply(version: string) {
  const ok = await confirm.ask({
    title: m.update_confirm_title({ version }),
    description: m.update_confirm(),
    action: m.update_apply(),
    tone: 'danger',
  });
  if (ok && controller) void changeOperation(controller, version);
}

let pending = $state(true);
let controller: AbortController | undefined;
const labels = {
  disabled: m.update_disabled,
  deferred: m.update_deferred,
  no_release: m.update_no_release,
  source_unavailable: m.update_source_unavailable,
  rate_limited: m.update_rate_limited,
  unavailable: m.update_unavailable,
  up_to_date: m.update_up_to_date,
};

async function check(mode: 'automatic' | 'manual', request: AbortController) {
  pending = true;
  try {
    const response = await postSystemUpdateCheck({ body: { mode }, signal: request.signal });
    if (!request.signal.aborted) result = response.data;
  } catch {
    if (!request.signal.aborted) result = undefined;
  } finally {
    if (!request.signal.aborted) pending = false;
  }
}

async function readStage(request: AbortController) {
  const generation = ++stageRequest;
  try {
    const response = await getSystemUpdateStage({ signal: request.signal });
    if (!request.signal.aborted && generation === stageRequest) stage = response.data;
  } catch {
    if (!request.signal.aborted && generation === stageRequest) stage = undefined;
  }
}

async function verify(version: string, request: AbortController) {
  stageRequest++;
  verifying = true;
  verifyFailed = false;
  try {
    const response = await postSystemUpdateStage({ body: { version }, signal: request.signal });
    if (request.signal.aborted) return;
    if (response.data) stage = response.data;
    else {
      verifyFailed = true;
      await readStage(request);
    }
  } catch {
    if (!request.signal.aborted) {
      verifyFailed = true;
      await readStage(request);
    }
  } finally {
    if (!request.signal.aborted) verifying = false;
  }
}

onMount(() => {
  const request = new AbortController();
  controller = request;
  void check('automatic', request);
  void readStage(request);
  void readOperation(request);
  const timer = setInterval(() => {
    if (!verifying && stage?.status === 'verifying') void readStage(request);
    // A recovery worker can be active while the durable journal still reports recovery_required.
    if (
      !updating &&
      (operation?.state === 'applying' || operation?.state === 'recovery_required' || applyFailed)
    ) {
      void readOperation(request);
    }
  }, 2000);
  return () => {
    clearInterval(timer);
    request.abort();
  };
});
</script>

<Section
  title={m.update_title()}
  description={pending
    ? m.update_checking()
    : result?.status === 'available'
      ? m.update_available({ version: result.latest_version ?? '' })
      : result?.status === 'disabled'
        ? disabledMessage(result)
        : labels[result?.status ?? 'unavailable']()}
>
  {#snippet actions()}
    {#if result?.status === 'disabled'}
      <Status kind="neutral">{m.update_off()}</Status>
    {:else}
      {#if result?.status === 'available' && result.latest_version}
        <Button variant="outline" size="sm" type="button"
          disabled={pending || verifying || stage?.status === 'verifying'}
          onclick={() => {
            if (controller && result?.latest_version) void verify(result.latest_version, controller);
          }}>{m.update_verify()}</Button>
      {/if}
      <Button variant="outline" size="sm" type="button" disabled={pending || verifying}
        onclick={() => { if (controller) void check('manual', controller); }}><RefreshCw aria-hidden="true" />{m.update_check()}</Button>
    {/if}
  {/snippet}
  {#if verifying || stage?.status === 'verifying' || stage?.status === 'verified' || verifyFailed}
  <div aria-live="polite">
    {#if verifying || stage?.status === 'verifying'}
      <p class="text-muted-foreground">{m.update_verifying()}</p>
    {:else if stage?.status === 'verified' && stage.version}
      <p class="mt-2">{m.update_verified({ version: stage.version })}</p>
    {/if}
    {#if verifyFailed}
      <p class="text-danger">{m.update_verify_failed()}</p>
    {/if}
  </div>
  {/if}
  {#if shownOperation || applyFailed}
  <div aria-live="polite">
    {#if shownOperation?.state === 'applying'}
      <p class="mt-2">{m.update_applying()}</p>
    {:else if shownOperation?.state === 'succeeded'}
      <p class="mt-2">{m.update_succeeded({ version: shownOperation.version ?? '' })}</p>
    {:else if shownOperation?.state === 'rolled_back'}
      <p class="text-danger">{m.update_rolled_back()}</p>
    {:else if shownOperation?.state === 'aborted'}
      <p class="mt-2">{m.update_aborted()}</p>
    {:else if shownOperation?.state === 'recovery_required'}
      <p class="text-danger">{m.update_recovery_required()}</p>
    {/if}
    {#if applyFailed}
      <p class="text-danger">{m.update_apply_failed()}</p>
    {/if}
  </div>
  {/if}
  {#if operation?.state === 'recovery_required'}
    <Button variant="outline" type="button" class="mt-2"
      disabled={updating} onclick={() => { if (controller) void changeOperation(controller); }}>
      {m.update_retry_recovery()}
    </Button>
  {:else if operation && operation.state !== 'applying' && stage?.status === 'verified' && stage.version}
    {@const version = stage.version}
    <Button variant="outline" type="button" class="w-fit"
      disabled={updating} onclick={() => askApply(version)}>
      {m.update_apply()}
    </Button>
  {/if}
</Section>

<ConfirmDialog {confirm} />
