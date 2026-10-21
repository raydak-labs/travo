import { adguardHandlers } from './adguard';
import { authHandlers } from './auth';
import { captiveHandlers } from './captive';
import { dataUsageHandlers } from './datausage';
import { networkHandlers } from './network';
import { servicesHandlers } from './services';
import { speedtestHandlers } from './speedtest';
import { sqmHandlers } from './sqm';
import { systemHandlers } from './system';
import { usbTetheringHandlers } from './usbtethering';
import { vpnHandlers } from './vpn';
import { wifiHandlers } from './wifi';

export const handlers = [
  ...authHandlers,
  ...systemHandlers,
  ...networkHandlers,
  ...dataUsageHandlers,
  ...usbTetheringHandlers,
  ...sqmHandlers,
  ...wifiHandlers,
  ...vpnHandlers,
  ...servicesHandlers,
  ...captiveHandlers,
  ...adguardHandlers,
  ...speedtestHandlers,
];
