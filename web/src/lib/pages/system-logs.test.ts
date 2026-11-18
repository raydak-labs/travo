import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { LogPage } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import SystemLogs from './SystemLogs.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as LogPage | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

describe('System log page', () => {
  it('shows entries with level, program and the truncation note', () => {
    // Shapes from testdata/device/axt1800-25.12.6/logs/logread.txt.
    loader.data = {
      entries: [
        {
          time: '2026-11-17T21:58:37',
          facility: 'daemon',
          level: 'err',
          program: 'procd',
          message: 'Got unexpected signal 1',
        },
        {
          time: '2026-11-17T21:58:34',
          facility: 'user',
          level: 'notice',
          program: '',
          message: 'Added device handler type: vrf',
        },
      ],
      truncated: true,
    };
    const body = render(SystemLogs).body;
    expect(body).toContain('11-17 21:58:37');
    expect(body).toContain('procd');
    expect(body).toContain('Got unexpected signal 1');
    expect(body).toContain('status-danger');
    expect(body).toContain('Showing the newest 2 matching lines.');
    expect(body).toContain('href="#/system"');
  });

  it('says when nothing matches', () => {
    loader.data = { entries: [], truncated: false };
    expect(render(SystemLogs).body).toContain('No log lines match.');
  });
});
