<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import '../api/client';
import { type DnsVerification, verifyVpndns } from '../api/gen';
import { fetchVpnDns } from '../api/vpn-status';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
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

<!-- One row in the VPN page's traffic rules: the DNS layer state, its resolvers and a check. -->
<SettingsRow
  label={m.vpn_dns()}
  description={dns.data ? `${m.vpn_dns_resolvers()}: ${dns.data.resolvers?.length ? dns.data.resolvers.join(', ') : m.vpn_dns_none()}` : undefined}
>
  {#snippet control()}
    <LoadState status={dns.status} retry={dns.retry} stale={dns.data !== undefined}>
      {#if dns.data}
        {@const info = dns.data}
        {@const layer = layers[info.layer]}
        <Status kind={layer.kind}>{layer.label()}</Status>
        {#if info.layer === 'active' && info.top && info.top !== 'vpn'}
          <Status kind="warn">{m.vpn_dns_overridden({ layer: info.top })}</Status>
        {/if}
      {/if}
    </LoadState>
    {#if verified && !verifying}
      {@const result = results[verified]}
      <span role="status"><Status kind={result.kind}>{result.label()}</Status></span>
    {/if}
    <Button variant="outline" size="sm" type="button" disabled={verifying} onclick={verify}>{verifying ? m.vpn_dns_verifying() : m.vpn_dns_verify()}</Button>
  {/snippet}
</SettingsRow>
{#if verifyError}<p role="alert" class="text-sm text-danger">{verifyError}</p>{/if}
