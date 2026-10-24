import { authHandlers } from './auth';
import { systemHandlers } from './system';
import { networkHandlers } from './network';
import { datausageHandlers } from './datausage';
import { usbtetheringHandlers } from './usbtethering';
import { sqmHandlers } from './sqm';
import { wifiHandlers } from './wifi';
import { vpnHandlers } from './vpn';
import { servicesHandlers } from './services';
import { captiveHandlers } from './captive';
import { adguardHandlers } from './adguard';
import { speedtestHandlers } from './speedtest';

export const handlers = [
  ...authHandlers,
  ...systemHandlers,
  ...networkHandlers,
  ...datausageHandlers,
  ...usbtetheringHandlers,
  ...sqmHandlers,
  ...wifiHandlers,
  ...vpnHandlers,
  ...servicesHandlers,
  ...captiveHandlers,
  ...adguardHandlers,
  ...speedtestHandlers,
];
