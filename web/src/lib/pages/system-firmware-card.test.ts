import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import SystemFirmwareCard from './SystemFirmwareCard.svelte';

describe('Firmware upgrade card', () => {
  it('explains that packages are dropped and offers no flash before an image is checked', () => {
    const body = render(SystemFirmwareCard).body;
    expect(body).toContain('Firmware upgrade');
    expect(body).toContain('LuCI attended sysupgrade');
    expect(body).not.toContain('Flash and restart');
  });
});
