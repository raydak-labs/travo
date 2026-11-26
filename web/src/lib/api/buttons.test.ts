import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  actionLabel,
  buttonsError,
  confirmText,
  fetchButtons,
  outcomeLabel,
  positionLabel,
  saveButtonAction,
} from './buttons';
import { type ButtonStatus, getButtons, setButtonAction } from './gen';

vi.mock('./client', () => ({ client: {} }));
vi.mock('./gen', () => ({ getButtons: vi.fn(), setButtonAction: vi.fn() }));

afterEach(() => vi.resetAllMocks());

// The AXT1800 keys of testdata buttons/devicetree-keys.tsv.
const status: ButtonStatus = {
  buttons: [
    { name: 'reset', label: 'reset', hotplug: 'reset', configurable: false, action: 'none' },
    { name: 'switch', label: 'switch', hotplug: 'BTN_0', configurable: true, action: 'none' },
  ],
};

function reply<T>(data: T | undefined, error?: { code?: string }, code = 200) {
  return {
    data,
    error,
    request: new Request('http://localhost'),
    response: new Response(null, { status: code }),
  } as never;
}

describe('button helpers', () => {
  it('loads the status and fails with the HTTP status', async () => {
    vi.mocked(getButtons).mockResolvedValue(reply(status));
    expect(await fetchButtons()).toEqual(status);
    vi.mocked(getButtons).mockResolvedValue(reply(undefined, undefined, 503));
    await expect(fetchButtons()).rejects.toMatchObject({ status: 503 });
  });

  it('labels actions, positions and outcomes', () => {
    expect(actionLabel('vpn')).toContain('VPN');
    expect(actionLabel('none')).toBe('Nothing');
    expect(confirmText('none')).toBeUndefined();
    expect(confirmText('guest_wifi')).toContain('guest');
    expect(positionLabel('pressed')).toBe('on position');
    expect(outcomeLabel('skipped_lockout')).toContain('lockout');
    expect(outcomeLabel('no_action')).toContain('no action');
  });

  it('saves the action and maps refusals', async () => {
    vi.mocked(setButtonAction).mockResolvedValue(reply(status));
    expect(await saveButtonAction('switch', 'vpn')).toEqual({ status });
    expect(setButtonAction).toHaveBeenCalledWith({
      path: { name: 'switch' },
      body: { action: 'vpn' },
    });
    vi.mocked(setButtonAction).mockResolvedValue(reply(undefined, { code: 'uci_busy' }, 409));
    expect(await saveButtonAction('switch', 'none')).toEqual({ error: buttonsError('uci_busy') });
    vi.mocked(setButtonAction).mockRejectedValue(new Error('network'));
    expect(await saveButtonAction('switch', 'none')).toEqual({ error: buttonsError(undefined) });
    expect(buttonsError('button_not_configurable')).toContain('stock');
  });
});
