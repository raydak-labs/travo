import type { ReactNode } from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { InterfaceTrafficChartCard } from '../interface-traffic-chart-card';
import type { InterfaceDataPoint } from '@/hooks/use-websocket';

// jsdom gives recharts a zero-sized container, so it renders no chart internals
// and the SVG output cannot be asserted on. Stub the primitives instead: the
// contract under test is the colour tokens the card hands to recharts, which is
// exactly what these stubs expose.
vi.mock('recharts', () => {
  type AnyProps = Record<string, unknown>;
  type Child = { children?: ReactNode };
  const AreaChart = ({ children }: AnyProps & Child) => <div data-testid="chart">{children}</div>;
  const Area = ({ stroke, fill, dataKey }: AnyProps) => (
    <div
      data-testid={`area-${String(dataKey)}`}
      data-stroke={String(stroke)}
      data-fill={String(fill)}
    />
  );
  const axis =
    (testId: string) =>
    ({ tick, stroke }: AnyProps) => (
      <div
        data-testid={testId}
        data-tick-fill={String((tick as AnyProps | undefined)?.fill)}
        data-stroke={String(stroke)}
      />
    );
  const Tooltip = ({ contentStyle }: AnyProps) => (
    <div data-testid="tooltip" data-content-style={JSON.stringify(contentStyle ?? {})} />
  );
  return {
    AreaChart,
    Area,
    XAxis: axis('x-axis'),
    YAxis: axis('y-axis'),
    Tooltip,
    ResponsiveContainer: ({ children }: AnyProps & Child) => <div>{children}</div>,
  };
});

// Counter samples: `timestamp` is milliseconds and the byte fields are
// monotonic totals — computeTrafficRates derives B/s from consecutive deltas,
// so a fixture that does not match this shape silently renders the empty state.
function points(count: number): InterfaceDataPoint[] {
  const start = Date.parse('2026-09-28T10:00:00Z');
  return Array.from({ length: count }, (_unused, i) => ({
    timestamp: start + i * 2000,
    rxBytes: i * 4000,
    txBytes: i * 1000,
  }));
}

describe('InterfaceTrafficChartCard', () => {
  it('pairs the muted copy colour for light and dark mode', () => {
    render(<InterfaceTrafficChartCard name="eth0" points={points(1)} />);

    const ifaceName = screen.getByText('eth0');
    expect(ifaceName.className).toContain('text-gray-500');
    expect(ifaceName.className).toContain('dark:text-gray-400');

    const empty = screen.getByText(/Collecting data/i);
    expect(empty.className).toContain('text-gray-500');
    expect(empty.className).toContain('dark:text-gray-400');
  });

  it('paints the series and axes from the --chart-* theme tokens', () => {
    render(<InterfaceTrafficChartCard name="eth0" points={points(3)} />);

    // No hard-coded chart colours may survive: every stroke/fill resolves
    // through a --chart-* variable so dark mode follows index.css.
    expect(screen.getByTestId('area-rx')).toHaveAttribute('data-stroke', 'var(--chart-rx)');
    expect(screen.getByTestId('area-tx')).toHaveAttribute('data-stroke', 'var(--chart-tx)');
    expect(screen.getByTestId('x-axis')).toHaveAttribute('data-tick-fill', 'var(--chart-axis)');
    expect(screen.getByTestId('x-axis')).toHaveAttribute('data-stroke', 'var(--chart-grid)');
    expect(screen.getByTestId('y-axis')).toHaveAttribute('data-tick-fill', 'var(--chart-axis)');
    expect(screen.getByTestId('tooltip')).toHaveAttribute(
      'data-content-style',
      expect.stringContaining('var(--chart-tooltip-bg)'),
    );

    const markup = document.body.innerHTML;
    for (const hardCoded of ['#3b82f6', '#f59e0b', '#9ca3af', 'rgba(0,0,0,0.8)']) {
      expect(markup).not.toContain(hardCoded);
    }
  });
});
