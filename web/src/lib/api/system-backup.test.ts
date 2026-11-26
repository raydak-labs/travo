import { afterEach, describe, expect, it, vi } from 'vitest';
import { getSystemBackup } from './gen';
import { downloadSystemBackup } from './system';

vi.mock('./gen', () => ({ getSystemBackup: vi.fn() }));

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('configuration backup download', () => {
  it('downloads the gzip response and revokes the object URL', async () => {
    vi.useFakeTimers();
    const data = new Blob(['test archive'], { type: 'application/gzip' });
    vi.mocked(getSystemBackup).mockResolvedValue({
      data,
      response: new Response(data),
      request: new Request('http://localhost'),
    });
    const click = vi.fn();
    const link = { click, href: '', download: '' };
    vi.stubGlobal('document', { createElement: () => link });
    const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:test');
    const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});

    await downloadSystemBackup();

    expect(getSystemBackup).toHaveBeenCalledWith({ parseAs: 'blob' });
    expect(create).toHaveBeenCalledWith(data);
    expect(link).toMatchObject({ href: 'blob:test', download: 'travo-config.tar.gz' });
    expect(click).toHaveBeenCalledOnce();
    await vi.runAllTimersAsync();
    expect(revoke).toHaveBeenCalledWith('blob:test');
  });

  it('rejects an error response without opening a download', async () => {
    vi.mocked(getSystemBackup).mockResolvedValue({
      data: undefined,
      error: { code: 'backup_failed', error: 'Could not create configuration backup' },
      response: new Response(null, { status: 500 }),
      request: new Request('http://localhost'),
    });
    const create = vi.spyOn(URL, 'createObjectURL');
    await expect(downloadSystemBackup()).rejects.toThrow('Configuration backup failed');
    expect(create).not.toHaveBeenCalled();
  });
});
