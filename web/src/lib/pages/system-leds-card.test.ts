import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { LedStatus } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import SystemLEDsCard from './SystemLEDsCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as LedStatus | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

const leds = [
  { name: 'blue:run', trigger: 'none', on: true },
  { name: 'white:system', trigger: 'none', on: false },
];
const schedule = { enabled: false, off_at: '23:00', on_at: '07:00' };

describe('LEDs card', () => {
  it('lists LEDs with their state and offers to turn them off', () => {
    loader.data = { leds, off: false, schedule };
    const body = render(SystemLEDsCard).body;
    expect(body).toContain('blue:run');
    expect(body).toContain('white:system');
    expect(body).toContain('Turn LEDs off');
    expect(body).not.toContain('Held off by Travo');
    expect(body).toContain('Night schedule');
  });

  it('held off: offers to restore the default', () => {
    loader.data = { leds: leds.map((l) => ({ ...l, on: false })), off: true, schedule };
    const body = render(SystemLEDsCard).body;
    expect(body).toContain('Restore default LEDs');
    expect(body).toContain('Held off by Travo');
  });

  it('no LEDs: says so', () => {
    loader.data = { leds: [], off: false, schedule };
    expect(render(SystemLEDsCard).body).toContain('no LEDs');
  });
});
