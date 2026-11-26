import { describe, expect, it } from 'vitest';
import { ledLabel } from './api/leds';

describe('ledLabel', () => {
  it.each([
    ['blue:run', 'Power (blue)'],
    ['white:system', 'System (white)'],
    ['green:wlan-2g', 'WiFi (green)'],
    ['amber:foo', 'Foo (amber)'],
    ['status', 'Status'],
  ])('names %s as %s', (name, label) => {
    expect(ledLabel(name)).toBe(label);
  });
});
