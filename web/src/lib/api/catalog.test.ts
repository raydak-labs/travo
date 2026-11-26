import { afterEach, describe, expect, it, vi } from 'vitest';
import { m } from '../paraglide/messages.js';
import { catalogError, changeCatalog, luciUrl, sizeMiB } from './catalog';
import { installCatalogService, removeCatalogService, setCatalogAutostart } from './gen';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({
  getServicesCatalog: vi.fn(),
  installCatalogService: vi.fn(),
  removeCatalogService: vi.fn(),
  setCatalogAutostart: vi.fn(),
}));

afterEach(() => vi.resetAllMocks());

const view = { entries: [], busy: false };
const answer = (data: unknown, error?: unknown) => ({ data, error }) as never;

describe('services catalog', () => {
  it('maps error codes, never error text', () => {
    expect(catalogError('catalog_no_space')).toBe(m.catalog_err_no_space());
    expect(catalogError('catalog_built_in')).toBe(m.catalog_err_built_in());
    expect(catalogError('whatever')).toBe(m.catalog_failed());
  });

  it('rounds sizes up to whole MiB', () => {
    expect(sizeMiB(25 * 2 ** 20)).toBe(25);
    expect(sizeMiB(1)).toBe(1);
  });

  it('links LuCI on its own port', () => {
    expect(luciUrl({ protocol: 'http:', hostname: '192.168.1.1' }, 'admin/network/sqm')).toBe(
      'http://192.168.1.1:8080/cgi-bin/luci/admin/network/sqm',
    );
    expect(luciUrl({ protocol: 'https:', hostname: 'fd00::1' }, 'admin/x')).toBe(
      'https://[fd00::1]:8443/cgi-bin/luci/admin/x',
    );
  });

  it('sends each action to its endpoint', async () => {
    vi.mocked(installCatalogService).mockResolvedValue(answer(view));
    vi.mocked(removeCatalogService).mockResolvedValue(
      answer(undefined, { code: 'catalog_in_use' }),
    );
    vi.mocked(setCatalogAutostart).mockResolvedValue(answer(view));
    expect(await changeCatalog('vnstat', 'install')).toEqual({ view });
    expect(await changeCatalog('vnstat', 'remove')).toEqual({ error: m.catalog_err_in_use() });
    await changeCatalog('vnstat', 'off');
    expect(vi.mocked(setCatalogAutostart).mock.calls[0]?.[0]).toMatchObject({
      path: { id: 'vnstat' },
      body: { enabled: false },
    });
  });
});
