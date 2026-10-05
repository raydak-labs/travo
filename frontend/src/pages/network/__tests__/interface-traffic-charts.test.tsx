import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';

// recharts draws nothing in a zero-sized jsdom container; the contract under
// test is whether a chart is rendered at all for an interface that exists only
// in retained server history.
vi.mock('recharts', () => ({
  AreaChart: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="chart">{children}</div>
  ),
  Area: () => null,
  XAxis: () => null,
  YAxis: () => null,
  Tooltip: () => null,
  ResponsiveContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

const liveState = {
  interfaceDataPoints: {} as Record<string, unknown[]>,
  connected: true,
};

vi.mock('@/hooks/use-websocket', () => ({
  useWebSocket: () => liveState,
}));

const historyState = {
  data: undefined as
    { points: { t: number; ifname: string; rx_bytes: number; tx_bytes: number }[] } | undefined,
};

vi.mock('@/hooks/use-data-usage', () => ({
  useTrafficHistory: () => historyState,
}));

import { InterfaceTrafficCharts } from '../interface-traffic-charts';

describe('InterfaceTrafficCharts', () => {
  it('draws an interface that only exists in retained server history', () => {
    historyState.data = {
      points: [
        { t: 10, ifname: 'eth0', rx_bytes: 0, tx_bytes: 0 },
        { t: 12, ifname: 'eth0', rx_bytes: 2000, tx_bytes: 0 },
        { t: 14, ifname: 'eth0', rx_bytes: 6000, tx_bytes: 0 },
      ],
    };

    render(<InterfaceTrafficCharts />);

    // Before this, the page listed interfaces only from the live buffer, so a
    // fresh load showed "Collecting data..." and 0 B/s while the server was
    // already holding ten minutes of samples.
    expect(screen.getByText('eth0')).toBeInTheDocument();
    expect(screen.queryByText('Collecting data…')).not.toBeInTheDocument();
    expect(screen.getAllByTestId('chart').length).toBeGreaterThan(0);
  });

  it('shows the disconnected message when there is neither history nor live data', () => {
    historyState.data = undefined;

    render(<InterfaceTrafficCharts />);

    expect(screen.getByText(/Waiting for interface data/i)).toBeInTheDocument();
  });
});
