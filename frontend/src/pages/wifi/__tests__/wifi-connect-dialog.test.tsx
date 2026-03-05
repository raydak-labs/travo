import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WifiConnectDialog } from '../wifi-connect-dialog';
import type { WifiScanResult } from '@shared/index';

const mockNetwork: WifiScanResult = {
  ssid: 'TestNetwork',
  bssid: 'AA:BB:CC:DD:EE:FF',
  channel: 6,
  signal_dbm: -55,
  signal_percent: 65,
  encryption: 'wpa2',
  band: '2.4ghz',
};

describe('WifiConnectDialog', () => {
  it('renders SSID and password input', () => {
    render(
      <WifiConnectDialog
        network={mockNetwork}
        isConnecting={false}
        error={null}
        onConnect={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByText('TestNetwork')).toBeInTheDocument();
    expect(screen.getByLabelText('Password')).toBeInTheDocument();
  });

  it('shows and hides password with toggle', async () => {
    const user = userEvent.setup();

    render(
      <WifiConnectDialog
        network={mockNetwork}
        isConnecting={false}
        error={null}
        onConnect={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    const passwordInput = screen.getByLabelText('Password');
    expect(passwordInput).toHaveAttribute('type', 'password');

    await user.click(screen.getByLabelText('Show password'));
    expect(passwordInput).toHaveAttribute('type', 'text');

    await user.click(screen.getByLabelText('Hide password'));
    expect(passwordInput).toHaveAttribute('type', 'password');
  });

  it('calls connect on submit with password', async () => {
    const user = userEvent.setup();
    const onConnect = vi.fn();

    render(
      <WifiConnectDialog
        network={mockNetwork}
        isConnecting={false}
        error={null}
        onConnect={onConnect}
        onCancel={vi.fn()}
      />,
    );

    await user.type(screen.getByLabelText('Password'), 'mypassword');
    await user.click(screen.getByRole('button', { name: 'Connect' }));

    expect(onConnect).toHaveBeenCalledWith('TestNetwork', 'mypassword');
  });

  it('does not require password for open networks', () => {
    const openNetwork: WifiScanResult = { ...mockNetwork, encryption: 'none' };

    render(
      <WifiConnectDialog
        network={openNetwork}
        isConnecting={false}
        error={null}
        onConnect={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.queryByLabelText('Password')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Connect' })).not.toBeDisabled();
  });

  it('shows error message on failure', () => {
    render(
      <WifiConnectDialog
        network={mockNetwork}
        isConnecting={false}
        error="Connection failed"
        onConnect={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByText('Connection failed')).toBeInTheDocument();
  });

  it('shows connecting state', () => {
    render(
      <WifiConnectDialog
        network={mockNetwork}
        isConnecting={true}
        error={null}
        onConnect={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    expect(screen.getByText('Connecting...')).toBeInTheDocument();
  });

  it('shows validation error for short password', async () => {
    const user = userEvent.setup();
    const onConnect = vi.fn();

    render(
      <WifiConnectDialog
        network={mockNetwork}
        isConnecting={false}
        error={null}
        onConnect={onConnect}
        onCancel={vi.fn()}
      />,
    );

    await user.type(screen.getByLabelText('Password'), 'short');
    await user.click(screen.getByRole('button', { name: 'Connect' }));

    expect(screen.getByText('Password must be at least 8 characters')).toBeInTheDocument();
    expect(onConnect).not.toHaveBeenCalled();
  });

  it('clears validation error when user types more', async () => {
    const user = userEvent.setup();

    render(
      <WifiConnectDialog
        network={mockNetwork}
        isConnecting={false}
        error={null}
        onConnect={vi.fn()}
        onCancel={vi.fn()}
      />,
    );

    await user.type(screen.getByLabelText('Password'), 'short');
    await user.click(screen.getByRole('button', { name: 'Connect' }));

    expect(screen.getByText('Password must be at least 8 characters')).toBeInTheDocument();

    await user.type(screen.getByLabelText('Password'), '123');
    expect(screen.queryByText('Password must be at least 8 characters')).not.toBeInTheDocument();
  });
});
