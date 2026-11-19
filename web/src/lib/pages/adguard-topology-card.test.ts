import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import { adguardError, topologyRefusal } from '../api/adguard';
import AdGuardTopologyCard from './AdGuardTopologyCard.svelte';

describe('AdGuard topology card', () => {
  it('offers the expert switch behind the disclosure', () => {
    const body = render(AdGuardTopologyCard, { props: { expert: false, refresh: () => {} } }).body;
    expect(body).toContain('Expert: AdGuard Home on port 53');
  });

  it('shows the expert state with the addresses AdGuard answers on', () => {
    const body = render(AdGuardTopologyCard, {
      props: { expert: true, hosts: ['127.0.0.1', '192.168.1.1'], refresh: () => {} },
    }).body;
    expect(body).toContain('Back to the default setup');
    expect(body).toContain('127.0.0.1, 192.168.1.1');
  });

  it('maps refusals by code', () => {
    expect(topologyRefusal('adguard_topology_no_change').message).toBeDefined();
    expect(topologyRefusal('adguard_topology_not_ready').error).toContain('loopback');
    expect(topologyRefusal('adguard_busy').error).toBe(adguardError('adguard_busy'));
    expect(adguardError('adguard_expert_topology')).toContain('port 53');
  });
});
