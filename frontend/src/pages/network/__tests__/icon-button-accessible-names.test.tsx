import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DhcpReservationsTable } from '../dhcp-reservations-table';
import { FirewallPortForwardRulesTable } from '../firewall-port-forward-rules-table';
import { MACPolicyTable } from '../../wifi/mac-policy-table';
import { ClientsSearchBar } from '../../clients/clients-search-bar';
import type { DHCPReservation, PortForwardRule, MACPolicy } from '@shared/index';

/**
 * Icon-only controls have no visible text, so their accessible name has to
 * come from an explicit label (WCAG 4.1.2). These assertions exist so a future
 * refactor cannot silently drop the name again.
 */
describe('icon-only action buttons', () => {
  it('names the DHCP reservation delete button', async () => {
    const onDeleteSection = vi.fn();
    const reservations = [
      { name: 'printer', mac: 'aa:bb:cc:dd:ee:ff', ip: '192.168.1.20', section: 'r1' },
    ] as DHCPReservation[];
    const user = userEvent.setup();

    render(
      <DhcpReservationsTable
        reservations={reservations}
        onDeleteSection={onDeleteSection}
        deletePending={false}
      />,
    );

    await user.click(screen.getByRole('button', { name: 'Delete reservation for printer' }));
    expect(onDeleteSection).toHaveBeenCalledWith('r1');
  });

  it('names the port-forward delete button', async () => {
    const deleteRule = { mutate: vi.fn(), isPending: false };
    const rules = [
      {
        id: '1',
        name: 'ssh',
        protocol: 'tcp',
        src_dport: '22',
        dest_ip: '192.168.1.5',
        dest_port: '22',
      },
    ] as unknown as PortForwardRule[];
    const user = userEvent.setup();

    render(<FirewallPortForwardRulesTable rules={rules} deleteRule={deleteRule} />);

    await user.click(screen.getByRole('button', { name: 'Delete port forward rule ssh' }));
    expect(deleteRule.mutate).toHaveBeenCalledWith('1');
  });

  it('names the MAC policy delete button', async () => {
    const onDelete = vi.fn();
    const policies = [{ ssid: 'guest', mac: null }] as unknown as MACPolicy[];
    const user = userEvent.setup();

    render(<MACPolicyTable policies={policies} onDelete={onDelete} isPending={false} />);

    await user.click(screen.getByRole('button', { name: 'Delete MAC policy for guest' }));
    expect(onDelete).toHaveBeenCalledWith(0);
  });

  it('names the client search clear button', async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();

    render(<ClientsSearchBar value="laptop" onChange={onChange} />);

    await user.click(screen.getByRole('button', { name: 'Clear search' }));
    expect(onChange).toHaveBeenCalledWith('');
  });
});
