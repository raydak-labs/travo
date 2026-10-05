import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

const mocks = vi.hoisted(() => ({
  setRepeaterOptions: vi.fn(),
  setMode: vi.fn(),
  connect: vi.fn(),
  setAP: vi.fn(),
}));

vi.mock('@/hooks/use-wifi', () => ({
  useWifiScan: () => ({ data: [], isLoading: false, refetch: vi.fn() }),
  useWifiConnect: () => ({ mutateAsync: mocks.connect, isPending: false }),
  useWifiMode: () => ({ mutateAsync: mocks.setMode, isPending: false }),
  useWifiConnection: () => ({ data: { mode: 'ap', connected: false } }),
  useAPConfigs: () => ({
    data: [
      {
        section: 'default_radio0',
        band: '2g',
        ssid: 'Old-2G',
        encryption: 'psk2',
        key: 'oldkey24',
      },
      {
        section: 'default_radio1',
        band: '5g',
        ssid: 'Old-5G',
        encryption: 'psk2',
        key: 'oldkey24',
      },
    ],
  }),
  useSetAPConfig: () => ({ mutateAsync: mocks.setAP, isPending: false }),
  useRepeaterOptions: () => ({ data: { allow_ap_on_sta_radio: false } }),
  useSetRepeaterOptions: () => ({ mutateAsync: mocks.setRepeaterOptions, isPending: false }),
}));

import { useRepeaterWizard } from '../use-repeater-wizard';

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.setRepeaterOptions.mockResolvedValue({ status: 'ok' });
  mocks.setMode.mockResolvedValue({ status: 'ok' });
  mocks.connect.mockResolvedValue({ status: 'ok' });
  mocks.setAP.mockResolvedValue({ status: 'ok' });
});

async function runApply(setUpstream: (w: ReturnType<typeof useRepeaterWizard>) => void) {
  const { result } = renderHook(() => useRepeaterWizard(true), { wrapper });
  await waitFor(() => expect(result.current.apConfigs?.length).toBe(2));

  act(() => {
    setUpstream(result.current);
  });
  await act(async () => {
    await result.current.handleApply();
  });
  return result;
}

describe('useRepeaterWizard handleApply', () => {
  it('applies each mutation exactly once — no duplicate mode switch', async () => {
    const result = await runApply((w) =>
      w.setUpstream({ ssid: 'Hotel_Guest', password: 'password123', encryption: 'wpa2' }),
    );

    expect(result.current.done).toBe(true);
    expect(result.current.applyError).toBeNull();
    // A second mode switch would open a second uci apply + confirm + rollback window.
    expect(mocks.setMode).toHaveBeenCalledTimes(1);
    // The mutation variable is now an object so it can carry acknowledge_lockout.
    expect(mocks.setMode).toHaveBeenCalledWith({ mode: 'repeater' });
    expect(mocks.connect).toHaveBeenCalledTimes(1);
    expect(mocks.setAP).toHaveBeenCalledTimes(2);
  });

  it('names the failing step and restores the previous radio state', async () => {
    mocks.setAP.mockImplementation(async (vars: { section: string }) => {
      if (vars.section === 'default_radio1') throw new Error('uci commit failed');
      return { status: 'ok' };
    });

    const result = await runApply((w) =>
      w.setUpstream({ ssid: 'Hotel_Guest', password: 'password123', encryption: 'wpa2' }),
    );

    expect(result.current.done).toBe(false);
    expect(result.current.failedStep).toBe('AP "default_radio1"');
    expect(result.current.applyError).toContain('Step AP "default_radio1" failed');
    expect(result.current.applyError).toContain('uci commit failed');

    // The already-applied AP, the repeater options and the WiFi mode are
    // put back; the upstream connection itself cannot be undone from the client.
    const apSections = mocks.setAP.mock.calls.map((c) => (c[0] as { section: string }).section);
    expect(apSections.filter((s) => s === 'default_radio0')).toHaveLength(2);
    expect(mocks.setMode).toHaveBeenLastCalledWith({ mode: 'ap' });
    expect(mocks.setRepeaterOptions).toHaveBeenLastCalledWith({ allow_ap_on_sta_radio: false });
  });

  it('reports a failure of the upstream connect step', async () => {
    mocks.connect.mockRejectedValue(new Error('no such network'));

    const result = await runApply((w) =>
      w.setUpstream({ ssid: 'Hotel_Guest', password: 'password123', encryption: 'wpa2' }),
    );

    expect(result.current.failedStep).toBe('upstream connection');
    expect(result.current.applyError).toContain('no such network');
    expect(mocks.setAP).not.toHaveBeenCalled();
  });
});
