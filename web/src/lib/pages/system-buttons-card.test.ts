import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import type { ButtonStatus } from '../api/gen';
import type { LoadStatus } from '../ui/loader.svelte';
import SystemButtonsCard from './SystemButtonsCard.svelte';

const { loader } = vi.hoisted(() => ({
  loader: {
    status: { state: 'ready' } as LoadStatus,
    data: undefined as ButtonStatus | undefined,
    retry: vi.fn(),
  },
}));
vi.mock('../ui/loader.svelte', () => ({ createLoader: () => loader }));

const reset = {
  name: 'reset',
  label: 'reset',
  hotplug: 'reset',
  configurable: false,
  action: 'none',
} as const;
const slide = { name: 'switch', label: 'switch', hotplug: 'BTN_0', configurable: true } as const;

describe('Buttons card', () => {
  it('keeps reset stock and offers the switch actions', () => {
    loader.data = { buttons: [reset, { ...slide, action: 'none' }] };
    const body = render(SystemButtonsCard).body;
    expect(body).toContain('Stock behaviour');
    expect(body).toContain('factory reset');
    // The action list opens in a popover; the closed trigger shows the current action.
    expect(body).toContain('data-slot="select-trigger"');
    expect(body).not.toContain('Last slide');
  });

  it('shows the mapped action and the last slide', () => {
    loader.data = {
      buttons: [
        reset,
        { ...slide, action: 'vpn', last: { at: 100, position: 'released', outcome: 'applied' } },
      ],
    };
    const body = render(SystemButtonsCard).body;
    expect(body).toContain('Last slide');
    expect(body).toContain('off position, done');
  });

  it('no buttons: says so', () => {
    loader.data = { buttons: [] };
    expect(render(SystemButtonsCard).body).toContain('no hardware buttons');
  });
});
