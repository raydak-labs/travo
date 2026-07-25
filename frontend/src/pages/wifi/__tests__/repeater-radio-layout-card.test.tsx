import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { RepeaterRadioLayoutCard } from '../repeater-radio-layout-card';

const useWifiConnection = vi.fn();
const useRepeaterRadioReconcile = vi.fn();

vi.mock('@/hooks/use-wifi', () => ({
  useWifiConnection: () => useWifiConnection(),
  useRepeaterRadioReconcile: () => useRepeaterRadioReconcile(),
}));

describe('RepeaterRadioLayoutCard', () => {
  beforeEach(() => {
    useRepeaterRadioReconcile.mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
    });
  });

  it('shows guidance when not in repeater mode instead of an empty shell', () => {
    useWifiConnection.mockReturnValue({ data: { mode: 'client' } });

    render(<RepeaterRadioLayoutCard />);

    expect(
      screen.getByText(/available in travel \/ repeater mode/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /re-apply sta\/ap separation/i }),
    ).not.toBeInTheDocument();
  });

  it('shows re-apply action when in repeater mode', () => {
    useWifiConnection.mockReturnValue({ data: { mode: 'repeater' } });

    render(<RepeaterRadioLayoutCard />);

    expect(
      screen.getByRole('button', { name: /re-apply sta\/ap separation/i }),
    ).toBeInTheDocument();
  });
});
