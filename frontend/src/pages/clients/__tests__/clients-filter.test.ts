import { describe, expect, it } from 'vitest';
import { filterClientsBySearch } from '@/pages/clients/clients-filter';
import type { Client } from '@shared/index';

const sample: Client[] = [
  {
    mac_address: 'aa:bb:cc:dd:ee:01',
    ip_address: '192.168.1.10',
    hostname: 'phone',
    alias: 'MyPhone',
    interface_name: 'br-lan',
    connected_since: '',
    rx_bytes: 0,
    tx_bytes: 0,
  },
];

describe('filterClientsBySearch', () => {
  it('returns all when search is empty', () => {
    expect(filterClientsBySearch(sample, '')).toEqual(sample);
  });

  it('filters by IP substring', () => {
    expect(filterClientsBySearch(sample, '192.168')).toEqual(sample);
    expect(filterClientsBySearch(sample, '10.0')).toEqual([]);
  });

  it('filters by alias case-insensitively', () => {
    expect(filterClientsBySearch(sample, 'myphone')).toEqual(sample);
  });
});
