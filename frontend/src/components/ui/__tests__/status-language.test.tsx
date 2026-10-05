import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SectionHeading } from '../section-heading';
import { StatValue } from '../stat-value';
import { SummaryBand, SummaryTile } from '../summary-band';

describe('SectionHeading', () => {
  it('renders the shared group-label style as a level-2 heading', () => {
    render(<SectionHeading>Diagnostics</SectionHeading>);
    const heading = screen.getByRole('heading', { level: 2, name: 'Diagnostics' });
    expect(heading.className).toContain('uppercase');
  });
});

describe('StatValue', () => {
  it('distinguishes an unknown value from a falsy known one', () => {
    const { rerender } = render(<StatValue label="Clients" value={0} />);
    expect(screen.getByText('0')).toBeInTheDocument();

    rerender(<StatValue label="Clients" value={null} />);
    expect(screen.getByText('No data')).toBeInTheDocument();
  });

  it('marks a last-known value as stale instead of recolouring it', () => {
    render(<StatValue label="Signal" value="-62 dBm" stale />);
    expect(screen.getByText('stale')).toBeInTheDocument();
    expect(screen.getByText('-62 dBm')).toBeInTheDocument();
  });
});

describe('SummaryBand', () => {
  it('keeps other tiles readable when one tile fails, and retries only that tile', async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    render(
      <SummaryBand label="WiFi status">
        <SummaryTile title="Connection" tone="ok">
          <StatValue label="SSID" value="home" />
        </SummaryTile>
        <SummaryTile title="Internet" isError error={new Error('ubus timeout')} onRetry={onRetry}>
          <StatValue label="Reachable" value="yes" />
        </SummaryTile>
      </SummaryBand>,
    );

    expect(screen.getByText('home')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('ubus timeout');
    expect(screen.queryByText('Reachable')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('marks a stale tile rather than blanking it', () => {
    render(
      <SummaryBand label="WiFi status">
        <SummaryTile title="Radios" stale>
          <StatValue label="phy0-ap" value="5 GHz · ch 36" />
        </SummaryTile>
      </SummaryBand>,
    );

    expect(screen.getByText('5 GHz · ch 36')).toBeInTheDocument();
  });
});
