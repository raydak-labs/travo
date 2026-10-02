import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { HeaderOverflowMenu } from '../header-overflow-menu';
import { HeaderNotificationsMenu } from '../header-notifications-menu';
import { useAuthStore } from '@/stores/auth-store';
import { useReboot, useShutdown } from '@/hooks/use-system';
import { useAlerts } from '@/hooks/use-alerts';

vi.mock('@/hooks/use-system', () => ({
  useReboot: vi.fn(),
  useShutdown: vi.fn(),
}));

vi.mock('@/hooks/use-alerts', () => ({
  useAlerts: vi.fn(),
}));

const mockUseReboot = vi.mocked(useReboot);
const mockUseShutdown = vi.mocked(useShutdown);
const mockUseAlerts = vi.mocked(useAlerts);

function stubMutations() {
  const reboot = { mutate: vi.fn(), isPending: false };
  const shutdown = { mutate: vi.fn(), isPending: false };
  mockUseReboot.mockReturnValue(reboot as unknown as ReturnType<typeof useReboot>);
  mockUseShutdown.mockReturnValue(shutdown as unknown as ReturnType<typeof useShutdown>);
  return { reboot, shutdown };
}

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({ token: 'test-token' } as never);
  mockUseAlerts.mockReturnValue({
    alerts: [],
    unreadCount: 0,
    markAllRead: vi.fn(),
  } as unknown as ReturnType<typeof useAlerts>);
});

describe('HeaderOverflowMenu', () => {
  it('exposes the menu state through aria-expanded', async () => {
    stubMutations();
    const user = userEvent.setup();
    render(<HeaderOverflowMenu />);

    const trigger = screen.getByRole('button', { name: 'Actions' });
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(trigger).toHaveAttribute('aria-haspopup', 'menu');

    await user.click(trigger);
    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('menu', { name: 'Router actions' })).toBeInTheDocument();
    expect(screen.getAllByRole('menuitem')).toHaveLength(3);
  });

  it('moves focus into the menu on open and back to the trigger on Escape', async () => {
    stubMutations();
    const user = userEvent.setup();
    render(<HeaderOverflowMenu />);

    const trigger = screen.getByRole('button', { name: 'Actions' });
    await user.click(trigger);

    const firstItem = screen.getByRole('menuitem', { name: /reboot router/i });
    await waitFor(() => expect(firstItem).toHaveFocus());

    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('menuitem', { name: /shut down router/i })).toHaveFocus();

    await user.keyboard('{Escape}');
    await waitFor(() => expect(trigger).toHaveFocus());
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('keeps the confirm dialog open until the reboot mutation settles', async () => {
    const { reboot } = stubMutations();
    let settle: (() => void) | undefined;
    reboot.mutate.mockImplementation((_vars: unknown, opts?: { onSettled?: () => void }) => {
      settle = opts?.onSettled;
    });
    const user = userEvent.setup();
    render(<HeaderOverflowMenu />);

    await user.click(screen.getByRole('button', { name: 'Actions' }));
    await user.click(screen.getByRole('menuitem', { name: /reboot router/i }));

    const confirm = screen.getByRole('button', { name: 'Reboot' });
    await user.click(confirm);
    expect(reboot.mutate).toHaveBeenCalledTimes(1);
    // Dialog is still mounted while the request is in flight.
    expect(screen.getByRole('button', { name: 'Reboot' })).toBeInTheDocument();

    settle?.();
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Reboot' })).not.toBeInTheDocument(),
    );
  });
});

describe('HeaderNotificationsMenu', () => {
  it('closes on Escape and returns focus to the trigger', async () => {
    const user = userEvent.setup();
    render(<HeaderNotificationsMenu />);

    const trigger = screen.getByRole('button', { name: 'Notifications' });
    expect(trigger).toHaveAttribute('aria-expanded', 'false');

    await user.click(trigger);
    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    const panel = screen.getByRole('region', { name: 'Notifications' });
    await waitFor(() => expect(panel).toHaveFocus());

    await user.keyboard('{Escape}');
    await waitFor(() => expect(trigger).toHaveFocus());
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('region', { name: 'Notifications' })).not.toBeInTheDocument();
  });

  it('closes on an outside pointer press', async () => {
    const user = userEvent.setup();
    render(
      <div>
        <HeaderNotificationsMenu />
        <button type="button">elsewhere</button>
      </div>,
    );

    await user.click(screen.getByRole('button', { name: 'Notifications' }));
    expect(screen.getByRole('region', { name: 'Notifications' })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'elsewhere' }));
    expect(screen.queryByRole('region', { name: 'Notifications' })).not.toBeInTheDocument();
  });
});
