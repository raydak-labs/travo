import { describe, expect, it } from 'vitest';
import { topologyModel } from './topology';
import { connectivity, throughput, uplinks } from './topology.test-inputs';
import { uplinkFacts } from './uplink-facts';

const model = topologyModel({ uplinks, connectivity, throughput });
const source = (name: string) => model.sources.find((s) => s.name === name);

describe('uplinkFacts', () => {
  it('lists what the WiFi uplink carries, in reading order', () => {
    const wwan = source('wwan');
    if (!wwan) throw new Error('fixture has wwan');
    const facts = uplinkFacts(wwan, uplinks.uplinks[1]);
    expect(facts.map(([label]) => label)).toEqual([
      'Protocol',
      'Network',
      'Band',
      'Signal',
      'Address',
      'Gateway',
      'Device',
      'Traffic',
    ]);
    expect(Object.fromEntries(facts)).toMatchObject({
      Network: 'HomeUplink',
      Band: '5 GHz',
      Signal: '-63 dBm',
      Gateway: '192.168.90.2',
    });
  });

  it('shows only the protocol for a WAN that is down', () => {
    const wan = source('wan');
    if (!wan) throw new Error('fixture has wan');
    expect(uplinkFacts(wan, uplinks.uplinks[0])).toEqual([['Protocol', 'DHCP (automatic)']]);
  });
});
