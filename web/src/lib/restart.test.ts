import { beforeEach, describe, expect, it, vi } from 'vitest';

const { postSystemReboot } = vi.hoisted(() => ({ postSystemReboot: vi.fn() }));
vi.mock('./api/gen', () => ({ postSystemReboot }));
vi.mock('./api/health', () => ({ fetchHealth: vi.fn() }));
vi.mock('./ui/notify', () => ({ notify: { error: vi.fn(), progress: vi.fn(), success: vi.fn() } }));

import { restart } from './restart.svelte';

const confirmAnswer = (ok: boolean) => ({ ask: vi.fn().mockResolvedValue(ok) }) as never;

describe('restart', () => {
  beforeEach(() => postSystemReboot.mockReset());

  it('asks first and sends nothing when the user cancels', async () => {
    const confirm = confirmAnswer(false);
    await restart.ask(confirm);
    expect((confirm as { ask: ReturnType<typeof vi.fn> }).ask).toHaveBeenCalledWith(
      expect.objectContaining({ tone: 'danger' }),
    );
    expect(postSystemReboot).not.toHaveBeenCalled();
    expect(restart.state).toBe('idle');
  });

  it('goes back to idle when the daemon refuses, so the user can try again', async () => {
    postSystemReboot.mockResolvedValue({ error: { code: 'txn_pending' } });
    await restart.ask(confirmAnswer(true));
    expect(postSystemReboot).toHaveBeenCalledWith({ body: { confirm: true } });
    expect(restart.state).toBe('idle');
  });
});
