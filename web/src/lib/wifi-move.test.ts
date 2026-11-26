import { describe, expect, it } from 'vitest';
import type { WiFiSummary } from './api/gen';
import { apsMovedByJoin } from './wifi-move';

const summary: WiFiSummary = {
  radios: [
    { name: 'radio0', band: '5g', disabled: false, up: true },
    { name: 'radio1', band: '2g', disabled: false, up: true },
  ],
  stas: [],
  aps: [
    { section: 'default_radio1', radio: 'radio1', ssid: 'TravelAP', ifname: 'phy1-ap0', up: true },
  ],
};

describe('apsMovedByJoin', () => {
  it('names the AP when its radio is the only compatible one', () => {
    expect(apsMovedByJoin(summary, ['2g'])).toEqual(['TravelAP']);
  });
  it('moves nothing when a compatible radio is free', () => {
    expect(apsMovedByJoin(summary, ['5g'])).toEqual([]);
    expect(apsMovedByJoin(summary, ['5g', '2g'])).toEqual([]);
  });
  it('moves nothing without a summary or compatible radio', () => {
    expect(apsMovedByJoin(undefined, ['2g'])).toEqual([]);
    expect(apsMovedByJoin(summary, ['6g'])).toEqual([]);
  });
});
