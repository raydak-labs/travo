import { describe, expect, it } from 'vitest';

// Every page module the shell can show, as source text.
const sources = import.meta.glob(
  [
    './Internet.svelte',
    './Network*.svelte',
    '!./NetworkInterfacesCard.svelte',
    './System.svelte',
    './SystemHardware.svelte',
    './SystemLogs.svelte',
    './SystemFirmware.svelte',
    './SystemAccess.svelte',
    './Services.svelte',
    './AdGuard.svelte',
    './VPN.svelte',
    './WireGuard.svelte',
    './Tailscale.svelte',
  ],
  { query: '?raw', import: 'default', eager: true },
) as Record<string, string>;

// Cards and sections each page group renders; each must sit on exactly one page.
const CARD_COMPONENTS = [
  'WANCard',
  'USBTetherCard',
  'FailoverCard',
  'DataUsageCard',
  'IPv6Card',
  'DDNSCard',
  'NetworkInterfacesCard',
  'LANAddressCard',
  'DHCPServerCard',
  'DNSNamesCard',
  'PortForwardsCard',
  'DiagnosticsCard',
  'WakeOnLanCard',
  'SystemSettingsCard',
  'SystemLEDsCard',
  'SystemButtonsCard',
  'SystemUpdate',
  'SystemOpenWrtCard',
  'SystemFirmwareCard',
  'SystemSSHKeysCard',
  'SystemFactoryResetCard',
  'PasswordCard',
  'DoHCard',
  'AdGuardTopologyCard',
  'VPNKillSwitchRow',
  'VPNSplitRow',
  'VPNDNSRow',
];
const INLINE_CARDS = ['m.system_details()', 'm.tokens_yours()', 'm.vpn_saved_profiles()'];

function pagesWith(needle: string): string[] {
  return Object.entries(sources)
    .filter(([, source]) => source.includes(needle))
    .map(([path]) => path);
}

describe('card placement', () => {
  it('loads the expected page modules', () => {
    expect(Object.keys(sources)).toHaveLength(15);
  });

  it.each(CARD_COMPONENTS)('%s renders on exactly one page', (name) => {
    expect(pagesWith(`./${name}.svelte'`)).toHaveLength(1);
  });

  it.each(INLINE_CARDS)('%s renders on exactly one page', (title) => {
    expect(pagesWith(`<Section title={${title}}`)).toHaveLength(1);
  });

  it('drops the cards the header menu and the nav replace', () => {
    for (const gone of ['tokens_open', 'logs_open', 'reboot_title', 'system_about']) {
      expect(pagesWith(gone)).toEqual([]);
    }
  });
});
