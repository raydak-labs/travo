import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { ApConfig } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import WiFiSetupCard from './WiFiSetupCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as ApConfig[] | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

// The factory APs of testdata/device/axt1800-25.12.6/factory-default/uci-show-wireless.txt.
const factory: ApConfig[] = [
  {
    section: 'default_radio0',
    radio: 'radio0',
    band: '5g',
    ssid: 'OpenWrt',
    enabled: false,
    guest: false,
    security: 'open',
  },
  {
    section: 'default_radio1',
    radio: 'radio1',
    band: '2g',
    ssid: 'OpenWrt',
    enabled: false,
    guest: false,
    security: 'open',
  },
];

describe('wifi setup card', () => {
  it('asks for a name and key while the WiFi is as OpenWrt ships it', () => {
    loader.data = factory;
    const body = render(WiFiSetupCard, { props: { apply: vi.fn() } }).body;
    expect(body).toContain('Set up WiFi');
    expect(body).toContain('value="Travo"');
    expect(body).toContain('minlength="8"');
    expect(body).toContain('Turn on WiFi');
  });

  it('stays away once an AP is on or has a key', () => {
    loader.data = [{ ...factory[0], enabled: true }, factory[1]] as ApConfig[];
    expect(render(WiFiSetupCard, { props: { apply: vi.fn() } }).body).not.toContain('Set up WiFi');
    loader.data = [{ ...factory[0], security: 'protected' }] as ApConfig[];
    expect(render(WiFiSetupCard, { props: { apply: vi.fn() } }).body).not.toContain('Set up WiFi');
  });
});
