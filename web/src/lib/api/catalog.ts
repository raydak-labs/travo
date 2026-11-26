import { m } from '../paraglide/messages.js';
import { client } from './client';
import {
  type CatalogItem,
  type CatalogView,
  getServicesCatalog,
  installCatalogService,
  removeCatalogService,
  setCatalogAutostart,
} from './gen';
import type { Client } from './gen/client';
import { APIRequestError } from './request';

export type { CatalogItem, CatalogView };
export type CatalogAction = 'install' | 'remove' | 'on' | 'off';
export type CatalogResult = { view: CatalogView } | { error: string };

/** Maps the API's stable error codes to a message; never parses error text. */
export function catalogError(code: string | undefined): string {
  switch (code) {
    case 'catalog_busy':
      return m.catalog_err_busy();
    case 'catalog_offline':
      return m.catalog_err_offline();
    case 'catalog_no_space':
      return m.catalog_err_no_space();
    case 'catalog_built_in':
      return m.catalog_err_built_in();
    case 'catalog_in_use':
      return m.catalog_err_in_use();
    case 'catalog_not_installed':
      return m.catalog_err_not_installed();
    default:
      return m.catalog_failed();
  }
}

/** Whole MiB, rounded up, for the flash warning. */
export function sizeMiB(bytes: number): number {
  return Math.max(1, Math.ceil(bytes / 2 ** 20));
}

/** LuCI's page for an entry; LuCI listens on :8080 (:8443 over HTTPS) next to Travo. */
export function luciUrl(location: Pick<Location, 'protocol' | 'hostname'>, path: string): string {
  const port = location.protocol === 'https:' ? 8443 : 8080;
  const host = location.hostname.includes(':') ? `[${location.hostname}]` : location.hostname;
  return `${location.protocol}//${host}:${port}/cgi-bin/luci/${path}`;
}

function code(error: unknown): string | undefined {
  return (error as { code?: string } | undefined)?.code;
}

export async function fetchCatalog(
  signal?: AbortSignal,
  requestClient: Client = client,
): Promise<CatalogView> {
  const { data, error, response } = await getServicesCatalog({ signal, client: requestClient });
  if (!data) throw new APIRequestError(catalogError(code(error)), response?.status);
  return data;
}

export async function changeCatalog(
  id: string,
  action: CatalogAction,
  requestClient: Client = client,
): Promise<CatalogResult> {
  const path = { id };
  try {
    const { data, error } =
      action === 'install'
        ? await installCatalogService({ path, client: requestClient })
        : action === 'remove'
          ? await removeCatalogService({ path, client: requestClient })
          : await setCatalogAutostart({
              path,
              body: { enabled: action === 'on' },
              client: requestClient,
            });
    return data ? { view: data } : { error: catalogError(code(error)) };
  } catch {
    return { error: catalogError(undefined) };
  }
}
