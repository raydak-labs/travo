import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { LedStatus } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import { switchState } from '../ui/switch-html.test-helper';
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
  it('lists LEDs by readable name with the kernel name as hint, all on', () => {
    loader.data = { leds, off: false, schedule };
    const body = render(SystemLEDsCard).body;
    expect(body).toContain('Power (blue)');
    expect(body).toContain('blue:run');
    expect(body).toContain('System (white)');
    expect(body).toContain('white:system');
    expect(switchState(body, 'leds-all')).toEqual({ checked: true, disabled: false });
    expect(body).not.toContain('Held off by Travo');
    expect(body).toContain('Night schedule');
  });

  it('held off: the all-LEDs switch is off and says why', () => {
    loader.data = { leds: leds.map((l) => ({ ...l, on: false })), off: true, schedule };
    const body = render(SystemLEDsCard).body;
    expect(switchState(body, 'leds-all')).toEqual({ checked: false, disabled: false });
    expect(body).toContain('Held off by Travo');
  });

  it('shows the dark window only while night mode is on', () => {
    loader.data = { leds, off: false, schedule };
    const off = render(SystemLEDsCard).body;
    expect(switchState(off, 'leds-night')?.checked).toBe(false);
    expect(off.match(/<input[^>]*type="time"[^>]*>/g) ?? []).toHaveLength(0);
    loader.data = { leds, off: false, schedule: { ...schedule, enabled: true } };
    const on = render(SystemLEDsCard).body.match(/<input[^>]*type="time"[^>]*>/g) ?? [];
    expect(on).toHaveLength(2);
    for (const input of on) expect(input).not.toContain('disabled=""');
  });

  it('no LEDs: says so', () => {
    loader.data = { leds: [], off: false, schedule };
    expect(render(SystemLEDsCard).body).toContain('no LEDs');
  });
});
