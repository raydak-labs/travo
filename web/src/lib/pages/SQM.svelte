<script lang="ts">
import ExternalLink from '@lucide/svelte/icons/external-link';
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Switch from '$lib/components/ui/switch/switch.svelte';
import { luciUrl } from '../api/catalog';
import { fetchInterfaces } from '../api/network-view';
import {
  fetchSQM,
  SQM_LINKLAYERS,
  SQM_QDISCS,
  SQM_SCRIPTS,
  type SqmSettings,
  saveSQM,
  sqmInterfaceOptions,
} from '../api/sqm';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import SelectField from '../ui/SelectField.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';

const sqm = createLoader(fetchSQM);
const net = createLoader(fetchInterfaces);
onDestroy(() => {
  sqm.stop();
  net.stop();
});

let form = $state<SqmSettings>({
  enabled: false,
  interface: 'eth1',
  download: 0,
  upload: 0,
  qdisc: 'cake',
  script: 'piece_of_cake.qos',
  linklayer: 'none',
  overhead: 0,
});
let filled = false;
let busy = $state(false);

$effect(() => {
  const data = sqm.data;
  if (!data || filled) return;
  filled = true;
  form = { ...data };
});

const interfaceOptions = $derived(sqmInterfaceOptions(net.data?.interfaces ?? [], form.interface));

const linklayerLabels: Record<SqmSettings['linklayer'], () => string> = {
  none: m.sqm_linklayer_none,
  ethernet: m.sqm_linklayer_ethernet,
  atm: m.sqm_linklayer_atm,
};

async function save(event: SubmitEvent) {
  event.preventDefault();
  if (busy) return;
  busy = true;
  notify.progress(m.sqm_applying(), 'sqm');
  const result = await saveSQM(form);
  busy = false;
  if ('error' in result) {
    notify.error(result.error, 'sqm');
    return;
  }
  notify.success(m.sqm_saved(), 'sqm');
  form = { ...result.settings };
  filled = true;
  void sqm.retry();
}

const formId = $props.id();
const luci = luciUrl(window.location, 'admin/network/sqm');
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.sqm_title()} description={m.sqm_page_description()}>
    {#snippet actions()}
      <Button variant="ghost" size="sm" href={luci} target="_blank" rel="noopener">
        {m.sqm_open_luci()}<ExternalLink aria-hidden="true" />
      </Button>
    {/snippet}
  </PageHeader>

  <Section title={m.sqm_section_title()} description={m.sqm_section_description()}>
    <LoadState status={sqm.status} retry={sqm.retry} stale={sqm.data !== undefined}>
      {#if sqm.data}
        <form id={formId} onsubmit={save}>
          <SettingsRow label={m.sqm_enabled()} for="{formId}-enabled" description={m.sqm_enabled_help()}>
            {#snippet control()}
              <Switch id="{formId}-enabled" bind:checked={form.enabled} disabled={busy} />
            {/snippet}
          </SettingsRow>
          <SettingsRow label={m.sqm_interface()} description={m.sqm_interface_help()}>
            {#snippet control()}
              <SelectField
                bind:value={form.interface}
                disabled={busy}
                class="w-64"
                options={interfaceOptions}
              />
            {/snippet}
          </SettingsRow>
          <SettingsRow label={m.sqm_download()} for="{formId}-download" description={m.sqm_download_help()}>
            {#snippet control()}
              <Input
                id="{formId}-download"
                type="number"
                min={0}
                max={10000000}
                step={1}
                class="w-28"
                bind:value={form.download}
                required
                disabled={busy}
              />
            {/snippet}
          </SettingsRow>
          <SettingsRow label={m.sqm_upload()} for="{formId}-upload" description={m.sqm_upload_help()}>
            {#snippet control()}
              <Input
                id="{formId}-upload"
                type="number"
                min={0}
                max={10000000}
                step={1}
                class="w-28"
                bind:value={form.upload}
                required
                disabled={busy}
              />
            {/snippet}
          </SettingsRow>
          <SettingsRow label={m.sqm_qdisc()}>
            {#snippet control()}
              <SelectField
                bind:value={form.qdisc}
                disabled={busy}
                class="w-64"
                options={SQM_QDISCS.map((v) => ({ value: v, label: v }))}
              />
            {/snippet}
          </SettingsRow>
          <SettingsRow label={m.sqm_script()}>
            {#snippet control()}
              <SelectField
                bind:value={form.script}
                disabled={busy}
                class="w-64"
                options={SQM_SCRIPTS.map((v) => ({ value: v, label: v }))}
              />
            {/snippet}
          </SettingsRow>
          <SettingsRow label={m.sqm_linklayer()}>
            {#snippet control()}
              <SelectField
                bind:value={form.linklayer}
                disabled={busy}
                class="w-64"
                options={SQM_LINKLAYERS.map((v) => ({ value: v, label: linklayerLabels[v]() }))}
              />
            {/snippet}
          </SettingsRow>
          <SettingsRow label={m.sqm_overhead()} for="{formId}-overhead" description={m.sqm_overhead_help()}>
            {#snippet control()}
              <Input
                id="{formId}-overhead"
                type="number"
                min={0}
                max={256}
                step={1}
                class="w-28"
                bind:value={form.overhead}
                required
                disabled={busy}
              />
            {/snippet}
          </SettingsRow>
        </form>
      {/if}
    </LoadState>
    {#snippet footer()}
      {#if sqm.data}
        <Button variant="default" type="submit" form={formId} disabled={busy}>{m.sqm_save()}</Button>
      {/if}
    {/snippet}
  </Section>
</div>
