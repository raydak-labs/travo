import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { ScheduleView } from '../api/wifi-schedule';
import type { LoadStatus } from '../ui/loader.svelte';
import WiFiScheduleCard from './WiFiScheduleCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as ScheduleView | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

const ap = {
  band: '5g',
  enabled: true,
  guest: false,
  radio: 'radio1',
  security: 'protected',
} as const;
const rule = {
  id: 'a1b2c3d4',
  section: 'default_radio1',
  off: '23:00',
  on: '07:00',
  days: [1, 2, 3, 4, 5],
  enabled: true,
  acknowledge_lockout: false,
};

describe('wifi schedule card', () => {
  it('lists rules by SSID with window, state, last toggle and the router zone', () => {
    loader.data = {
      rules: [rule, { ...rule, id: 'ffff0000', days: [0, 6], enabled: false }],
      runs: [
        {
          id: rule.id,
          at: Date.UTC(2026, 10, 16, 22, 0) / 1000,
          action: 'off',
          outcome: 'applied',
        },
      ],
      cronRunning: true,
      aps: [{ ...ap, section: 'default_radio1', ssid: 'Home' }],
      zone: 'Europe/Berlin',
    };
    const body = render(WiFiScheduleCard).body;
    expect(body).toContain('local time (Europe/Berlin)');
    expect(body).toContain('Off 23:00, on 07:00, Mon to Fri');
    expect(body).toContain('Off 23:00, on 07:00, Sat and Sun');
    expect(body).toContain('Last: Mon 23:00, off, done');
    expect(body).toContain('aria-label="Edit schedule for Home"');
    expect(body).toContain('Paused');
    expect(body).toContain('Resume');
    expect(body).not.toContain('cron service is not running');
  });

  it('empty: says so, warns when cron is down, offers Add', () => {
    loader.data = {
      rules: [],
      runs: [],
      cronRunning: false,
      aps: [{ ...ap, section: 's', ssid: 'Home' }],
    };
    const body = render(WiFiScheduleCard).body;
    expect(body).toContain('No schedule.');
    expect(body).toContain('cron service is not running');
    expect(body).toContain('local time, set under System settings');
    expect(body).toContain('Add schedule');
  });
});
