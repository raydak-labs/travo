import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { TrafficRate } from '../traffic-rate';

describe('TrafficRate', () => {
  it('shares one unit when both directions use the same scale', () => {
    render(<TrafficRate rxBytesPerSec={3_125_000} txBytesPerSec={250_000} />);

    expect(screen.getByText('Download 25.0 Mbit/s, upload 2.0 Mbit/s')).toBeInTheDocument();
    expect(screen.getByText('25.0')).toBeInTheDocument();
    expect(screen.getByText('2.0')).toBeInTheDocument();
    expect(screen.getAllByText('Mbit/s')).toHaveLength(1);
  });

  it('keeps a unit next to each direction when the scales differ', () => {
    render(<TrafficRate rxBytesPerSec={117_500_000} txBytesPerSec={2_500_000} />);

    expect(screen.getByText('Download 0.9 Gbit/s, upload 20.0 Mbit/s')).toBeInTheDocument();
    expect(screen.getByText('Gbit/s')).toBeInTheDocument();
    expect(screen.getByText('Mbit/s')).toBeInTheDocument();
  });

  it('shows an unknown marker until both samples exist', () => {
    render(<TrafficRate rxBytesPerSec={null} txBytesPerSec={null} />);

    expect(screen.getByLabelText('Traffic rate unknown')).toBeInTheDocument();
  });
});
