import { describe, expect, it, vi } from 'vitest';
import { lazyPage } from './lazy';

describe('lazyPage', () => {
  it('imports the chunk once and shares the result', async () => {
    const Page = {};
    const importer = vi.fn().mockResolvedValue({ default: Page });
    const load = lazyPage(importer);
    expect(await load()).toBe(Page);
    expect(await load()).toBe(Page);
    expect(importer).toHaveBeenCalledTimes(1);
  });

  it('does not cache a failed import, so retry fetches the chunk again', async () => {
    const Page = {};
    const importer = vi
      .fn()
      .mockRejectedValueOnce(new Error('chunk 404'))
      .mockResolvedValueOnce({ default: Page });
    const load = lazyPage(importer);
    await expect(load()).rejects.toThrow('chunk 404');
    expect(await load()).toBe(Page);
    expect(importer).toHaveBeenCalledTimes(2);
  });
});
