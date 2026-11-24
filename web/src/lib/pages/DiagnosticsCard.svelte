<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import {
  DIAGNOSTIC_TOOLS,
  type Diagnosis,
  type DiagnosticTool,
  diagnosisStatus,
  runDiagnostic,
} from '../api/diagnostics';
import { m } from '../paraglide/messages.js';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import SelectField from '../ui/SelectField.svelte';
import Status from '../ui/Status.svelte';

const labels: Record<DiagnosticTool, () => string> = {
  ping: m.diag_tool_ping,
  traceroute: m.diag_tool_traceroute,
  lookup: m.diag_tool_lookup,
};

let tool = $state<DiagnosticTool>('ping');
let target = $state('');
let running = $state(false);
let result = $state<Diagnosis>();
const request = new AbortController();
onDestroy(() => request.abort());

async function run(event: SubmitEvent) {
  event.preventDefault();
  if (running || !target.trim()) return;
  running = true;
  const outcome = await runDiagnostic(tool, target, request.signal);
  running = false;
  if (request.signal.aborted) return;
  if ('error' in outcome) notify.error(outcome.error);
  else result = outcome.diagnosis;
}
</script>

<Section title={m.diag_run_title()} description={m.diag_intro()}>
  <form class="flex flex-wrap items-end gap-3" onsubmit={run}>
    <Label class="grid gap-1">
      {m.diag_tool()}
      <SelectField bind:value={tool} disabled={running} class="w-40"
        options={DIAGNOSTIC_TOOLS.map((option) => ({ value: option, label: labels[option]() }))} />
    </Label>
    <Label class="grid min-w-40 flex-1 gap-1">
      {m.diag_target()}
      <Input bind:value={target} maxlength={253} autocomplete="off" autocapitalize="off" spellcheck={false}
        disabled={running} placeholder="openwrt.org" />
    </Label>
    <Button variant="outline" type="submit" disabled={running || !target.trim()}>
      {running ? m.diag_running() : m.diag_run()}
    </Button>
  </form>
  {#if result}
    {@const state = diagnosisStatus(result)}
    <div class="grid gap-2" aria-live="polite">
      <p class="flex flex-wrap items-center gap-2">
        <span class="break-all font-medium">{labels[result.tool]()} {result.target}</span>
        <Status kind={state.kind}>{state.label}</Status>
      </p>
      <pre class="max-h-96 overflow-auto rounded-md bg-muted p-3 font-mono text-xs whitespace-pre-wrap break-all">{result.output}</pre>
      {#if result.truncated}<p class="text-sm text-muted-foreground">{m.diag_truncated()}</p>{/if}
    </div>
  {/if}
</Section>
