<script lang="ts">
import { onDestroy } from 'svelte';
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import '../api/client';
import { type DnsVerification, verifyVpndns } from '../api/gen';
import { fetchVpnDns } from '../api/vpn-status';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';

const request = new AbortController();
const dns = createLoader(fetchVpnDns, 10000);
let verifying = $state(false);
let verified = $state<DnsVerification['result']>();
let verifyError = $state('');
onDestroy(() => {
  dns.stop();
  request.abort();
});

const layers = {
  active: { kind: 'ok', label: m.vpn_dns_active },
  inactive: { kind: 'neutral', label: m.vpn_dns_inactive },
  failed: { kind: 'danger', label: m.vpn_dns_failed },
} as const;

const results = {
  ok: { kind: 'ok', label: m.vpn_dns_verify_ok },
  not_active: { kind: 'neutral', label: m.vpn_dns_verify_not_active },
  overridden: { kind: 'warn', label: m.vpn_dns_verify_overridden },
  mismatch: { kind: 'warn', label: m.vpn_dns_verify_mismatch },
  lookup_failed: { kind: 'danger', label: m.vpn_dns_verify_lookup_failed },
  layer_failed: { kind: 'danger', label: m.vpn_dns_failed },
} as const;

async function verify() {
  if (verifying) return;
  verifying = true;
  verified = undefined;
  verifyError = '';
  try {
    const response = await verifyVpndns({ signal: request.signal });
    if (request.signal.aborted) return;
    if (response.data) verified = response.data.result;
    else verifyError = m.vpn_dns_unavailable();
    void dns.retry();
  } catch {
    if (!request.signal.aborted) verifyError = m.vpn_dns_unavailable();
  } finally {
    verifying = false;
  }
}
</script>

<Section title={m.vpn_dns()}>
  <LoadState status={dns.status} retry={dns.retry} stale={dns.data !== undefined}>
    {#if dns.data}
      {@const info = dns.data}
      {@const layer = layers[info.layer]}
      <div class="flex flex-wrap items-center gap-2">
        <Status kind={layer.kind}>{layer.label()}</Status>
        {#if info.layer === 'active' && info.top && info.top !== 'vpn'}
          <Status kind="warn">{m.vpn_dns_overridden({ layer: info.top })}</Status>
        {/if}
      </div>
      <SettingsRow label={m.vpn_dns_resolvers()}>
        {#snippet control()}
          <span class="break-all">{info.resolvers?.length ? info.resolvers.join(', ') : m.vpn_dns_none()}</span>
        {/snippet}
      </SettingsRow>
    {/if}
  </LoadState>
  <Button variant="outline" size="sm" type="button" class="w-fit" disabled={verifying} onclick={verify}>{m.vpn_dns_verify()}</Button>
  {#if verifying}
    <p role="status">{m.vpn_dns_verifying()}</p>
  {:else if verified}
    {@const result = results[verified]}
    <p role="status"><Status kind={result.kind}>{result.label()}</Status></p>
  {/if}
  {#if verifyError}<Alert class="text-danger">{verifyError}</Alert>{/if}
</Section>
