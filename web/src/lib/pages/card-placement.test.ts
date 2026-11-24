import { describe, expect, it } from 'vitest';

// Every page module the shell can show, as source text.
const sources = import.meta.glob(
  [
    './Internet.svelte',
    './Network*.svelte',
    '!./NetworkInterfacesCard.svelte',
    './System.svelte',
    './SystemLEDs.svelte',
    './SystemLogs.svelte',
    './SystemUpdates.svelte',
    './SystemTokens.svelte',
    './SystemReset.svelte',
    './Services.svelte',
  ],
  { query: '?raw', import: 'default', eager: true },
) as Record<string, string>;

// Cards and inline sections the Network and System pages rendered before the nav split.
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
];
const INLINE_CARDS = [
  'm.system_device()',
  'm.system_backup()',
  'm.reboot_title()',
  'm.system_about()',
  'm.password_change_title()',
];

function pagesWith(needle: string): string[] {
  return Object.entries(sources)
    .filter(([, source]) => source.includes(needle))
    .map(([path]) => path);
}

describe('card placement after the nav split', () => {
  it('loads the expected page modules', () => {
    expect(Object.keys(sources)).toHaveLength(14);
  });

  it.each(CARD_COMPONENTS)('%s renders on exactly one page', (name) => {
    expect(pagesWith(`./${name}.svelte'`)).toHaveLength(1);
  });

  it.each(INLINE_CARDS)('%s renders on exactly one page', (title) => {
    expect(pagesWith(`<Card title={${title}}>`)).toHaveLength(1);
  });

  it('drops the link cards the nav replaces', () => {
    expect(pagesWith('tokens_open')).toEqual([]);
    expect(pagesWith('logs_open')).toEqual([]);
  });
});
