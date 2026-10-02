import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ClientAliasCell } from '../client-alias-cell';
import { useSetClientAlias } from '@/hooks/use-network';
import type { Client } from '@shared/index';

vi.mock('@/hooks/use-network', () => ({
  useSetClientAlias: vi.fn(),
}));

const mockUseSetClientAlias = vi.mocked(useSetClientAlias);

const client = {
  mac_address: 'aa:bb:cc:dd:ee:ff',
  hostname: 'laptop',
  ip_address: '192.168.1.10',
  alias: 'Work laptop',
} as unknown as Client;

beforeEach(() => {
  vi.clearAllMocks();
  mockUseSetClientAlias.mockReturnValue({
    mutate: vi.fn(),
    isPending: false,
  } as unknown as ReturnType<typeof useSetClientAlias>);
});

describe('ClientAliasCell', () => {
  it('gives the hover-revealed edit button an accessible name', () => {
    render(<ClientAliasCell client={client} />);

    expect(screen.getByRole('button', { name: 'Edit alias for Work laptop' })).toBeInTheDocument();
  });

  it('reveals the edit button for keyboard users, not just on hover', () => {
    render(<ClientAliasCell client={client} />);

    const editButton = screen.getByRole('button', { name: /edit alias/i });
    // The button is hidden with opacity only; without these two variants it is
    // focusable but invisible (WCAG 2.4.7).
    expect(editButton.className).toContain('opacity-0');
    expect(editButton.className).toContain('focus-visible:opacity-100');
    expect(editButton.className).toContain('group-focus-within:opacity-100');
  });

  it('names the save and cancel controls in the inline edit form', async () => {
    const user = userEvent.setup();
    render(<ClientAliasCell client={client} />);

    await user.click(screen.getByRole('button', { name: /edit alias/i }));

    expect(screen.getByRole('button', { name: 'Save alias' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel alias edit' })).toBeInTheDocument();
  });
});
