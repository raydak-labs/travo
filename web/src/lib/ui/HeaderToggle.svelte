<script lang="ts">
import Label from '$lib/components/ui/label/label.svelte';
import Switch from '$lib/components/ui/switch/switch.svelte';
import { m } from '../paraglide/messages.js';
import Status from './Status.svelte';
import type { StatusKind } from './status';

let {
  id,
  label,
  status,
  kind,
  checked,
  disabled = false,
  stale = false,
  title,
  onchange,
}: {
  id: string;
  label: string;
  status: string;
  kind: StatusKind;
  checked: boolean;
  disabled?: boolean;
  stale?: boolean;
  title?: string;
  onchange: (checked: boolean) => void;
} = $props();
</script>

<div data-slot="header-toggle" class="inline-flex min-h-8 max-w-full flex-wrap items-center gap-2 rounded-md border border-border bg-card px-2.5 py-1 text-sm shadow-xs" {title}>
  <Label for={id} class={disabled ? 'cursor-not-allowed opacity-70' : 'cursor-pointer'}>{label}</Label>
  <Status {kind}>{status}</Status>
  {#if stale}<Status kind="stale">{m.load_stale()}</Status>{/if}
  <Switch {id} {disabled} bind:checked={() => checked, (on) => onchange(on)} />
</div>
