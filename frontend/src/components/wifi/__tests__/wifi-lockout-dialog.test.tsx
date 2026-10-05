import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WifiLockoutDialog } from '@/components/wifi/wifi-lockout-dialog';
import { ApiError } from '@/lib/api-client';
import { WIFI_LOCKOUT_ERROR_CODE, isWifiLockoutError } from '@/lib/wifi-lockout';

describe('isWifiLockoutError', () => {
  it('recognises the backend lockout code', () => {
    expect(
      isWifiLockoutError(new ApiError(409, 'no access point left', WIFI_LOCKOUT_ERROR_CODE)),
    ).toBe(true);
  });

  it('does not treat an ordinary failure as a lockout', () => {
    expect(isWifiLockoutError(new ApiError(500, 'boom'))).toBe(false);
    expect(isWifiLockoutError(new Error('boom'))).toBe(false);
    expect(isWifiLockoutError(undefined)).toBe(false);
  });
});

describe('WifiLockoutDialog', () => {
  it('cannot be accepted until the acknowledgement box is ticked', async () => {
    const user = userEvent.setup();
    let confirmed = 0;
    render(
      <WifiLockoutDialog
        open
        isPending={false}
        onCancel={() => {}}
        onConfirm={() => {
          confirmed += 1;
        }}
      />,
    );

    const confirm = screen.getByRole('button', { name: /apply anyway/i });
    expect(confirm).toBeDisabled();

    await user.click(confirm);
    expect(confirmed).toBe(0);

    await user.click(screen.getByRole('checkbox'));
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(confirmed).toBe(1);
  });

  it('says why the change was refused and what to do about it', () => {
    render(<WifiLockoutDialog open isPending={false} onCancel={() => {}} onConfirm={() => {}} />);

    expect(screen.getByText(/You are connected over WiFi/i)).toBeInTheDocument();
    expect(screen.getByText(/Ethernet cable/i)).toBeInTheDocument();
  });

  it('cancelling reports that nothing was sent', async () => {
    const user = userEvent.setup();
    let cancelled = 0;
    let confirmed = 0;
    render(
      <WifiLockoutDialog
        open
        isPending={false}
        onCancel={() => {
          cancelled += 1;
        }}
        onConfirm={() => {
          confirmed += 1;
        }}
      />,
    );

    await user.click(screen.getByRole('checkbox'));
    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(cancelled).toBe(1);
    expect(confirmed).toBe(0);
  });

  it('starts unticked every time it opens', async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <WifiLockoutDialog open isPending={false} onCancel={() => {}} onConfirm={() => {}} />,
    );
    await user.click(screen.getByRole('checkbox'));
    rerender(
      <WifiLockoutDialog open={false} isPending={false} onCancel={() => {}} onConfirm={() => {}} />,
    );
    rerender(<WifiLockoutDialog open isPending={false} onCancel={() => {}} onConfirm={() => {}} />);

    expect(screen.getByRole('checkbox')).not.toBeChecked();
    expect(screen.getByRole('button', { name: /apply anyway/i })).toBeDisabled();
  });
});
