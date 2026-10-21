import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';

export const datausageHandlers = [
  http.get(API_ROUTES.network.dataUsage, () => {
    return HttpResponse.json({
      available: true,
      interfaces: [
        {
          name: 'eth0',
          label: 'Ethernet WAN',
          today: { rx_bytes: 52428800, tx_bytes: 15728640 },
          month: { rx_bytes: 2147483648, tx_bytes: 536870912 },
          total: { rx_bytes: 10737418240, tx_bytes: 2684354560 },
        },
        {
          name: 'wwan0',
          label: 'WiFi Uplink',
          today: { rx_bytes: 10485760, tx_bytes: 3145728 },
          month: { rx_bytes: 8589934592, tx_bytes: 1073741824 },
          total: { rx_bytes: 21474836480, tx_bytes: 5368709120 },
        },
      ],
    });
  }),

  http.get(API_ROUTES.network.dataUsageBudget, () => {
    return HttpResponse.json({
      budgets: [
        {
          interface: 'wwan0',
          monthly_limit_bytes: 10737418240,
          warning_threshold_pct: 80,
          reset_day: 1,
        },
      ],
    });
  }),

  http.put(API_ROUTES.network.dataUsageBudget, () => {
    return HttpResponse.json({ status: 'ok' });
  }),

  http.post(API_ROUTES.network.dataUsageReset, () => {
    return HttpResponse.json({ status: 'ok' });
  }),
];
