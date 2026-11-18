<script lang="ts">
import CircleCheck from '@lucide/svelte/icons/circle-check';
import Info from '@lucide/svelte/icons/info';
import LoaderCircle from '@lucide/svelte/icons/loader-circle';
import OctagonX from '@lucide/svelte/icons/octagon-x';
import X from '@lucide/svelte/icons/x';
import { onMount } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { m } from '../paraglide/messages.js';
import type { TxnPhase } from './transaction-toast.svelte';

let {
  phase,
  settle,
  closeToast,
}: { phase: TxnPhase; settle: (confirm: boolean) => void; closeToast?: () => void } = $props();

// One toast instance lives through every phase, so the clock restarts when a window opens.
const deadline = $derived(phase.phase === 'pending' ? phase.deadline : 0);
const left = (at: number) => Math.max(0, Math.ceil((deadline - at) / 1000));
let now = $state(0);
// Screen readers hear the window once; the ticking copy is visual only.
let announced = $state(0);
$effect.pre(() => {
  const at = performance.now();
  now = at;
  announced = left(at);
});
onMount(() => {
  const timer = setInterval(() => {
    now = performance.now();
  }, 1000);
  return () => clearInterval(timer);
});
const icons = { success: CircleCheck, info: Info, error: OctagonX };
</script>

<div class="flex w-full gap-3 rounded-lg border border-border bg-surface p-4 text-sm text-fg shadow-lg">
  {#if phase.phase === 'applying' || phase.phase === 'restoring'}
    <LoaderCircle aria-hidden="true" class="mt-0.5 size-4 shrink-0 animate-spin" />
    <p>{phase.text}</p>
  {:else if phase.phase === 'pending'}
    <div class="grid w-full gap-2">
      <p class="font-semibold">{m.txn_pending_title()}</p>
      <p class="text-muted-foreground">
        <span aria-hidden="true">{m.txn_pending_countdown({ seconds: left(now) })}</span>
        <span class="sr-only">{m.txn_pending_countdown({ seconds: announced })}</span>
      </p>
      <div class="flex flex-wrap gap-2">
        <Button size="sm" disabled={phase.settling} onclick={() => settle(true)}>{m.txn_confirm()}</Button>
        <Button size="sm" variant="outline" disabled={phase.settling} onclick={() => settle(false)}>{m.txn_rollback()}</Button>
      </div>
    </div>
  {:else if phase.phase === 'done'}
    {@const Icon = icons[phase.tone]}
    <Icon aria-hidden="true" class="mt-0.5 size-4 shrink-0 {phase.tone === 'error' ? 'text-danger' : phase.tone === 'success' ? 'text-ok' : 'text-info'}" />
    <p class="flex-1">{phase.text}</p>
    <Button variant="ghost" size="icon-xs" aria-label={m.txn_dismiss()} onclick={() => closeToast?.()}>
      <X aria-hidden="true" />
    </Button>
  {/if}
</div>
