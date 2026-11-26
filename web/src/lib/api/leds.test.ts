import { afterEach, describe, expect, it, vi } from 'vitest';
import { getLeds, type LedStatus, setLedSchedule, setLeds } from './gen';
import { fetchLEDs, ledsError, saveLEDSchedule, scheduleProblem, setLEDsOff } from './leds';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({ getLeds: vi.fn(), setLeds: vi.fn(), setLedSchedule: vi.fn() }));

afterEach(() => vi.resetAllMocks());

// The AXT1800 LEDs of testdata leds/sysfs-default.tsv (mmc0:: is left to the kernel).
const status: LedStatus = {
  leds: [
    { name: 'blue:run', trigger: 'none', on: true, held_off: false },
    { name: 'white:system', trigger: 'none', on: false, held_off: false },
  ],
  off: false,
  schedule: { enabled: false, off_at: '23:00', on_at: '07:00' },
};

function reply<T>(data: T | undefined, error?: { code?: string }, status = 200) {
  return {
    data,
    error,
    request: new Request('http://localhost'),
    response: new Response(null, { status }),
  } as never;
}

describe('LED helpers', () => {
  it('loads the status and fails with the HTTP status', async () => {
    vi.mocked(getLeds).mockResolvedValue(reply(status));
    expect(await fetchLEDs()).toEqual(status);
    vi.mocked(getLeds).mockResolvedValue(reply(undefined, undefined, 503));
    await expect(fetchLEDs()).rejects.toMatchObject({ status: 503 });
  });

  it('checks the schedule like the daemon', () => {
    expect(scheduleProblem({ enabled: true, off_at: '23:00', on_at: '07:00' })).toBeUndefined();
    expect(scheduleProblem({ enabled: true, off_at: '07:00', on_at: '07:00' })).toContain('differ');
    expect(scheduleProblem({ enabled: false, off_at: '07:00', on_at: '07:00' })).toBeUndefined();
    expect(scheduleProblem({ enabled: false, off_at: '24:00', on_at: '07:00' })).toContain('HH:MM');
  });

  it('maps codes to messages', () => {
    expect(ledsError('led_schedule_invalid')).toContain('HH:MM');
    expect(ledsError('uci_busy')).toContain('in progress');
    expect(ledsError(undefined)).toContain('could not be changed');
  });

  it('sends off and the schedule, reporting refusals', async () => {
    vi.mocked(setLeds).mockResolvedValue(reply({ ...status, off: true }));
    expect(await setLEDsOff(true)).toEqual({ status: { ...status, off: true } });
    expect(setLeds).toHaveBeenCalledWith({ body: { off: true } });
    vi.mocked(setLedSchedule).mockResolvedValue(
      reply(undefined, { code: 'led_schedule_invalid' }, 400),
    );
    expect(await saveLEDSchedule(status.schedule)).toEqual({
      error: ledsError('led_schedule_invalid'),
    });
    vi.mocked(setLeds).mockRejectedValue(new Error('network'));
    expect(await setLEDsOff(false)).toEqual({ error: ledsError(undefined) });
  });
});
