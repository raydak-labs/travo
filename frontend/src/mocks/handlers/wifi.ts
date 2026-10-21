import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import {
  mockAPConfigs,
  mockGuestWifi,
  mockMACAddresses,
  mockRadios,
  mockSavedNetworks,
  mockWifiConnection,
  mockWifiScanResults,
} from '../data';

export const wifiHandlers = [
  http.get(API_ROUTES.wifi.scan, () => {
    return HttpResponse.json(mockWifiScanResults);
  })

  http.get(API_ROUTES.wifi.connection, () => {
    return HttpResponse.json(mockWifiConnection);
  })

  http.get(API_ROUTES.wifi.health, () => {
    return HttpResponse.json({ status: 'ok', issues: [] });
  })

  http.post(API_ROUTES.wifi.connect, async ({ request }) => {
    const body = (await request.json()) as { ssid: string; password: string };
    return HttpResponse.json({
      status: 'ok',
      apply: { pending: true, token: `apply-${body.ssid}`, rollback_timeout_seconds: 30 },
    });
  })

  http.post(API_ROUTES.wifi.disconnect, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.put(API_ROUTES.wifi.mode, async ({ request }) => {
    const body = (await request.json()) as { mode: string };
    return HttpResponse.json({
      status: 'ok',
      apply: { pending: true, token: `apply-mode-${body.mode}`, rollback_timeout_seconds: 30 },
    });
  })

  http.post(API_ROUTES.wifi.applyConfirm, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.wifi.saved, () => {
    return HttpResponse.json(mockSavedNetworks);
  })

  http.get(API_ROUTES.wifi.radios, () => {
    return HttpResponse.json(mockRadios);
  })

  http.put(`${API_ROUTES.wifi.radios}/:name/role`, () => {
    return HttpResponse.json({ status: 'ok', pending: false });
  })

  http.get(API_ROUTES.wifi.bandSwitching, () => {
    return HttpResponse.json({
      config: {
        enabled: false,
        preferred_band: '5g',
        check_interval_sec: 10,
        down_switch_threshold_dbm: -70,
        down_switch_delay_sec: 30,
        up_switch_threshold_dbm: -60,
        up_switch_delay_sec: 60,
        min_viable_signal_dbm: -80,
      },
      status: {
        state: 'inactive',
        current_band: '',
        signal_dbm: 0,
        weak_signal_secs: 0,
        cooldown_sec: 0,
      },
    });
  })

  http.put(API_ROUTES.wifi.bandSwitching, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.delete(`${API_ROUTES.wifi.deleteSaved}/:section`, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.put(API_ROUTES.wifi.savedPriority, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.wifi.ap, () => {
    return HttpResponse.json(mockAPConfigs);
  })
  http.get(API_ROUTES.wifi.repeaterOptions, () => {
    return HttpResponse.json({ allow_ap_on_sta_radio: false });
  })
  http.put(API_ROUTES.wifi.repeaterOptions, async ({ request }) => {
    const body = (await request.json()) as { allow_ap_on_sta_radio: boolean };
    return HttpResponse.json({
      status: 'ok',
      allow_ap_on_sta_radio: body.allow_ap_on_sta_radio,
    });
  })
  http.post(API_ROUTES.wifi.repeaterReconcile, () => {
    return HttpResponse.json({
      status: 'ok',
      apply: { pending: true, token: 'apply-repeater-reconcile', rollback_timeout_seconds: 30 },
    });
  })
  http.put(`${API_ROUTES.wifi.ap}/:section`, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.wifi.mac, () => {
    return HttpResponse.json(mockMACAddresses);
  })
  http.put(API_ROUTES.wifi.mac, () => {
    return HttpResponse.json({ status: 'ok' });
  })
  http.post(API_ROUTES.wifi.macRandomize, () => {
    return HttpResponse.json({ status: 'ok', mac: '02:ab:cd:ef:12:34' });
  })

  http.get(API_ROUTES.wifi.guest, () => {
    return HttpResponse.json(mockGuestWifi);
  })
  http.put(API_ROUTES.wifi.guest, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.wifi.radio, () => {
    return HttpResponse.json({ enabled: true });
  })
  http.put(API_ROUTES.wifi.radio, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.wifi.autoreconnect, () => {
    return HttpResponse.json({ enabled: false });
  })
  http.put(API_ROUTES.wifi.autoreconnect, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.wifi.schedule, () => {
    return HttpResponse.json({ enabled: false, on_time: '07:00', off_time: '23:00', days: [] });
  })

  http.put(API_ROUTES.wifi.schedule, () => {
    return HttpResponse.json({ status: 'ok' });
  })

  http.get(API_ROUTES.wifi.macPolicies, () => {
    return HttpResponse.json({ policies: [] });
  })

  http.put(API_ROUTES.wifi.macPolicies, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
];
