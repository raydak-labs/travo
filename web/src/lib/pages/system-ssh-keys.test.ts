import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { KeyList } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import SystemSSHKeysCard from './SystemSSHKeysCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as KeyList | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

describe('SSH keys card', () => {
  it('lists keys by comment and fingerprint with a labelled remove', () => {
    loader.data = {
      keys: [
        {
          id: 'a'.repeat(64),
          type: 'ssh-ed25519',
          comment: 'laptop',
          fingerprint: 'SHA256:wDQ6rSZgjr4cNqg9V83faMCsq8u5Q3JFikiOxv5Rw84',
        },
      ],
      unmanaged: 2,
    };
    const body = render(SystemSSHKeysCard).body;
    expect(body).toContain('laptop');
    expect(body).toContain('SHA256:wDQ6rSZgjr4cNqg9V83faMCsq8u5Q3JFikiOxv5Rw84');
    expect(body).toContain('aria-label="Remove laptop"');
    expect(body).toContain('2 more line(s)');
    expect(body).toContain('Add key');
  });

  it('empty: says so and hides the unmanaged note', () => {
    loader.data = { keys: [], unmanaged: 0 };
    const body = render(SystemSSHKeysCard).body;
    expect(body).toContain('No SSH keys yet.');
    expect(body).not.toContain('more line(s)');
  });
});
