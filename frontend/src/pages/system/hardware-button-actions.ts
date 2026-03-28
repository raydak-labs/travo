import type { ButtonAction } from '@shared/index';

export function hardwareButtonActionLabel(action: ButtonAction): string {
  switch (action) {
    case 'none':
      return 'Do nothing';
    case 'vpn_toggle':
      return 'Toggle VPN';
    case 'wifi_toggle':
      return 'Toggle WiFi';
    case 'led_toggle':
      return 'Toggle LEDs';
    case 'reboot':
      return 'Reboot';
    default:
      return action;
  }
}

export const HARDWARE_BUTTON_ACTION_OPTIONS: readonly { value: ButtonAction; label: string }[] = [
  { value: 'none', label: 'Do nothing' },
  { value: 'vpn_toggle', label: 'Toggle VPN' },
  { value: 'wifi_toggle', label: 'Toggle WiFi' },
  { value: 'led_toggle', label: 'Toggle LEDs' },
  { value: 'reboot', label: 'Reboot' },
];
