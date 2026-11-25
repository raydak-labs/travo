import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Token } from '../api/gen';
import { fetchTokens } from '../api/tokens';
import type { LoadStatus } from '../ui/loader.svelte';
import SystemTokens from './SystemAccess.svelte';
import TokenSecretDialog from './TokenSecretDialog.svelte';

const { createLoader, loader } = vi.hoisted(() => ({
  createLoader: vi.fn(),
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as Token[] | undefined,
    retry: vi.fn(),
    stop: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));

const token: Token = {
  id: 'a'.repeat(32),
  name: 'Home Assistant',
  scope: 'admin',
  created_at: '2026-11-18T09:30:00Z',
};

function page(tokens: Token[] | undefined, status: LoadStatus = { state: 'ready' }) {
  loader.data = tokens;
  loader.status = status;
  return render(SystemTokens).body;
}

describe('Access page: API tokens', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    createLoader.mockImplementation(() => loader);
  });

  it('loads the list once, without polling', () => {
    page([]);
    expect(createLoader).toHaveBeenCalledWith(fetchTokens);
  });

  it('empty: says so and offers Create token (the form opens in a dialog)', () => {
    const body = page([]);
    expect(body).toContain('No API tokens yet.');
    expect(body).toContain('Create token');
  });

  it('lists tokens with scope and a labelled revoke, never a secret', () => {
    const body = page([token]);
    expect(body).toContain('Home Assistant');
    expect(body).toContain('Admin');
    expect(body).toContain('aria-label="Revoke Home Assistant"');
    expect(body).not.toContain('tgt_');
  });

  it('opens with the Access page header, without a back link (the nav reaches System)', () => {
    const body = page([]);
    expect(body).toMatch(/<h1[^>]*>Access<\/h1>/);
    expect(body).not.toContain('href="#/system"');
  });

  it('shows the load error with retry', () => {
    const body = page(undefined, { state: 'error', message: 'API tokens are unavailable' });
    expect(body).toContain('API tokens are unavailable');
    expect(body).toContain('Retry');
  });
});

describe('new token dialog', () => {
  it('shows the secret once with the warning and a copy button', () => {
    const body = render(TokenSecretDialog, {
      props: { created: { ...token, token: 'tgt_abc' }, close: () => {} },
    }).body;
    expect(body).toContain('value="tgt_abc"');
    expect(body).toContain('This token will not be shown again.');
    expect(body).toContain('Copy');
  });

  it('renders nothing while no token was just created', () => {
    const body = render(TokenSecretDialog, { props: { created: undefined, close: () => {} } }).body;
    expect(body).not.toContain('tgt_');
    expect(body).not.toContain('will not be shown again');
  });
});
