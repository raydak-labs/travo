import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useRepeaterWizard } from '@/components/wifi/repeater-wizard/use-repeater-wizard';

beforeEach(() => {
  localStorage.setItem('openwrt-auth-token', 'test-token');
});

function createWrapper(qc: QueryClient) {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  };
}

describe('useRepeaterWizard', () => {
  it('does not overwrite user toggle when repeater options cache updates after hydration', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    qc.setQueryData(['wifi', 'repeater-options'], { allow_ap_on_sta_radio: true });

    const { result } = renderHook(() => useRepeaterWizard(true), {
      wrapper: createWrapper(qc),
    });

    await waitFor(() => expect(result.current.allowApOnStaRadio).toBe(true));

    act(() => result.current.setAllowApOnStaRadio(false));
    expect(result.current.allowApOnStaRadio).toBe(false);

    act(() => {
      qc.setQueryData(['wifi', 'repeater-options'], { allow_ap_on_sta_radio: true });
    });

    expect(result.current.allowApOnStaRadio).toBe(false);
  });

  it('re-hydrates allowApOnStaRadio after reset when wizard opens again', async () => {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    qc.setQueryData(['wifi', 'repeater-options'], { allow_ap_on_sta_radio: true });

    const { result, rerender } = renderHook(({ open }) => useRepeaterWizard(open), {
      wrapper: createWrapper(qc),
      initialProps: { open: false },
    });

    rerender({ open: true });
    await waitFor(() => expect(result.current.allowApOnStaRadio).toBe(true));

    act(() => result.current.setAllowApOnStaRadio(false));
    expect(result.current.allowApOnStaRadio).toBe(false);

    act(() => result.current.reset());
    rerender({ open: false });

    qc.setQueryData(['wifi', 'repeater-options'], { allow_ap_on_sta_radio: false });
    rerender({ open: true });

    await waitFor(() => expect(result.current.allowApOnStaRadio).toBe(false));
  });
});
